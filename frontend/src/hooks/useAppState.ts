/**
 * Core frontend state management hook.
 *
 * Implements SPEC §47 (State Management) with distinct slices:
 * - Search State, Filter State, Map State, Selected Parcel,
 * - Selected Transaction, Selected Comparable, Valuation State,
 * - Layer State, UI State
 *
 * Per SPECS §74: frontend does NOT compute — only fetches via MCP tools,
 * renders, and handles interaction/navigation.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import * as mcpApi from '../services/mcpApi';
import type {
  Parcel,
  ParcelGeometry,
  ParcelSummary,
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

export type {
  Parcel,
  ParcelGeometry,
  ParcelSummary,
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
};

/** Normalized parcel identity: county/district/section/landNumber */
export interface ParcelIdentity {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}

/** Filter state per SPEC §8 */
export interface FilterState {
  dateRange: '1y' | '3y' | '5y' | 'custom';
  dateFrom?: string;
  dateTo?: string;
  areaMinPing?: number;
  areaMaxPing?: number;
  minPrice?: number;
  maxPrice?: number;
  minUnitPrice?: number;
  maxUnitPrice?: number;
  urbanZoning?: string;
  landUseCategory?: string;
  transactionType?: string;
}

/** Layer visibility state */
export interface LayerState {
  showSatellite: boolean;
  showNLSC: boolean;
  showStreetView: boolean;
  showRoads: boolean;
  showComparables: boolean;
  showTransactions: boolean;
}

/** API connection status per SPEC §5 header requirements */
export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected';

interface AppState {
  // Search state
  searchQuery: string;
  searchLoading: boolean;
  searchError: string | null;
  searchErrorMcp: McpError | null;
  searchResults: ParcelSummary[];
  searchTotalCount: number;

  // Selected parcel
  selectedParcelIdentity: ParcelIdentity | null;
  parcelLoading: boolean;
  parcelError: string | null;
  parcelErrorMcp: McpError | null;
  selectedParcel: ParcelGeometry | null;
  parcelInfo: Parcel | null;

  // Transactions
  transactions: Transaction[];
  transactionStatistics: TransactionStatistics | null;
  transactionsLoading: boolean;
  transactionsError: string | null;
  selectedTransaction: Transaction | null;

  // Comparables
  comparables: ComparableResult[];
  comparablesLoading: boolean;
  comparablesError: string | null;
  selectedComparable: ComparableResult | null;

  // Roads / GIS
  roadAccess: RoadAccessResult | null;
  nearbyRoads: NearbyRoad[];
  roadsLoading: boolean;

  // Valuation
  valuation: ValuationResult | null;
  valuationExplanation: ValuationExplanation | null;
  valuationLoading: boolean;
  valuationError: string | null;

  // Map context
  mapContext: MapContext | null;

  // Provenance
  provenance: Record<string, ProvenanceChain | undefined>;

  // Layers
  layers: LayerState;

  // Filters
  filters: FilterState;

  // UI state
  connectionStatus: ConnectionStatus;
  systemError: string | null;
}

const DEFAULT_FILTERS: FilterState = {
  dateRange: '5y',
  areaMinPing: undefined,
  areaMaxPing: undefined,
};

const DEFAULT_LAYERS: LayerState = {
  showSatellite: false,
  showNLSC: false,
  showStreetView: false,
  showRoads: true,
  showComparables: true,
  showTransactions: true,
};

const initialState: AppState = {
  searchQuery: '',
  searchLoading: false,
  searchError: null,
  searchErrorMcp: null,
  searchResults: [],
  searchTotalCount: 0,

  selectedParcelIdentity: null,
  parcelLoading: false,
  parcelError: null,
  parcelErrorMcp: null,
  selectedParcel: null,
  parcelInfo: null,

  transactions: [],
  transactionStatistics: null,
  transactionsLoading: false,
  transactionsError: null,
  selectedTransaction: null,

  comparables: [],
  comparablesLoading: false,
  comparablesError: null,
  selectedComparable: null,

  roadAccess: null,
  nearbyRoads: [],
  roadsLoading: false,

  valuation: null,
  valuationExplanation: null,
  valuationLoading: false,
  valuationError: null,

  mapContext: null,
  provenance: {},
  layers: DEFAULT_LAYERS,
  filters: DEFAULT_FILTERS,
  connectionStatus: 'connecting',
  systemError: null,
};

