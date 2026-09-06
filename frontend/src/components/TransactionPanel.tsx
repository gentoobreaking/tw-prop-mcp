/**
 * TransactionPanel — transaction list, detail, and statistics.
 *
 * Per SPEC §18-21: count, list, detail, statistics.
 * Per SPEC §67: values must match API response exactly.
 * Per SPEC §74: frontend does not compute statistics — reads from backend.
 */

import React from 'react';
import type { Transaction, TransactionStatistics } from '../types';
import './TransactionPanel.css';

interface TransactionPanelProps {
  transactions: Transaction[];
  statistics: TransactionStatistics | null;
  loading: boolean;
  error: string | null;
  selectedTransaction: Transaction | null;
  onSelect: (tx: Transaction) => void;
}

/** Format price in NT$ */
function formatPrice(price: number): string {
  return `NT$ ${price.toLocaleString()}`;
}

/** Format unit price: per ping and per sqm */
function formatUnitPrice(tx: Transaction): string {
  const perPing = tx.price_per_ping ?? (tx.unit_price * 3.305785);
  return `${perPing.toFixed(0)} 元/坪 (${tx.unit_price.toLocaleString()} 元/㎡)`;
}

export const TransactionPanel: React.FC<TransactionPanelProps> = ({
  transactions,
  statistics,
  loading,
  error,
  selectedTransaction,
  onSelect,
}) => {
  if (loading) {
    return (
      <div className="transaction-panel">
        <div className="transaction-loading">載入交易中…</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="transaction-panel">
        <div className="transaction-error">{error}</div>
      </div>
    );
  }

  if (!transactions || transactions.length === 0) {
    return (
      <div className="transaction-panel">
        <EmptyTransactionState />
      </div>
    );
  }

  return (
    <div className="transaction-panel">
      {statistics && (
        <StatisticsSection stats={statistics} />
      )}

      <div className="transaction-list">
        {transactions.map((tx) => (
          <button
            key={tx.transaction_id}
            className={`transaction-item ${
              selectedTransaction?.transaction_id === tx.transaction_id ? 'selected' : ''
            }`}
            onClick={() => onSelect(tx)}
          >
            <div className="tx-main">
              <span className="tx-id">{tx.transaction_id.slice(0, 8)}…</span>
              <span className="tx-date">{tx.transaction_date}</span>
            </div>
            <div className="tx-details">
              <span className="tx-price">{formatPrice(tx.total_price)}</span>
              <span className="tx-unit-price">{formatUnitPrice(tx)}</span>
              {tx.transaction_type && <span className="tx-type">{tx.transaction_type}</span>}
            </div>
          </button>
        ))}
      </div>

      {selectedTransaction && (
        <TransactionDetail tx={selectedTransaction} />
      )}
    </div>
  );
};

/** Statistics display — SPEC §21 */
const StatisticsSection: React.FC<{ stats: TransactionStatistics }> = ({ stats }) => {
  const pricePerPing = stats.price_per_ping;
  if (!pricePerPing) return null;

  return (
    <div className="statistics-section">
      <h4 className="statistics-title">價格統計 ({stats.count} 筆)</h4>
      <div className="statistics-grid">
        <StatRow label="Min" value={pricePerPing.min.toLocaleString()} unit="元/坪" />
        <StatRow label="P10" value={pricePerPing.p10.toLocaleString()} unit="元/坪" />
        <StatRow label="P25" value={pricePerPing.p25.toLocaleString()} unit="元/坪" />
        <StatRow label="Median" value={pricePerPing.median.toLocaleString()} unit="元/坪" />
        <StatRow label="Mean" value={pricePerPing.mean.toFixed(0)} unit="元/坪" />
        <StatRow label="P75" value={pricePerPing.p75.toLocaleString()} unit="元/坪" />
        <StatRow label="P90" value={pricePerPing.p90.toLocaleString()} unit="元/坪" />
        <StatRow label="Max" value={pricePerPing.max.toLocaleString()} unit="元/坪" />
      </div>
    </div>
  );
};

const StatRow: React.FC<{ label: string; value: string; unit: string }> = ({
  label,
  value,
  unit,
}) => (
  <div className="stat-row">
    <span className="stat-label">{label}</span>
    <span className="stat-value">{value}</span>
    <span className="stat-unit">{unit}</span>
  </div>
);

/** Transaction detail — SPEC §20 */
const TransactionDetail: React.FC<{ tx: Transaction }> = ({ tx }) => (
  <div className="transaction-detail">
    <h4 className="tx-detail-title">交易明細</h4>
    <div className="tx-detail-grid">
      <DetailRow label="交易日期" value={tx.transaction_date} />
      <DetailRow label="縣市" value={tx.county} />
      <DetailRow label="行政區" value={tx.district} />
      <DetailRow label="段名" value={tx.section ?? '—'} />
      <DetailRow label="地號" value={tx.land_number ?? '—'} />
      <DetailRow label="交易類型" value={tx.transaction_type} />
      <DetailRow label="總價" value={formatPrice(tx.total_price)} />
      <DetailRow label="單價" value={formatUnitPrice(tx)} />
      {tx.land_area_sqm !== undefined && (
        <DetailRow label="土地面積" value={`${tx.land_area_sqm.toFixed(2)} ㎡`} />
      )}
      {tx.building_area_sqm !== undefined && (
        <DetailRow label="建物面積" value={`${tx.building_area_sqm.toFixed(2)} ㎡`} />
      )}
      {tx.urban_zoning && <DetailRow label="使用分區" value={tx.urban_zoning} />}
      {tx.land_use_category && <DetailRow label="使用地類別" value={tx.land_use_category} />}
      {tx.building_type && <DetailRow label="建物種類" value={tx.building_type} />}
      {tx.floor !== undefined && <DetailRow label="樓層" value={tx.floor} />}
      {tx.age !== undefined && <DetailRow label="屋齡" value={`${tx.age} 年`} />}
    </div>
  </div>
);

const DetailRow: React.FC<{ label: string; value: string | number }> = ({ label, value }) => (
  <div className="tx-detail-row">
    <span className="tx-detail-label">{label}</span>
    <span className="tx-detail-value">{value}</span>
  </div>
);

const EmptyTransactionState: React.FC = () => (
  <div className="transaction-empty">
    <p>No transactions found.</p>
    <p className="transaction-empty-hint">
      Try:
      <ul>
        <li>expanding date range</li>
        <li>increasing area range</li>
        <li>changing section</li>
      </ul>
    </p>
  </div>
);
