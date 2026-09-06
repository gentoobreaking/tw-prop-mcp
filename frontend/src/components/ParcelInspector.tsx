/**
 * ParcelInspector — central detail UI per SPEC §15.
 *
 * Required sections: Basic Information, Geometry, Transaction, Comparable,
 * Road, Valuation, Provenance.
 *
 * Per SPEC §74: frontend does NOT compute — reads values from MCP data only.
 * Per SPEC §46: no hardcoded production data.
 * Per SPEC §67: values must match API response exactly.
 */

import React from 'react';
import type {
  ParcelGeometry,
  Parcel,
  Transaction,
  ComparableResult,
  RoadAccessResult,
  ValuationResult,
  ProvenanceChain,
} from '../types';
import './ParcelInspector.css';

interface ParcelInspectorProps {
  parcel: ParcelGeometry | null;
  parcelInfo: Parcel | null;
  parcelLoading: boolean;
  parcelError: string | null;
  transactions: Transaction[];
  selectedTransaction: Transaction | null;
  _onSelectTransaction: (tx: Transaction) => void;
  comparables: ComparableResult[];
  selectedComparable: ComparableResult | null;
  _onSelectComparable: (comp: ComparableResult) => void;
  roadAccess: RoadAccessResult | null;
  valuation: ValuationResult | null;
  provenance: ProvenanceChain | undefined;
  onViewTransactions: () => void;
  onViewComparables: () => void;
  onViewRoad: () => void;
  onViewValuation: () => void;
  onViewProvenance: () => void;
}

/** Format area in both ping and sqm per SPEC §68 */
function formatArea(sqm: number): string {
  const ping = sqm / 3.305785;
  return `${ping.toFixed(2)} 坪 (${sqm.toFixed(2)} ㎡)`;
}

/** Check if official and GIS areas differ — SPEC §17 */
function checkAreaMismatch(official?: number, gis?: number): boolean {
  if (official === undefined || gis === undefined) return false;
  // Allow 1% tolerance for floating point
  const tolerance = official * 0.01;
  return Math.abs(official - gis) > tolerance;
}

