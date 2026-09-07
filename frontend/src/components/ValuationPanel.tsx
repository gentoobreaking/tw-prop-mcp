/**
 * ValuationPanel — displays land valuation results and comparable analysis.
 *
 * Per SPEC §26-30: Bear/Base/Bull values, confidence, comparable count,
 * valuation inputs, provenance (query hash).
 * Per SPEC §29: if insufficient data, show explicit message — NEVER fabricate.
 * Per SPEC §74: frontend does NOT calculate confidence or valuation — backend only.
 * Per SPEC §46: no hardcoded valuation values.
 */

import type { ValuationResult, ComparableResult, ResponseMetadata, ValuationExplanation } from '../types';
import './ValuationPanel.css';

interface ValuationPanelProps {
  valuation: ValuationResult | null | undefined;
  comparables: ComparableResult[];
  metadata?: ResponseMetadata;
  loading?: boolean;
  error?: string | null;
  onExplain?: (valuationId: string) => void;
  explanation?: ValuationExplanation | null;
  onViewProvenance?: () => void;
}
const CONFIDENCE_COLORS: Record<string, string> = {
  HIGH: '#16a34a',
  MEDIUM: '#ca8a04',
  LOW: '#ea580c',
  INSUFFICIENT: '#dc2626',
};

const CONFIDENCE_LABELS: Record<string, string> = {
  HIGH: '高',
  MEDIUM: '中',
  LOW: '低',
  INSUFFICIENT: '不足',
};

export const ValuationPanel: React.FC<ValuationPanelProps> = ({
  valuation,
  comparables,
  metadata,
  loading = false,
  error,
  onExplain,
  explanation,
  onViewProvenance,
}) => {
  if (loading) {
    return (
      <div className="valuation-panel">
        <h3>Valuation</h3>
        <div className="valuation-loading">計算估價中…</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="valuation-panel">
        <h3>Valuation</h3>
        <div className="valuation-error">{error}</div>
      </div>
    );
  }

  if (!valuation) {
    return (
      <div className="valuation-panel">
        <h3>Valuation</h3>
        <p className="no-data">No valuation data available</p>
      </div>
    );
  }

  // SPEC §29: Insufficient data — must NOT fabricate a value
  if (valuation.status === 'INSUFFICIENT_DATA') {
    return (
      <div className="valuation-panel">
        <h3>Valuation</h3>
        <div className="insufficient-data">
          <p>Insufficient Data</p>
          <p>
            {comparables.length > 0
              ? `Available: ${comparables.length} comparable${comparables.length === 1 ? '' : 's'}`
              : 'No comparables available'}
          </p>
          <p className="insufficient-reason">
            Not enough comparable transactions to compute a reliable valuation.
          </p>
        </div>
      </div>
    );
  }

  const confidenceColor = CONFIDENCE_COLORS[valuation.confidence] ?? '#6b7280';
  const confidenceLabel = CONFIDENCE_LABELS[valuation.confidence] ?? valuation.confidence;

  return (
    <div className="valuation-panel">
      <h3>Estimated Value</h3>

      <div className="valuation-values">
        <div className="value-row">
          <span>Bear (P25)</span>
          <span className="value">{valuation.bear_value.toLocaleString()} 元/坪</span>
        </div>
        <div className="value-row highlight">
          <span>Base (P50)</span>
          <span className="value">{valuation.base_value.toLocaleString()} 元/坪</span>
        </div>
        <div className="value-row">
          <span>Bull (P75)</span>
          <span className="value">{valuation.bull_value.toLocaleString()} 元/坪</span>
        </div>
      </div>

      {/* Confidence bar — spec §26 example */}
      <div className="confidence-section">
        <div
          className="confidence-badge"
          style={{ backgroundColor: `${confidenceColor}20`, color: confidenceColor }}
        >
          <span
            className="confidence-dot"
            style={{ backgroundColor: confidenceColor }}
          />
          Confidence: {confidenceLabel}
        </div>
        <div className="confidence-bar-container">
          <div
            className="confidence-bar-fill"
            style={{
              width: `${Math.min(100, Math.max(0, (valuation.confidence === 'HIGH' ? 90 : valuation.confidence === 'MEDIUM' ? 70 : valuation.confidence === 'LOW' ? 50 : 0)))}%`,
              backgroundColor: confidenceColor,
            }}
          />
        </div>
      </div>

      {/* Valuation inputs — SPEC §27 */}
      <div className="valuation-meta">
        <div>Comparables: {comparables.length}</div>
        <div>Algorithm: {valuation.algorithm_version ?? '—'}</div>
        <div>Config: {valuation.configuration_version ?? '—'}</div>
        <div>Outlier Method: {valuation.outlier_method ?? '—'}</div>
        {metadata?.query_hash && (
          <div>
            Query Hash: <code>{metadata.query_hash.slice(0, 16)}…</code>
          </div>
        )}
        {valuation.query_hash && (
          <div>
            Valuation Hash: <code>{valuation.query_hash.slice(0, 16)}…</code>
          </div>
        )}
      </div>

      {/* Comparable summary in valuation panel */}
      {comparables.length > 0 && (
        <div className="comparable-list">
          <h4>Comparable Transactions</h4>
          {comparables.map((c, index) => (
            <div key={c.id} className="comparable-item">
              <div className="score">#{index + 1} | Score: {(c.total_score * 100).toFixed(1)}%</div>
              <div className="details">
                <div>候選交易 #{c.candidate_transaction_id}</div>
                <div className="distance">{c.distance_m.toFixed(0)} m away</div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Explanation link */}
      {onExplain && valuation.id && (
        <div className="valuation-actions">
          <button
            className="valuation-explain-btn"
            onClick={() => onExplain(valuation.id)}
          >
            查看估價說明
          </button>
          {onViewProvenance && (
            <button
              className="valuation-provenance-btn"
              onClick={onViewProvenance}
            >
              查看溯源
            </button>
          )}
        </div>
      )}

      {/* Valuation explanation — methodology and outlier handling */}
      {explanation && (
        <div className="valuation-explanation">
          <h4>估價說明</h4>
          {explanation.methodology && (
            <p className="explanation-methodology">{explanation.methodology}</p>
          )}
          {explanation.explanation && (
            <p className="explanation-text">{explanation.explanation}</p>
          )}
        </div>
      )}
    </div>
  );
};

export default ValuationPanel;
