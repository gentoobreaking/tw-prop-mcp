/**
 * MCP (Model Context Protocol) client for the frontend.
 *
 * Uses the official @modelcontextprotocol/sdk with StreamableHTTPClientTransport
 * to handle the MCP Streamable HTTP protocol (SSE init + JSON-RPC POST).
 *
 * Never touches the database directly — all data flows through MCP tools.
 * All methods return typed results matching the domain models.
 */

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import type {
  Parcel,
  ParcelGeometry,
  SearchParcelResult,
  Transaction,
  TransactionStatistics,
  PriceStats,
  RoadAccessResult,
  NearbyRoad,
  ComparableResult,
  ValuationResult,
  ValuationExplanation,
  MapContext,
  ResponseMetadata,
  ProvenanceChain,
  McpError,
} from '../types';

const RUNTIME_CONFIG = typeof window !== 'undefined'
  ? (window as unknown as { RUNTIME_CONFIG?: { MCP_SERVER_URL?: string } }).RUNTIME_CONFIG
  : undefined;
const MCP_BASE_URL = RUNTIME_CONFIG?.MCP_SERVER_URL ?? import.meta.env.VITE_MCP_SERVER_URL;

// Singleton client + transport
let client: Client | null = null;
let transport: StreamableHTTPClientTransport | null = null;


async function getClient(): Promise<Client> {
  if (!MCP_BASE_URL) {
    throw new Error('MCP_SERVER_URL not configured.');
  }
  const baseUrl = new URL(MCP_BASE_URL, typeof window !== 'undefined' ? window.location.origin : undefined);
  transport = new StreamableHTTPClientTransport(baseUrl);
  client = new Client({
    name: 'tw-prop-mcp-frontend',
    version: '2.0.0',
  });
  await client.connect(transport);
  return client;
}

/**
 * Calls an MCP tool and returns the parsed result.
 * Throws an Error with a `.mcpError` property if the MCP tool
 * returned an error result.
 */
export async function callMCPTool<T>(toolName: string, params?: Record<string, unknown>): Promise<T> {
  const c = await getClient();
  const result = await c.callTool({
    name: toolName,
    arguments: params ?? {},
  });

  const contents = (result.content ?? []) as Array<Record<string, unknown>>;

  // Check for MCP error result
  if (result.isError || contents.length === 0) {
    for (const content of contents) {
      if ('text' in content && typeof content.text === 'string') {
        let parsed: unknown;
        try {
          parsed = JSON.parse(content.text);
        } catch {
          throw new Error(`MCP tool ${toolName} failed: ${content.text}`);
        }
        if (parsed && typeof parsed === 'object' && 'error' in parsed) {
          const err = parsed as McpError;
          const e = new Error(err.error.message) as Error & { mcpError: McpError };
          e.mcpError = err;
          throw e;
        }
      }
    }
    throw new Error(`MCP tool ${toolName} returned an error with no content`);
  }

  for (const content of contents) {
    if ('text' in content && typeof content.text === 'string') {
      const text = content.text;
      try {
        return JSON.parse(text) as T;
      } catch {
        // If not JSON, return the text itself
        return text as unknown as T;
      }
    }
  }

  return result as unknown as T;
}

/**
 * Extract structured error from a thrown error, or null if it's a plain error.
 */
export function extractMcpError(err: unknown): McpError | null {
  if (err && typeof err === 'object' && 'mcpError' in err) {
    const typed = err as { mcpError: McpError };
    return typed.mcpError;
  }
  return null;
}

// --- Parcel Tools ---

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

export async function searchParcels(params: {
  county: string;
  district: string;
  section?: string;
  areaMinSqm?: number;
  areaMaxSqm?: number;
  urbanZoning?: string;
  limit?: number;
  offset?: number;
}): Promise<SearchParcelResult> {
  return callMCPTool<SearchParcelResult>('search_parcels', {
    county: params.county,
    district: params.district,
    ...(params.section && { section: params.section }),
    ...(params.areaMinSqm !== undefined && { area_min_sqm: params.areaMinSqm }),
    ...(params.areaMaxSqm !== undefined && { area_max_sqm: params.areaMaxSqm }),
    ...(params.urbanZoning && { urban_zoning: params.urbanZoning }),
    limit: params.limit ?? 100,
    offset: params.offset ?? 0,
  });
}

