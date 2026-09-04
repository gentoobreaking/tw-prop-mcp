/**
 * MCP (Model Context Protocol) API client for the frontend.
 *
 * The frontend communicates with the MCP server exclusively through
 * JSON-RPC over Streamable HTTP — it NEVER touches the database directly
 * (P4/P18 AI Isolation: Service Layer is the unique path).
 *
 * MCP server URL is configured via VITE_MCP_SERVER_URL.
 */

import type { ViewData, Transaction, Parcel, LatLng } from '../types';

const MCP_BASE_URL = import.meta.env.VITE_MCP_SERVER_URL || 'http://localhost:8080/mcp';

interface MCPRequest {
  jsonrpc: '2.0';
  method: string;
  params?: Record<string, unknown>;
  id: number | string;
}

interface MCPResponse<T = unknown> {
  jsonrpc: '2.0';
  result?: T;
  error?: { code: number; message: string };
  id: number | string;
}

let requestId = 0;

async function callMCP<T>(method: string, params?: Record<string, unknown>): Promise<T> {
  const req: MCPRequest = {
    jsonrpc: '2.0',
    method,
    params,
    id: ++requestId,
  };

  const resp = await fetch(MCP_BASE_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(req),
  });

  if (!resp.ok) {
    throw new Error(`MCP server error: ${resp.status} ${resp.statusText}`);
  }

  const data: MCPResponse<T> = await resp.json();

  if (data.error) {
    throw new Error(`MCP error [${data.error.code}]: ${data.error.message}`);
  }

  return data.result as T;
}

export async function searchTransactions(params: {
  county: string;
  district: string;
  section?: string;
  landNumber?: string;
  limit?: number;
  offset?: number;
}): Promise<{ transactions: Transaction[]; metadata: unknown }> {
  return callMCP<{ transactions: Transaction[]; metadata: unknown }>('search_transactions', {
    county: params.county,
    district: params.district,
    ...(params.section && { section: params.section }),
    ...(params.landNumber && { land_number: params.landNumber }),
    ...(params.limit && { limit: params.limit }),
    ...(params.offset && { offset: params.offset }),
  });
}

export async function getParcel(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<Parcel> {
  return callMCP<Parcel>('get_parcel', {
    county: params.county,
    district: params.district,
    section: params.section,
    land_number: params.landNumber,
  });
}

export async function getParcelGeometry(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<{
  geometry: unknown;
  centroid: LatLng;
  bbox: unknown;
  area_sqm: number;
  metadata: unknown;
}> {
  return callMCP<{
    geometry: unknown;
    centroid: LatLng;
    bbox: unknown;
    area_sqm: number;
    metadata: unknown;
  }>('get_parcel_geometry', {
    county: params.county,
    district: params.district,
    section: params.section,
    land_number: params.landNumber,
  });
}

export async function getMapContext(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<{ latitude: number; longitude: number; zoom: number; bounds?: unknown }> {
  return callMCP<{ latitude: number; longitude: number; zoom: number; bounds?: unknown }>(
    'get_parcel_map_context',
    {
      county: params.county,
      district: params.district,
      section: params.section,
      land_number: params.landNumber,
    },
  );
}

export async function findComparables(params: {
  parcelId: string;
  count?: number;
  searchRadiusM?: number;
}): Promise<{ comparables: unknown[]; metadata: unknown }> {
  return callMCP<{ comparables: unknown[]; metadata: unknown }>('find_comparable_transactions', {
    parcel_id: params.parcelId,
    count: params.count ?? 10,
    search_radius_m: params.searchRadiusM,
  });
}

export async function estimateLandValue(
  parcelId: string,
): Promise<{ valuation: unknown; metadata: unknown }> {
  return callMCP<{ valuation: unknown; metadata: unknown }>('estimate_land_value', {
    parcel_id: parcelId,
  });
}

export async function checkRoadAccess(
  parcelId: string,
): Promise<{ road_access: unknown[]; metadata: unknown }> {
  return callMCP<{ road_access: unknown[]; metadata: unknown }>('check_road_access', {
    parcel_id: parcelId,
  });
}

export async function loadMapView(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<ViewData> {
  // First, fetch the parcel to obtain its UUID — required as input for
  // check_road_access, find_comparable_transactions, and estimate_land_value.
  const parcel = await getParcel(params);
  const parcelId = parcel.parcel_id ?? parcel.id ?? '';

  const [parcelResp, transactionsResp, roadsResp, comparablesResp, valuationResp, mapContextResp] =
    await Promise.allSettled([
      getParcelGeometry(params),
      searchTransactions({
        county: params.county,
        district: params.district,
        section: params.section,
        landNumber: params.landNumber,
        limit: 50,
      }),
      checkRoadAccess(parcelId),
      findComparables({ parcelId }),
      estimateLandValue(parcelId),
      getMapContext(params),
    ]);

  // Build ViewData with type-safe extraction
  const metadata =
    mapContextResp.status === 'fulfilled'
      ? mapContextResp.value
      : transactionsResp.status === 'fulfilled'
        ? (transactionsResp.value.metadata as Record<string, unknown>)
        : {};

  return {
    parcel:
      parcelResp.status === 'fulfilled'
        ? (parcelResp.value as unknown as ViewData['parcel'])
        : undefined,
    transactions:
      transactionsResp.status === 'fulfilled'
        ? (transactionsResp.value.transactions as Transaction[])
        : [],
    roads:
      roadsResp.status === 'fulfilled'
        ? (roadsResp.value.road_access as unknown as ViewData['roads'])
        : [],
    comparables:
      comparablesResp.status === 'fulfilled'
        ? (comparablesResp.value.comparables as unknown as ViewData['comparables'])
        : [],
    valuation:
      valuationResp.status === 'fulfilled'
        ? (valuationResp.value.valuation as unknown as ViewData['valuation'])
        : undefined,
    map_context:
      mapContextResp.status === 'fulfilled'
        ? (mapContextResp.value as unknown as ViewData['map_context'])
        : undefined,
    metadata: metadata as unknown as ViewData['metadata'],
  };
}
