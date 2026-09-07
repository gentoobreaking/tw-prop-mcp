import React, { useEffect, useState, useCallback, useRef } from 'react';
import { getDataFreshness, triggerDataRefresh, getImportProgress } from '../services/mcpApi';
import type { DataFreshness } from '../types';
import type { ImportProgress } from '../services/mcpApi';
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
  const [progress, setProgress] = useState<ImportProgress | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastRefreshMsg, setLastRefreshMsg] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

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

  const fetchProgress = useCallback(async () => {
    try {
      const p = await getImportProgress();
      setProgress(p);
      return p;
    } catch {
      // Fallback to admin endpoint
      try {
        const res = await fetch('/admin/progress');
        if (res.ok) {
          const j = (await res.json()) as ImportProgress;
          setProgress(j);
          return j;
        }
      } catch {}
      return null;
    }
  }, []);

  useEffect(() => {
    void fetchFreshness();
    const id = window.setInterval(() => void fetchFreshness(), 6 * 60 * 60 * 1000);
    return () => clearInterval(id);
  }, [fetchFreshness]);

  // Poll progress when refreshing
  useEffect(() => {
    if (!refreshing) return;
    const poll = async () => {
      const p = await fetchProgress();
      if (p && !p.running && p.percent === 100 && !p.error) {
        setLastRefreshMsg(p.message || '更新完成，資料已是最新');
        void fetchFreshness();
        setTimeout(() => window.location.reload(), 1500);
        setRefreshing(false);
        clearInterval(pollRef.current!);
      } else if (p && p.error) {
        setLastRefreshMsg(`失敗: ${p.error}`);
        setRefreshing(false);
        clearInterval(pollRef.current!);
      }
    };
    pollRef.current = window.setInterval(() => void poll(), 2000);
    return () => {
      clearInterval(pollRef.current!);
    };
  }, [refreshing, fetchProgress, fetchFreshness]);

  const handleRefresh = async () => {
    setRefreshing(true);
    setLastRefreshMsg(null);
    setProgress({ running: true, stage: 'starting', percent: 5, message: '啟動更新', updated_at: new Date().toISOString() });
    try {
      const res = await triggerDataRefresh();
      setLastRefreshMsg(res.message);
      // Progress polling will handle reload
    } catch (e) {
      setLastRefreshMsg(e instanceof Error ? e.message : String(e));
      setRefreshing(false);
    } finally {
      void fetchFreshness();
    }
  };

  const handleZipUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.name.toLowerCase().endsWith('.zip')) {
      setLastRefreshMsg('僅支援 .zip 檔案 (lvr_landcsv.zip)');
      return;
    }
    setRefreshing(true);
    setLastRefreshMsg(`上傳 ${file.name} 中…`);
    setProgress({ running: true, stage: 'uploading', percent: 10, message: `上傳 ${file.name}`, updated_at: new Date().toISOString() });
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch('/admin/import', { method: 'POST', body: form });
      const text = await res.text();
      if (!res.ok) {
        throw new Error(text || `上傳失敗 ${res.status}`);
      }
      setLastRefreshMsg(`已接收 ${file.name}，後台匯入中…`);
      setProgress({ running: true, stage: 'importing', percent: 30, message: '解壓並匯入中', updated_at: new Date().toISOString() });
      // Poll progress
    } catch (err) {
      setLastRefreshMsg(err instanceof Error ? err.message : String(err));
      setRefreshing(false);
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
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
        <label className="freshness-upload-btn" title="手動上傳 lvr_landcsv.zip">
          匯入 ZIP
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            onChange={(ev) => void handleZipUpload(ev)}
            disabled={refreshing}
            style={{ display: 'none' }}
          />
        </label>
        {data?.is_stale && !refreshing && <span className="freshness-hint">建議更新</span>}
      </div>
      {refreshing && progress && (
        <div className="freshness-progress">
          <div className="progress-bar-outer">
            <div className="progress-bar-inner" style={{ width: `${progress.percent}%` }} />
          </div>
          <span className="progress-text">
            {progress.stage} {progress.percent}% — {progress.message}
          </span>
          {progress.error && <span className="progress-error">{progress.error}</span>}
        </div>
      )}
      {(error || lastRefreshMsg) && !refreshing && (
        <div className="freshness-msg">
          {error && <span className="error">{error}</span>}
          {lastRefreshMsg && <span className="info">{lastRefreshMsg}</span>}
        </div>
      )}
      {lastRefreshMsg && refreshing && <div className="freshness-msg"><span className="info">{lastRefreshMsg}</span></div>}
    </div>
  );
};