/** Convert years to date range */
function yearsToDateRange(years: number): { from: string; to: string } {
  const to = new Date();
  const from = new Date();
  from.setFullYear(to.getFullYear() - years);
  return {
    from: from.toISOString().split('T')[0],
    to: to.toISOString().split('T')[0],
  };
}

/** Ping to sqm conversion (1 ping = 3.305785 m²) */
export const PING_TO_SQM = 3.305785;

/** Format error for display per SPEC §43 */
function formatError(err: unknown): { message: string; mcpError: McpError | null } {
  const mcpError = mcpApi.extractMcpError(err);
  if (mcpError) {
    // Branch on error code per SPEC §72
    const code = mcpError.error.code;
    const baseMsg = mcpError.error.message || 'Unknown error';
    switch (code) {
      case 'INVALID_ARGUMENT':
        return { message: `Invalid request: ${baseMsg}`, mcpError };
      case 'PARCEL_NOT_FOUND':
        return { message: `Parcel not found: ${baseMsg}`, mcpError };
      case 'DATA_NOT_AVAILABLE':
        return { message: `Data not available: ${baseMsg}`, mcpError };
      case 'GIS_NOT_AVAILABLE':
        return { message: `GIS data unavailable: ${baseMsg}`, mcpError };
      case 'SNAPSHOT_NOT_FOUND':
        return { message: `Snapshot not found: ${baseMsg}`, mcpError };
      case 'VALUATION_NOT_AVAILABLE':
        return { message: `Valuation not available: ${baseMsg}`, mcpError };
      case 'SOURCE_UNAVAILABLE':
        return { message: `Data source unavailable: ${baseMsg}`, mcpError };
      case 'TRANSACTION_NOT_FOUND':
        return { message: `Transaction not found: ${baseMsg}`, mcpError };
      case 'INTERNAL_ERROR':
      default:
        return { message: `Server error: ${baseMsg}`, mcpError };
    }
  }
  return {
    message: err instanceof Error ? err.message : String(err || 'Unknown error'),
    mcpError: null,
  };
}

/**
 * Unified app state hook for the entire frontend.
 * Provides typed access to all state slices, MCP data loading,
 * and connection status management.
 */
