/**
 * MCP (Model Context Protocol) client for the frontend.
 *
 * Uses the official @modelcontextprotocol/sdk with StreamableHTTPClientTransport
 * to handle the MCP Streamable HTTP protocol (SSE init + JSON-RPC POST).
 *
 * Never touches the database directly — all data flows through MCP tools.
 */

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import type { ViewData, Transaction, Parcel, LatLng } from '../types';
const RUNTIME_CONFIG = typeof window !== 'undefined'
  ? (window as unknown as { RUNTIME_CONFIG?: { MCP_SERVER_URL?: string } }).RUNTIME_CONFIG
  : undefined;
const MCP_BASE_URL = RUNTIME_CONFIG?.MCP_SERVER_URL ?? import.meta.env.VITE_MCP_SERVER_URL;
// Singleton client + transport
let client: Client | null = null;
let transport: StreamableHTTPClientTransport | null = null;

async function getClient(): Promise<Client> {
  if (!client) {
    const baseUrl = new URL(MCP_BASE_URL, typeof window !== 'undefined' ? window.location.origin : undefined);
    transport = new StreamableHTTPClientTransport(baseUrl);
    client = new Client({
      name: 'tw-prop-mcp-frontend',
      version: '2.0.0',
    });
    await client.connect(transport);
  }
  return client;
}

export async function callMCPTool<T>(toolName: string, params?: Record<string, unknown>): Promise<T> {
  const c = await getClient();
  const result = await c.callTool({
    name: toolName,
    arguments: params ?? {},
  });
  // MCP tool result content extraction
  const contents = (result.content ?? []) as Array<Record<string, unknown>>;
  if (contents.length > 0) {
    for (const content of contents) {
      if ('text' in content && typeof content.text === 'string') {
        const text = content.text;
        try {
          return JSON.parse(text) as T;
        } catch {
          return text as unknown as T;
        }
      }
    }
  }

  return result as unknown as T;
}

export async function searchTransactions(params: {
  county: string;
  district: string;
  section?: string;
  landNumber?: string;
  limit?: number;
  offset?: number;
}): Promise<{ transactions: Transaction[]; metadata: unknown }> {
  return callMCPTool<{ transactions: Transaction[]; metadata: unknown }>('search_transactions', {
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
  return callMCPTool<Parcel>('get_parcel', {
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
  return callMCPTool<{
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
  return callMCPTool<{ latitude: number; longitude: number; zoom: number; bounds?: unknown }>(
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
  return callMCPTool<{ comparables: unknown[]; metadata: unknown }>('find_comparable_transactions', {
    parcel_id: params.parcelId,
    count: params.count ?? 10,
    search_radius_m: params.searchRadiusM,
  });
}

export async function estimateLandValue(
  parcelId: string,
): Promise<{ valuation: unknown; metadata: unknown }> {
  return callMCPTool<{ valuation: unknown; metadata: unknown }>('estimate_land_value', {
    parcel_id: parcelId,
  });
}

export async function checkRoadAccess(
  parcelId: string,
): Promise<{ road_access: unknown[]; metadata: unknown }> {
  return callMCPTool<{ road_access: unknown[]; metadata: unknown }>('check_road_access', {
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
  const parcelId = parcel.parcel_id ?? '';

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

  // SAFETY: getParcelGeometry returns a ParcelGeometry whose shape
  // matches ViewData['parcel'] — API schema guarantees compatibility
  const parcelGeom = parcelResp.status === 'fulfilled'
    ? parcelResp.value as unknown as ViewData['parcel']
    : undefined;

  // SAFETY: checkRoadAccess returns road_access objects matching
  // ViewData['roads'] — API schema guarantees compatibility
  const roadsData = roadsResp.status === 'fulfilled'
    ? roadsResp.value.road_access as unknown as ViewData['roads']
    : [];

  // SAFETY: findComparables returns comparables matching
  // ViewData['comparables'] — API schema guarantees compatibility
  const comparablesData = comparablesResp.status === 'fulfilled'
    ? comparablesResp.value.comparables as unknown as ViewData['comparables']
    : [];

  // SAFETY: estimateLandValue returns valuation matching
  // ViewData['valuation'] — API schema guarantees compatibility
  const valuationData = valuationResp.status === 'fulfilled'
    ? valuationResp.value.valuation as unknown as ViewData['valuation']
    : undefined;

  // SAFETY: getMapContext returns ViewData['map_context'] shape
  // — the return type matches by API contract
  const mapContextData = mapContextResp.status === 'fulfilled'
    ? mapContextResp.value as unknown as ViewData['map_context']
    : undefined;

  // SAFETY: metadata is Record<string, unknown> from the MCP API,
  // structurally compatible with ViewData['metadata']
  const metadataData = metadata as unknown as ViewData['metadata'];

  return {
    parcel: parcelGeom,
    transactions:
      transactionsResp.status === 'fulfilled'
        ? (transactionsResp.value.transactions as Transaction[])
        : [],
    roads: roadsData,
    comparables: comparablesData,
    valuation: valuationData,
    map_context: mapContextData,
    metadata: metadataData,
  };
}

// Disconnect when page unloads
export function disconnectMCP(): void {
  if (client) {
    void client.close();
    client = null;
    transport = null;
  }
}
