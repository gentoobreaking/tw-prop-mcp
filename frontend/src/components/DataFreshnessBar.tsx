import React, { useEffect, useState, useCallback } from 'react';
import { getDataFreshness, triggerDataRefresh } from '../services/mcpApi';
import type { DataFreshness } from '../types';
import './DataFreshnessBar.css';

function formatTime(iso?: string): string {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleString('zh-TW', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: 'Asia/Taipei',
    });
  } catch {
    return iso;
  }
}

export const DataFreshnessBar: React.FC = () => {
  const [data, setData] = useState<DataFreshness | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastRefreshMsg, setLastRefreshMsg] = useState<string | null>(null);

  const fetchFreshness = useCallback(async () => {
    try {
      const res = await getDataFreshness();
      setData(res);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchFreshness();
    const id = setInterval(() => void fetchFreshness(), 6 * 60 * 60 * 1000); // 6h poll aligns with scheduler
    return () => clearInterval(id);
  }, [fetchFreshness]);

  const handleRefresh = async () => {
    setRefreshing(true);
    setLastRefreshMsg(null);
    try {
      const res = await triggerDataRefresh();
      setLastRefreshMsg(res.message);
      // Poll for 60s every 3s for new snapshot
      let attempts = 20;
      const poll = async () => {
        const { promise, resolve } = Promise.withResolvers<void>();
        setTimeout(resolve, 3000);
        await promise;
        await fetchFreshness();
        attempts -= 1;
        if (attempts > 0) {
          const fresh = await getDataFreshness().catch(() => null);
          if (fresh && !fresh.is_stale) {
            setData(fresh);
            setLastRefreshMsg('更新完成，資料已是最新');
            setTimeout(() => window.location.reload(), 1500);
            return;
          }
          if (attempts > 0) void poll();
        }
      };
      void poll();
    } catch (e) {
      setLastRefreshMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setRefreshing(false);
      // Refresh display after trigger
      void fetchFreshness();
    }
  };

  if (loading) {
    return <div className="freshness-bar loading">資料時間載入中…</div>;
  }

  const isStale = data?.is_stale ?? false;
  const snapshotTime = data?.latest_import_completed_at ?? data?.latest_snapshot_at;

  return (
    <div className={`freshness-bar ${isStale ? 'stale' : 'fresh'}`}>
      <div className="freshness-main">
        <span className="freshness-label">資料時間</span>
        <span className="freshness-time" title={data?.stale_reason ?? ''}>
          {formatTime(snapshotTime)}
        </span>
        {data?.source && (
          <span className="freshness-meta">
            {data.source} {data.source_version} · {data.parcel_count} 筆地號 · {data.transaction_count} 筆交易
          </span>
        )}
        <span className={`freshness-badge ${isStale ? 'stale' : 'fresh'}`}>{isStale ? '過期' : '新鮮'}</span>
        {data?.next_release_window && (
          <span className="freshness-window" title="官方 1/11/21 發布，遇週末順延 3 天">
            下次發布窗口: {data.next_release_window}
          </span>
        )}
      </div>
      <div className="freshness-actions">
        <button
          type="button"
          className="freshness-refresh-btn"
          onClick={() => void handleRefresh()}
          disabled={refreshing}
          aria-label="立即更新資料"
          title={isStale ? '資料已過期，立即觸發下載' : '強制重新抓取最新資料'}
        >
          {refreshing ? '更新中…' : '立即更新'}
        </button>
        {data?.is_stale && !refreshing && <span className="freshness-hint">建議更新</span>}
      </div>
      {(error || lastRefreshMsg) && (
        <div className="freshness-msg">
          {error && <span className="error">{error}</span>}
          {lastRefreshMsg && <span className="info">{lastRefreshMsg}</span>}
        </div>
      )}
    </div>
  );
};