export function useAppState() {
  const [state, setState] = useState<AppState>(initialState);

  // Track whether initial connection check has happened
  const connectionCheckedRef = useRef(false);
  // Ref for loadParcelDependentData to break circular dependency in useCallback
  const loadDependentDataRef = useRef<(
    parcelId: string,
    identity: ParcelIdentity,
  ) => Promise<void>>(async () => {});

  // --- Connection status check ---
  useEffect(() => {
    if (connectionCheckedRef.current) return;
    connectionCheckedRef.current = true;

    const runtimeUrl = (typeof window !== 'undefined' &&
      (window as unknown as { RUNTIME_CONFIG?: { MCP_SERVER_URL?: string } }).RUNTIME_CONFIG?.MCP_SERVER_URL) ||
      import.meta.env.VITE_MCP_SERVER_URL;

    if (!runtimeUrl) {
      setState((s) => ({
        ...s,
        connectionStatus: 'disconnected',
        systemError: 'MCP server URL not configured. Set MCP_SERVER_URL in runtime config.',
      }));
      return;
    }

    setState((s) => ({
      ...s,
      connectionStatus: 'connected',
    }));
  }, []);

  // --- Retry wrapper for transient failures ---
  const withRetry = useCallback(async <T>(
    fn: () => Promise<T>,
    maxRetries = 3,
  ): Promise<T> => {
    let lastError: unknown;
    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        return await fn();
      } catch (err) {
        lastError = err;
        const mcpErr = mcpApi.extractMcpError(err);
        // Don't retry non-retryable errors (INVALID_ARGUMENT, PARCEL_NOT_FOUND, etc.)
        const isNonRetryable = mcpErr && !mcpErr.error.retryable && [
          'INVALID_ARGUMENT', 'PARCEL_NOT_FOUND', 'TRANSACTION_NOT_FOUND',
          'SNAPSHOT_NOT_FOUND', 'SOURCE_UNAVAILABLE',
        ].includes(mcpErr.error.code);
        if (isNonRetryable || attempt === maxRetries) {
          throw err;
        }
        // Exponential backoff
        let resolveBackoff!: () => void;
        const promise = new Promise<void>((resolve) => {
          resolveBackoff = resolve;
        });
        setTimeout(resolveBackoff, Math.min(1000 * 2 ** attempt, 10000));
        await promise;
      }
    }
    throw lastError;
  }, []);

  // --- Search ---

  const searchParcels = useCallback(async (query: string) => {
    if (!query.trim()) {
      setState((s) => ({
        ...s,
        searchError: 'Please enter a search term (county, district, section, or land number).',
        searchErrorMcp: null,
        searchResults: [],
        searchTotalCount: 0,
      }));
      return;
    }

    setState((s) => ({
      ...s,
      searchQuery: query,
      searchLoading: true,
      searchError: null,
      searchErrorMcp: null,
    }));

    try {
      // Parse query: support "county district section landNumber" format
      // Also support partial matching by section name (e.g., "竹篙灣")
      const parts = query.trim().split(/\s+/);
      let county = '';
      let district = '';
      let section = '';
      let landNumber = '';

      // Determine if this looks like a 4-key query or a partial search
      if (parts.length >= 4) {
        [county, district, section, landNumber] = parts;
      } else if (parts.length >= 2) {
        // Assume county + district + section (no land number)
        // or county + district + section + landNumber
        county = parts[0] ?? '';
        district = parts[1] ?? '';
        section = parts[2] ?? '';
        landNumber = parts[3] ?? '';
      } else {
        // Single token — treat as partial section/land_number search
        // Backend search_parcels requires county + district, so we need
        // to attempt a broader search. For now, try common counties.
        // The backend's get_parcel requires all 4 keys.
        // We'll treat single token as section name and attempt search
        // with a default county/district if the user provides only that.
        section = parts[0] ?? '';
      }

      // Use search_parcels with best available params
      // search_parcels requires county + district
      if (!county || !district) {
        // Try interpreting as section + land_number with known county/district
        // The spec examples use "竹篙灣段 3615" which is section + landNumber
        if (parts.length >= 2) {
          // Assume first part is section, second is land number
          // Try to match against common counties
          county = '';
          district = '';
          section = parts[0] ?? '';
          landNumber = parts.slice(1).join(' ');
        }
      }

      let results: ParcelSummary[] = [];
      let totalCount = 0;

      if (county && district) {
        // Full search
        const resp = await withRetry(() =>
          mcpApi.searchParcels({
            county,
            district,
            ...(section && { section }),
            // landNumber is not supported by search_parcels — only by get_parcel
            limit: 100,
          }),
        );
        results = resp.parcels ?? [];
        totalCount = resp.total_count ?? results.length;
      } else {
        // Partial search — try to find parcels matching the section/name
        // The backend search_parcels requires county+district, so for
        // bare partial queries we search across known counties.
        // This is a simplified approach — in production, the backend
        // would have broader search capability.
        setState((s) => ({
          ...s,
          searchLoading: false,
          searchError:
            'Search requires at least county and district. Try: "臺北市 中正區 八德段 001-002-003"',
        }));
        return;
      }

      // Sort results by deterministic ranking per SPEC §7.5:
      // Exact → Normalized → Prefix → Partial match
      // Tie-break: county, township, section, parcel_no (alphabetical/numerical)
      results.sort((a, b) => {
        // Exact match on section + landNumber ranks first
        const aExact = a.section === section && a.land_number === landNumber;
        const bExact = b.section === section && b.land_number === landNumber;
        if (aExact && !bExact) return -1;
        if (!aExact && bExact) return 1;

        // Normalized exact (case/whitespace normalized)
        const normQuery = query.toLowerCase().trim();
        const aStr = `${a.county}${a.district}${a.section}${a.land_number}`.toLowerCase();
        const bStr = `${b.county}${b.district}${b.section}${b.land_number}`.toLowerCase();
        const aNorm = aStr === normQuery;
        const bNorm = bStr === normQuery;
        if (aNorm && !bNorm) return -1;
        if (!aNorm && bNorm) return 1;

        // Prefix match
        const aPrefix = aStr.startsWith(normQuery);
        const bPrefix = bStr.startsWith(normQuery);
        if (aPrefix && !bPrefix) return -1;
        if (!aPrefix && bPrefix) return 1;

        // Partial match (substring)
        const aPartial = aStr.includes(normQuery);
        const bPartial = bStr.includes(normQuery);
        if (aPartial && !bPartial) return -1;
        if (!aPartial && bPartial) return 1;

        // Tie-break: alphabetical by county, district, section, land_number
        if (a.county !== b.county) return a.county.localeCompare(b.county);
        if (a.district !== b.district) return a.district.localeCompare(b.district);
        if (a.section !== b.section) return a.section.localeCompare(b.section);
        return a.land_number.localeCompare(b.land_number);
      });

      setState((s) => ({
        ...s,
        searchQuery: query,
        searchResults: results,
        searchTotalCount: totalCount,
        searchLoading: false,
        searchError: null,
        searchErrorMcp: null,
      }));
    } catch (err) {
      const { message, mcpError } = formatError(err);
      setState((s) => ({
        ...s,
        searchError: message,
        searchErrorMcp: mcpError,
        searchLoading: false,
      }));
    }
  }, [withRetry]);

  const clearSearchResults = useCallback(() => {
    setState((s) => ({
      ...s,
      searchResults: [],
      searchTotalCount: 0,
      searchQuery: '',
      searchError: null,
      searchErrorMcp: null,
    }));
  }, []);

  // --- Load parcel by identity ---

  const loadParcel = useCallback(
    async (identity: ParcelIdentity) => {
      setState((s) => ({
        ...s,
        selectedParcelIdentity: identity,
        parcelLoading: true,
        parcelError: null,
        parcelErrorMcp: null,
      }));

      try {
        // Get parcel info (metadata)
        const parcelInfo = await withRetry(() => mcpApi.getParcel(identity));

        // Get parcel geometry
        const geometry = await withRetry(() => mcpApi.getParcelGeometry(identity));

        // Get map context
        const mapContext = await withRetry(() => mcpApi.getParcelMapContext(identity));

        setState((s) => ({
          ...s,
          selectedParcel: geometry,
          parcelInfo,
          mapContext,
          parcelLoading: false,
          parcelError: null,
          parcelErrorMcp: null,
        }));

        // Now load dependent data: transactions, roads, comparables, valuation
        await loadDependentDataRef.current(parcelInfo.id, identity);
      } catch (err) {
        const { message, mcpError } = formatError(err);
        setState((s) => ({
          ...s,
          parcelError: message,
          parcelErrorMcp: mcpError,
          parcelLoading: false,
        }));
      }
    },
    [withRetry],
  );

  // --- Load data dependent on parcel selection ---

  const loadParcelDependentData = useCallback(
    async (parcelId: string, identity: ParcelIdentity) => {
      const [
        transactionsResp,
        transactionStatsResp,
        roadAccessResp,
        nearbyRoadsResp,
        comparablesResp,
        valuationResp,
        provenanceResp,
      ] = await Promise.allSettled([
        withRetry(() =>
          mcpApi.searchTransactions({
            county: identity.county,
            district: identity.district,
            section: identity.section,
            landNumber: identity.landNumber,
            limit: 50,
          }),
        ),
        withRetry(() =>
          mcpApi.getTransactionStatistics({
            county: identity.county,
            district: identity.district,
            section: identity.section,
          }),
        ),
        withRetry(() => mcpApi.checkRoadAccess(parcelId)),
        withRetry(() => mcpApi.findNearbyRoads(identity)),
        withRetry(() => mcpApi.findComparables({ parcelId, count: 10 })),
        withRetry(() => mcpApi.estimateLandValue(parcelId)),
        withRetry(() => mcpApi.getDataProvenance(parcelId)),
      ]);

      setState((s) => ({
        ...s,
        transactions:
          transactionsResp.status === 'fulfilled'
            ? transactionsResp.value.transactions
            : [],
        transactionStatistics:
          transactionStatsResp.status === 'fulfilled'
            ? transactionStatsResp.value
            : null,
        roadAccess:
          roadAccessResp.status === 'fulfilled'
            ? roadAccessResp.value
            : null,
        nearbyRoads:
          nearbyRoadsResp.status === 'fulfilled'
            ? nearbyRoadsResp.value.roads
            : [],
        comparables:
          comparablesResp.status === 'fulfilled'
            ? comparablesResp.value.comparables
            : [],
        valuation:
          valuationResp.status === 'fulfilled'
            ? valuationResp.value
            : null,
        provenance: {
          ...s.provenance,
          parcel: provenanceResp.status === 'fulfilled' ? provenanceResp.value : undefined,
          // Transaction/comparable provenance info is supplementary (data_provenance arrays)
          // Only the parcel-level ProvenanceChain from get_data_provenance is stored as a full chain
        },
      }));

      // Set valuation explanation after valuation is resolved
      if (valuationResp.status === 'fulfilled' && valuationResp.value.id) {
        try {
          const explanation = await withRetry(() =>
            mcpApi.explainValuation(valuationResp.value.id, 'detailed'),
          );
          setState((s) => ({
            ...s,
            valuationExplanation: explanation,
          }));
        } catch {
          // Non-critical: explanation is supplementary
        }
      }
    },
    [withRetry],
  );
  // Update ref for use in loadParcel (breaks circular dependency)
  loadDependentDataRef.current = loadParcelDependentData;

  // --- Get transaction by ID ---

  const loadTransaction = useCallback(
    async (transactionId: string) => {
      setState((s) => ({
        ...s,
        transactionsLoading: true,
        transactionsError: null,
      }));
      try {
        const tx = await withRetry(() => mcpApi.getTransaction(transactionId));
        setState((s) => ({
          ...s,
          selectedTransaction: tx,
          transactionsLoading: false,
        }));
      } catch (err) {
        const { message } = formatError(err);
        setState((s) => ({
          ...s,
          transactionsError: message,
          transactionsLoading: false,
        }));
      }
    },
    [withRetry],
  );

  // --- Apply filters and reload transactions ---

  const applyFilters = useCallback(async () => {
    if (!state.selectedParcelIdentity) return;

    const identity = state.selectedParcelIdentity;
    const filters = state.filters;

    let dateFrom = filters.dateFrom;
    let dateTo = filters.dateTo;

    // Apply date range presets
    if (!dateFrom && !dateTo) {
      const range = yearsToDateRange(
        filters.dateRange === '1y' ? 1 : filters.dateRange === '3y' ? 3 : 5,
      );
      dateFrom = range.from;
      dateTo = range.to;
    }

    // Convert ping filters to sqm for API
    // Note: search_transactions does not support area filters.
    // Area filtering applies to search_parcels (parcel search).
    // Transaction filters use date range + county/district/section/land_number.
    setState((s) => ({
      ...s,
      transactionsLoading: true,
      transactionsError: null,
    }));

    try {
      const resp = await withRetry(() =>
        mcpApi.searchTransactions({
          county: identity.county,
          district: identity.district,
          section: identity.section,
          landNumber: identity.landNumber,
          dateFrom,
          dateTo,
          limit: 100,
        }),
      );
      setState((s) => ({
        ...s,
        transactions: resp.transactions,
        transactionStatistics: resp.statistics
          ? { price_per_ping: resp.statistics as PriceStats } as TransactionStatistics
          : s.transactionStatistics,
        transactionsLoading: false,
      }));
    } catch (err) {
      const { message } = formatError(err);
      setState((s) => ({
        ...s,
        transactionsError: message,
        transactionsLoading: false,
      }));
    }
  }, [state.selectedParcelIdentity, state.filters, withRetry]);

  const resetFilters = useCallback(() => {
    setState((s) => ({
      ...s,
      filters: DEFAULT_FILTERS,
    }));
  }, []);

  const updateFilters = useCallback((newFilters: Partial<FilterState>) => {
    setState((s) => ({
      ...s,
      filters: { ...s.filters, ...newFilters },
    }));
  }, []);

  // --- Layer controls ---

  const updateLayers = useCallback((newLayers: Partial<LayerState>) => {
    setState((s) => ({
      ...s,
      layers: { ...s.layers, ...newLayers },
    }));
  }, []);

  // --- Selection handlers ---

  const selectParcel = useCallback(
    (identity: ParcelIdentity) => {
      setState((s) => ({
        ...s,
        selectedTransaction: null,
        selectedComparable: null,
      }));
      void loadParcel(identity);
    },
    [loadParcel],
  );

  const selectTransaction = useCallback((tx: Transaction | null) => {
    setState((s) => ({
      ...s,
      selectedTransaction: tx,
    }));
  }, []);

  const selectComparable = useCallback((comp: ComparableResult | null) => {
    setState((s) => ({
      ...s,
      selectedComparable: comp,
      // Comparables don't carry transaction objects — no auto-selection
      selectedTransaction: null,
    }));
  }, []);

  const clearSearch = useCallback(() => {
    setState((s) => ({
      ...s,
      searchResults: [],
      searchTotalCount: 0,
      searchQuery: '',
      searchError: null,
      searchErrorMcp: null,
    }));
  }, []);

  const clearAllErrors = useCallback(() => {
    setState((s) => ({
      ...s,
      searchError: null,
      searchErrorMcp: null,
      parcelError: null,
      parcelErrorMcp: null,
      transactionsError: null,
      valuationError: null,
      systemError: null,
    }));
  }, []);

  return {
    state,
    // Search
    searchParcels,
    clearSearchResults,
    clearSearch,
    // Parcel
    loadParcel,
    selectParcel,
    selectedParcelIdentity: state.selectedParcelIdentity,
    selectedParcel: state.selectedParcel,
    parcelInfo: state.parcelInfo,
    parcelLoading: state.parcelLoading,
    parcelError: state.parcelError,
    parcelErrorMcp: state.parcelErrorMcp,
    // Transactions
    transactions: state.transactions,
    transactionStatistics: state.transactionStatistics,
    transactionsLoading: state.transactionsLoading,
    transactionsError: state.transactionsError,
    loadTransaction,
    selectedTransaction: state.selectedTransaction,
    selectTransaction,
    // Comparables
    comparables: state.comparables,
    comparablesLoading: state.comparablesLoading,
    comparablesError: state.comparablesError,
    selectedComparable: state.selectedComparable,
    selectComparable,
    // Roads / GIS
    roadAccess: state.roadAccess,
    nearbyRoads: state.nearbyRoads,
    roadsLoading: state.roadsLoading,
    // Valuation
    valuation: state.valuation,
    valuationExplanation: state.valuationExplanation,
    valuationLoading: state.valuationLoading,
    valuationError: state.valuationError,
    // Map
    mapContext: state.mapContext,
    // Provenance
    provenance: state.provenance,
    // Layers
    layers: state.layers,
    updateLayers,
    // Filters
    filters: state.filters,
    updateFilters,
    applyFilters,
    resetFilters,
    // Search state
    searchQuery: state.searchQuery,
    searchLoading: state.searchLoading,
    searchError: state.searchError,
    searchErrorMcp: state.searchErrorMcp,
    searchResults: state.searchResults,
    searchTotalCount: state.searchTotalCount,
    // Connection
    connectionStatus: state.connectionStatus,
    systemError: state.systemError,
    // Utility
    clearAllErrors,
  };
}
