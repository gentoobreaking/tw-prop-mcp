/**
 * ComparablePanel — comparable transaction analysis with ranking.
 *
 * Per SPEC §22-23: shows comparable transactions with similarity scores,
 * distance, area similarity, time/zoning/land-use/road match scores.
 * Per SPEC §23: frontend must NOT independently reorder — backend ranking
 * is authoritative.
 * Per SPEC §74: frontend does not compute scores.
 */

import React from 'react';
import type { ComparableResult } from '../types';
import './ComparablePanel.css';

interface ComparablePanelProps {
  comparables: ComparableResult[];
  loading: boolean;
  error: string | null;
  selectedComparable: ComparableResult | null;
  onSelect: (comp: ComparableResult) => void;
}

export const ComparablePanel: React.FC<ComparablePanelProps> = ({
  comparables,
  loading,
  error,
  selectedComparable,
  onSelect,
}) => {
  if (loading) {
    return (
      <div className="comparable-panel">
        <div className="comparable-loading">計算可比交易中…</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="comparable-panel">
        <div className="comparable-error">{error}</div>
      </div>
    );
  }

  if (!comparables || comparables.length === 0) {
    return (
      <div className="comparable-panel">
        <div className="comparable-empty">
          <p>No comparable transactions found.</p>
          <p className="comparable-empty-hint">
            Try selecting a parcel to see comparable analysis.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="comparable-panel">
      <div className="comparable-header">
        <span className="comparable-count">{comparables.length} 個可比交易</span>
      </div>

      <div className="comparable-list">
        {comparables.map((comp, index) => (
          <ComparableItem
            key={comp.candidate_transaction_id || index}
            rank={index + 1}
            comparable={comp}
            isSelected={selectedComparable?.candidate_transaction_id === comp.candidate_transaction_id}
            onSelect={() => onSelect(comp)}
          />
        ))}
      </div>

      {selectedComparable && (
        <ComparableDetail comparable={selectedComparable} />
      )}
    </div>
  );
};

interface ComparableItemProps {
  rank: number;
  comparable: ComparableResult;
  isSelected: boolean;
  onSelect: () => void;
}

const ComparableItem: React.FC<ComparableItemProps> = ({
  rank,
  comparable,
  isSelected,
  onSelect,
}) => {
  const scorePercent = (comparable.total_score * 100).toFixed(1);

  return (
    <div
      className={`comparable-item ${isSelected ? 'selected' : ''}`}
      onClick={onSelect}
    >
      <div className="comparable-rank">#{rank}</div>
      <div className="comparable-score">{scorePercent}</div>

      <div className="comparable-info">
        <div className="comparable-location">
          交易 {comparable.candidate_transaction_id.slice(0, 8)}
        </div>
      </div>

      <div className="comparable-metrics">
        <Metric label="距離" value={`${comparable.distance_m.toFixed(0)} m`} />
        <Metric label="面積相似" value={`${(comparable.area_similarity * 100).toFixed(0)}%`} />
      </div>
    </div>
  );
};

const Metric: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div className="comparable-metric">
    <span className="metric-label">{label}</span>
    <span className="metric-value">{value}</span>
  </div>
);

/** Detailed comparable breakdown — SPEC §22 */
const ComparableDetail: React.FC<{ comparable: ComparableResult }> = ({ comparable }) => (
  <div className="comparable-detail">
    <h4 className="comparable-detail-title">可比分析</h4>
    <div className="comparable-scores">
      <ScoreBar label="總分" score={comparable.total_score} />
      <ScoreBar label="距離" score={comparable.distance_score} />
      <ScoreBar label="面積" score={comparable.area_similarity_score} />
      <ScoreBar label="時間" score={comparable.time_score} />
    </div>
    <div className="comparable-matches">
      <MatchLabel name="分區匹配" matched={comparable.zoning_match} />
      <MatchLabel name="使用地類別" matched={comparable.land_use_match} />
      <MatchLabel name="道路臨接" matched={comparable.road_access_match} />
    </div>
  </div>
);

const ScoreBar: React.FC<{ label: string; score: number }> = ({ label, score }) => {
  const pct = Math.max(0, Math.min(100, score * 100));
  return (
    <div className="score-bar">
      <span className="score-label">{label}</span>
      <div className="score-track">
        <div className="score-fill" style={{ width: `${pct}%` }} />
      </div>
      <span className="score-value">{score.toFixed(3)}</span>
    </div>
  );
};

const MatchLabel: React.FC<{ name: string; matched: boolean }> = ({ name, matched }) => (
  <div className="match-label">
    <span
      className={`match-badge ${matched ? 'matched' : 'unmatched'}`}
      aria-label={matched ? `${name}: 匹配` : `${name}: 不匹配`}
    >
      {name}: {matched ? '✓' : '✗'}
    </span>
  </div>
);
