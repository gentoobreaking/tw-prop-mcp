import { useState, useEffect, useCallback } from 'react';
import * as mcpApi from '../services/mcpApi';
import type { ViewData } from '../types';

interface UseMCPResult {
  data: ViewData | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
  clearError: () => void;
}

/**
 * Hook that loads all map view data from the MCP server.
 * Frontend only fetches structured data via MCP tools — no direct DB access.
 */
export function useMCP(): UseMCPResult {
  const [data, setData] = useState<ViewData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // TODO: Replace with actual user-selected parcel params from URL or UI
      // For now, load default view or wait for user interaction
      const viewData = await mcpApi.loadMapView({
        county: '臺北市',
        district: '中正區',
        section: '八德段',
        landNumber: '001-002-003',
      });
      setData(viewData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  const refresh = useCallback(() => {
    void loadData();
  }, [loadData]);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  useEffect(() => {
    // Only auto-load if we have an API URL configured
    if (import.meta.env.VITE_MCP_SERVER_URL) {
      void loadData();
    } else {
      setLoading(false);
      setError('MCP server URL not configured. Set VITE_MCP_SERVER_URL in .env');
    }
  }, [loadData]);

  return { data, loading, error, refresh, clearError };
}
