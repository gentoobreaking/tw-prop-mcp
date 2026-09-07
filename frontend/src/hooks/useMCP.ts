/**
 * Backward-compatible wrapper around useAppState.
 * Delegates to the unified state management hook.
 */

import { useAppState } from './useAppState';
import type { ViewData } from '../types';

export { useAppState } from './useAppState';

/**
 * Legacy hook interface for components still using the old shape.
 * Returns a minimal ViewData-like object from the current app state.
 */
export function useMCP(): {
  data: ViewData | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
  clearError: () => void;
} {
  const appState = useAppState();

  const data: ViewData | null = appState.selectedParcel
    ? {
        parcel: appState.selectedParcel,
        transactions: appState.transactions,
        roads: appState.nearbyRoads,
        comparables: appState.comparables,
        valuation: appState.valuation ?? undefined,
        map_context: appState.mapContext ?? undefined,
        metadata: {
          algorithm_version: appState.valuation?.algorithm_version ?? '',
          snapshot_id: appState.valuation?.snapshot_id ?? '',
          generated_at: '',
          query_hash: appState.valuation?.query_hash ?? '',
          request_id: '',
        },
      }
    : null;
  return {
    data,
    loading: appState.parcelLoading || appState.searchLoading,
    error: appState.parcelError || appState.systemError,
    refresh: () => {
      if (appState.selectedParcelIdentity) {
        void appState.loadParcel(appState.selectedParcelIdentity);
      }
    },
    clearError: appState.clearAllErrors,
  };
}