// --- GIS Tools ---

export async function getParcelGeometry(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
  epsg?: number;
}): Promise<ParcelGeometry> {
  return callMCPTool<ParcelGeometry>('get_parcel_geometry', {
    county: params.county,
    district: params.district,
    section: params.section,
    land_number: params.landNumber,
    ...(params.epsg && { epsg: params.epsg }),
  });
}

export async function getParcelMapContext(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<MapContext> {
  return callMCPTool<MapContext>('get_parcel_map_context', {
    county: params.county,
    district: params.district,
    section: params.section,
    land_number: params.landNumber,
  });
}

export async function checkRoadAccess(parcelId: string): Promise<RoadAccessResult> {
  return callMCPTool<RoadAccessResult>('check_road_access', {
    parcel_id: parcelId,
  });
}

export async function findNearbyRoads(params: {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}): Promise<{ roads: NearbyRoad[]; count: number; parcel_id: string }> {
  return callMCPTool<{ roads: NearbyRoad[]; count: number; parcel_id: string }>('find_nearby_roads', {
    county: params.county,
    district: params.district,
    section: params.section,
    land_number: params.landNumber,
  });
}

// --- Transaction Tools ---

export interface SearchTransactionsParams {
  county: string;
  district: string;
  section?: string;
  landNumber?: string;
  transactionType?: string;
  dateFrom?: string;
  dateTo?: string;
  limit?: number;
  offset?: number;
}

export async function searchTransactions(params: SearchTransactionsParams): Promise<{
  transactions: Transaction[];
  statistics: PriceStats;
  count: number;
  total_count: number;
  data_provenance: Array<Record<string, unknown>>;
  metadata: ResponseMetadata;
}> {
  return callMCPTool<{
    transactions: Transaction[];
    statistics: PriceStats;
    count: number;
    total_count: number;
    data_provenance: Array<Record<string, unknown>>;
    metadata: ResponseMetadata;
  }>('search_transactions', {
    county: params.county,
    district: params.district,
    ...(params.section && { section: params.section }),
    ...(params.landNumber && { land_number: params.landNumber }),
    ...(params.transactionType && { transaction_type: params.transactionType }),
    ...(params.dateFrom && { date_from: params.dateFrom }),
    ...(params.dateTo && { date_to: params.dateTo }),
    limit: params.limit ?? 50,
    offset: params.offset ?? 0,
  });
}

export async function getTransaction(transactionId: string): Promise<Transaction> {
  return callMCPTool<Transaction>('get_transaction', {
    transaction_id: transactionId,
  });
}

export async function getTransactionStatistics(params: {
  county: string;
  district: string;
  section?: string;
}): Promise<TransactionStatistics> {
  return callMCPTool<TransactionStatistics>('get_transaction_statistics', {
    county: params.county,
    district: params.district,
    ...(params.section && { section: params.section }),
  });
}

// --- Comparable Tools ---

export async function findComparables(params: {
  parcelId: string;
  count?: number;
  searchRadiusM?: number;
}): Promise<{ comparables: ComparableResult[]; count: number; query_hash: string }> {
  return callMCPTool<{ comparables: ComparableResult[]; count: number; query_hash: string }>(
    'find_comparable_transactions',
    {
      parcel_id: params.parcelId,
      count: params.count ?? 10,
      ...(params.searchRadiusM && { search_radius_m: params.searchRadiusM }),
    },
  );
}

// --- Valuation Tools ---

export async function estimateLandValue(
  parcelId: string,
  opts?: {
    snapshotId?: string;
    algorithmVersion?: string;
    configurationVersion?: string;
    outlierMethod?: string;
  },
): Promise<ValuationResult> {
  return callMCPTool<ValuationResult>('estimate_land_value', {
    parcel_id: parcelId,
    ...(opts?.snapshotId && { snapshot_id: opts.snapshotId }),
    ...(opts?.algorithmVersion && { algorithm_version: opts.algorithmVersion }),
    ...(opts?.configurationVersion && { configuration_version: opts.configurationVersion }),
    ...(opts?.outlierMethod && { outlier_method: opts.outlierMethod }),
  });
}

export async function explainValuation(
  valuationId: string,
  detailLevel: 'basic' | 'detailed' | 'full' = 'detailed',
): Promise<ValuationExplanation> {
  return callMCPTool<ValuationExplanation>('explain_valuation', {
    valuation_id: valuationId,
    detail_level: detailLevel,
  });
}

// --- Provenance Tools ---

export async function getDataProvenance(parcelId?: string): Promise<ProvenanceChain> {
  return callMCPTool<ProvenanceChain>('get_data_provenance', {
    ...(parcelId && { parcel_id: parcelId }),
  });
}

export async function getDataSnapshot(snapshotId: string): Promise<Record<string, unknown>> {
  return callMCPTool<Record<string, unknown>>('get_data_snapshot', {
    snapshot_id: snapshotId,
  });
}

// --- Combined loader ---

/**
 * Loads all data needed for a selected parcel.
 * Used by the state management hook after a user selects a parcel from search.
 */
export interface LoadParcelViewParams {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}

export async function loadParcelView(params: LoadParcelViewParams): Promise<{
  parcel: ParcelGeometry;
  transactions: Transaction[];
  transaction_statistics: PriceStats;
  road_access: RoadAccessResult;
  nearby_roads: NearbyRoad[];
  comparables: ComparableResult[];
  valuation: ValuationResult;
  map_context: MapContext;
  provenance: Record<string, ProvenanceChain>;
  metadata: ResponseMetadata;
}> {
  // First, fetch the parcel to obtain its UUID
  const parcel = await getParcel(params);
  const parcelId = parcel.parcel_id;

  // Fetch all data in parallel
  const [
    parcelGeom,
    transactionsResp,
    roadAccessResp,
    nearbyRoadsResp,
    comparablesResp,
    valuationResp,
    mapContextResp,
    transactionStatsResp,
    provenanceResp,
  ] = await Promise.allSettled([
    getParcelGeometry(params),
    searchTransactions({
      county: params.county,
      district: params.district,
      section: params.section,
      landNumber: params.landNumber,
      limit: 50,
    }),
    checkRoadAccess(parcelId),
    findNearbyRoads(params),
    findComparables({ parcelId }),
    estimateLandValue(parcelId),
    getParcelMapContext(params),
    getTransactionStatistics({
      county: params.county,
      district: params.district,
      section: params.section,
    }),
    getDataProvenance(parcelId),
  ]);

  const metadata: ResponseMetadata = {
    algorithm_version: transactionStatsResp.status === 'fulfilled'
      ? transactionStatsResp.value.metadata?.algorithm_version ?? ''
      : '',
    snapshot_id: transactionStatsResp.status === 'fulfilled'
      ? transactionStatsResp.value.metadata?.snapshot_id ?? ''
      : '',
    generated_at: '',
    query_hash: '',
    request_id: '',
  };

  return {
    parcel: parcelGeom.status === 'fulfilled' ? parcelGeom.value : {} as ParcelGeometry,
    transactions:
      transactionsResp.status === 'fulfilled' ? transactionsResp.value.transactions : [],
    transaction_statistics: transactionsResp.status === 'fulfilled'
      ? transactionsResp.value.statistics
      : transactionsResp.status === 'rejected' && transactionStatsResp.status === 'fulfilled'
        ? transactionStatsResp.value.price_per_ping
        : ({} as PriceStats),
    road_access: roadAccessResp.status === 'fulfilled'
      ? roadAccessResp.value
      : ({} as RoadAccessResult),
    nearby_roads: nearbyRoadsResp.status === 'fulfilled' ? nearbyRoadsResp.value.roads : [],
    comparables: comparablesResp.status === 'fulfilled' ? comparablesResp.value.comparables : [],
    valuation: valuationResp.status === 'fulfilled'
      ? valuationResp.value
      : ({} as ValuationResult),
    map_context: mapContextResp.status === 'fulfilled'
      ? mapContextResp.value
      : undefined as unknown as MapContext,
    provenance: {
      parcel: provenanceResp.status === 'fulfilled' ? provenanceResp.value : undefined,
    },
    metadata,
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