export const ParcelInspector: React.FC<ParcelInspectorProps> = ({
  parcel,
  parcelInfo,
  parcelLoading,
  parcelError,
  transactions,
  selectedTransaction,
  _onSelectTransaction,
  comparables,
  selectedComparable,
  _onSelectComparable,
  roadAccess,
  valuation,
  provenance,
  onViewTransactions,
  onViewComparables,
  onViewRoad,
  onViewValuation,
  onViewProvenance,
}) => {
  if (parcelLoading) {
    return (
      <aside className="parcel-inspector">
        <div className="parcel-loading">
          <div className="loading-spinner">載入中…</div>
        </div>
      </aside>
    );
  }

  if (parcelError) {
    return (
      <aside className="parcel-inspector">
        <div className="parcel-error">
          <h3>無法載入地號資料</h3>
          <p>{parcelError}</p>
        </div>
      </aside>
    );
  }

  if (!parcel && !parcelInfo) {
    return (
      <aside className="parcel-inspector">
        <div className="parcel-empty">
          <p>請搜尋地號以查看詳細資訊</p>
          <p className="parcel-empty-hint">例: 臺北市 中正區 八德段 001-002-003</p>
        </div>
      </aside>
    );
  }

  const displayParcel = parcel ?? parcelInfo;
  const areaMismatch = checkAreaMismatch(
    displayParcel?.area_sqm,
    parcel?.gis_area_sqm,
  );

  return (
    <aside className="parcel-inspector">
      {/* Basic Information — SPEC §16 */}
      <section className="inspector-section">
        <h3 className="inspector-section-title">基本資訊</h3>
        <div className="info-grid">
          <div className="info-row">
            <span className="info-label">縣市</span>
            <span className="info-value">{parcelInfo?.county ?? '—'}</span>
          </div>
          <div className="info-row">
            <span className="info-label">行政區</span>
            <span className="info-value">{parcelInfo?.district ?? '—'}</span>
          </div>
          <div className="info-row">
            <span className="info-label">段名</span>
            <span className="info-value">{parcelInfo?.section ?? '—'}</span>
          </div>
          <div className="info-row">
            <span className="info-label">地號</span>
            <span className="info-value">{parcelInfo?.land_number ?? '—'}</span>
          </div>
          <div className="info-row">
            <span className="info-label">面積</span>
            <span className="info-value">
              {parcel ? formatArea(parcel.area_sqm) : '—'}
              {areaMismatch && (
                <span className="area-warning"> ⚠ GIS_AREA_MISMATCH</span>
              )}
            </span>
          </div>
          {parcelInfo?.urban_zoning && (
            <div className="info-row">
              <span className="info-label">使用分區</span>
              <span className="info-value">{parcelInfo.urban_zoning}</span>
            </div>
          )}
          {parcelInfo?.land_use_category && (
            <div className="info-row">
              <span className="info-label">使用地類別</span>
              <span className="info-value">{parcelInfo.land_use_category}</span>
            </div>
          )}
        </div>
      </section>

      {/* Geometry — SPEC §17 */}
      {parcel && (
        <section className="inspector-section">
          <h3 className="inspector-section-title">地籍面資</h3>
          <div className="info-grid">
            <div className="info-row">
              <span className="info-label">坐標中心點</span>
              <span className="info-value">
                {parcel.centroid.lat.toFixed(6)}, {parcel.centroid.lng.toFixed(6)}
              </span>
            </div>
            <div className="info-row">
              <span className="info-label">範圍</span>
              <span className="info-value">
                N:{parcel.bbox.northeast.lat.toFixed(6)} E:{parcel.bbox.northeast.lng.toFixed(6)}
              </span>
            </div>
            {parcel.geometry && (
              <div className="info-row">
                <span className="info-label">幾何類型</span>
                <span className="info-value">
                  {parcel.geometry.type as string}
                </span>
              </div>
            )}
            {areaMismatch && (
              <div className="info-row">
                <span className="info-label">面積狀態</span>
                <span className="info-value warning">
                  GIS_AREA_MISMATCH (官方: {parcel.area_sqm.toFixed(2)}㎡, GIS: {parcel.gis_area_sqm?.toFixed(2)}㎡)
                </span>
              </div>
            )}
          </div>
        </section>
      )}

      {/* Transaction — SPEC §18 */}
      <section className="inspector-section">
        <div className="inspector-section-header">
          <h3 className="inspector-section-title">交易記錄</h3>
          <button className="inspector-nav" onClick={onViewTransactions}>
            查看全部 →
          </button>
        </div>
        {selectedTransaction ? (
          <div className="info-grid">
            <div className="info-row">
              <span className="info-label">交易日期</span>
              <span className="info-value">{selectedTransaction.transaction_date}</span>
            </div>
            <div className="info-row">
              <span className="info-label">總價</span>
              <span className="info-value">NT$ {selectedTransaction.total_price.toLocaleString()}</span>
            </div>
            <div className="info-row">
              <span className="info-label">單價</span>
              <span className="info-value">{selectedTransaction.unit_price.toLocaleString()} 元/㎡</span>
            </div>
          </div>
        ) : (
          <p className="inspector-empty">
            {transactions.length === 0
              ? '無交易記錄'
              : '點擊交易列表查看詳細資訊'}
          </p>
        )}
      </section>

      {/* Comparable — SPEC §22 */}
      <section className="inspector-section">
        <div className="inspector-section-header">
          <h3 className="inspector-section-title">可比交易</h3>
          <button className="inspector-nav" onClick={onViewComparables}>
            查看全部 ({comparables.length}) →
          </button>
        </div>
        {selectedComparable ? (
          <div className="comparable-summary">
            <div className="comparable-score">
              相似度: {(selectedComparable.score * 100).toFixed(1)}%
            </div>
            <div className="comparable-distance">
              距離: {selectedComparable.distance_m.toFixed(0)} m
            </div>
          </div>
        ) : (
          <p className="inspector-empty">
            {comparables.length === 0
              ? '無可比交易'
              : '點擷可比交易查看評分'}
          </p>
        )}
      </section>

      {/* Road / GIS — SPEC §31 */}
      <section className="inspector-section">
        <div className="inspector-section-header">
          <h3 className="inspector-section-title">道路 / GIS</h3>
          <button className="inspector-nav" onClick={onViewRoad}>
            查看道路資訊 →
          </button>
        </div>
        {roadAccess ? (
          <div className="info-grid">
            <div className="info-row">
              <span className="info-label">道路臨接</span>
              <span className="info-value">{roadAccess.status}</span>
            </div>
            {roadAccess.distance_m !== undefined && (
              <div className="info-row">
                <span className="info-label">距離</span>
                <span className="info-value">{roadAccess.distance_m.toFixed(1)} m</span>
              </div>
            )}
            {roadAccess.road_width_m !== undefined && (
              <div className="info-row">
                <span className="info-label">道路寬度</span>
                <span className="info-value">{roadAccess.road_width_m.toFixed(1)} m</span>
              </div>
            )}
          </div>
        ) : (
          <p className="inspector-empty">無道路資料</p>
        )}
      </section>

      {/* Valuation — SPEC §26 */}
      <section className="inspector-section">
        <div className="inspector-section-header">
          <h3 className="inspector-section-title">估價</h3>
          <button className="inspector-nav" onClick={onViewValuation}>
            查看估價 →
          </button>
        </div>
        {valuation && valuation.status !== 'INSUFFICIENT_DATA' ? (
          <div className="info-grid">
            <div className="info-row">
              <span className="info-label">Bear (P25)</span>
              <span className="info-value">{valuation.bear_value.toLocaleString()} 元/坪</span>
            </div>
            <div className="info-row highlight">
              <span className="info-label">Base (P50)</span>
              <span className="info-value">{valuation.base_value.toLocaleString()} 元/坪</span>
            </div>
            <div className="info-row">
              <span className="info-label">Bull (P75)</span>
              <span className="info-value">{valuation.bull_value.toLocaleString()} 元/坪</span>
            </div>
            <div className="info-row">
              <span className="info-label">信心度</span>
              <span className="info-value">{valuation.confidence}</span>
            </div>
          </div>
        ) : valuation?.status === 'INSUFFICIENT_DATA' ? (
          <div className="insufficient-data">
            <p>Insufficient Data</p>
            <p>可比交易不足，無法估價</p>
          </div>
        ) : (
          <p className="inspector-empty">尚未執行估價</p>
        )}
      </section>

      {/* Provenance — SPEC §11, §39-40 */}
      <section className="inspector-section">
        <div className="inspector-section-header">
          <h3 className="inspector-section-title">資料溯源</h3>
          <button className="inspector-nav" onClick={onViewProvenance}>
            查看溯源 →
          </button>
        </div>
        <ProvenanceSummary provenance={provenance} />
      </section>
    </aside>
  );
};

/** Compact provenance summary — SPEC §39-40 */
const ProvenanceSummary: React.FC<{ provenance?: ProvenanceChain }> = ({ provenance }) => {
  if (!provenance || !provenance.chain || provenance.chain.length === 0) {
    return <p className="inspector-empty">無溯源資料</p>;
  }
  const first = provenance.chain[0];
  return (
    <div className="provenance-summary">
      <div className="provenance-row">
        <span className="info-label">來源</span>
        <span className="info-value">{first.source || '—'}</span>
      </div>
      <div className="provenance-row">
        <span className="info-label">資料集</span>
        <span className="info-value">{first.snapshot_id || '—'}</span>
      </div>
      {provenance.query_hash && (
        <div className="provenance-row">
          <span className="info-label">查詢哈希</span>
          <span className="info-value">
            <code>{provenance.query_hash.slice(0, 16)}…</code>
          </span>
        </div>
      )}
    </div>
  );
};
