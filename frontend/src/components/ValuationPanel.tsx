import React from 'react';
import type { ValuationResult, ComparableResult, ResponseMetadata } from '../types';
import './ValuationPanel.css';

interface ValuationPanelProps {
  valuation?: ValuationResult;
  comparables: ComparableResult[];
  metadata: ResponseMetadata;
}

/**
 * Displays valuation results and comparable transaction list.
 * Pure display — no computation (P4/P18: frontend is visualization only).
 */
export const ValuationPanel: React.FC<ValuationPanelProps> = ({
  valuation,
  comparables,
  metadata,
}) => {
  if (!valuation) {
    return (
      <div className="valuation-panel">
        <h3>Valuation</h3> <p className="no-data">No valuation data available</p>
      </div>
    );
  }

  const confidenceColors = {
    HIGH: '#16a34a',
    MEDIUM: '#ca8a04',
    LOW: '#ea580c',
    INSUFFICIENT: '#dc2626',
  };

  return (
    <div className="valuation-panel">
      <h3>Valuation Result</h3>

      <div className="valuation-values">
        <div className="value-row">
          <span>Bear (P25)</span>
          <span className="value">{valuation.bear_value.toLocaleString()}</span>
        </div>
        <div className="value-row highlight">
          <span>Base (P50)</span>
          <span className="value">{valuation.base_value.toLocaleString()}</span>
        </div>
        <div className="value-row">
          <span>Bull (P75)</span>
          <span className="value">{valuation.bull_value.toLocaleString()}</span>
        </div>
      </div>

      <div className="confidence-badge">
        <span
          className="confidence-dot"
          style={{ backgroundColor: confidenceColors[valuation.confidence] }}
        />
        Confidence: {valuation.confidence}
      </div>

      <div className="valuation-meta">
        <div>Algorithm: {valuation.algorithm_version}</div>
        <div>Config: {valuation.configuration_version}</div>
        <div>Comparables: {valuation.comparable_count}</div>
        <div>
          Query Hash: <code>{metadata.query_hash.slice(0, 16)}…</code>
        </div>
      </div>

      {comparables.length > 0 && (
        <div className="comparable-list">
          <h4>Comparable Transactions</h4>
          {comparables.map((c) => (
            <div key={c.transaction.transaction_id} className="comparable-item">
              <div className="score">{c.score.toFixed(2)}</div>
              <div className="details">
                <div>
                  {c.transaction.county} {c.transaction.district}
                </div>
                <div className="price">NT$ {c.transaction.total_price.toLocaleString()}</div>
                <div className="distance">{c.distance_m.toFixed(0)} m</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default ValuationPanel;
