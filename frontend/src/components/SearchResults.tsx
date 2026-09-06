/**
 * SearchResults — displays parcel search results as a ranked list.
 *
 * Per SPEC §9: results shown as list with county/district/section/land_number/area.
 * Per SPEC §7.5: deterministic ordering (exact → normalized → prefix → partial).
 * Per SPEC §9.1: selecting a result centers map, highlights geometry, opens inspector.
 */

import React from 'react';
import type { ParcelSummary } from '../types';
import './SearchResults.css';

export interface SearchResultItem {
  parcel_id: string;
  county: string;
  district: string;
  section: string;
  land_number: string;
  area_sqm: number;
  urban_zoning?: string;
  centroid?: { lat: number; lng: number };
}

interface SearchResultsProps {
  results: ParcelSummary[];
  totalCount: number;
  onSelect: (parcel: ParcelSummary) => void;
  loading?: boolean;
  emptyMessage?: string;
}

/** Format area: show in ping (台坪) per SPEC §8.1 */
function formatAreaPing(sqm: number): string {
  const ping = sqm / 3.305785;
  return `${ping.toFixed(2)} 坪`;
}

export const SearchResults: React.FC<SearchResultsProps> = ({
  results,
  totalCount,
  onSelect,
  loading,
  emptyMessage = '沒有找到地號結果',
}) => {
  if (loading) {
    return (
      <div className="search-results">
        <div className="search-results-loading">搜尋中…</div>
      </div>
    );
  }

  if (!results || results.length === 0) {
    return (
      <div className="search-results">
        <div className="search-results-empty">{emptyMessage}</div>
      </div>
    );
  }

  return (
    <div className="search-results">
      <div className="search-results-header">
        <span className="search-results-count">
          共找到 {totalCount} 筆 ({results.length} 筆顯示)
        </span>
      </div>
      <div className="search-results-list">
        {results.map((parcel, index) => (
          <button
            key={parcel.parcel_id || `${parcel.county}-${parcel.district}-${parcel.section}-${parcel.land_number}-${index}`}
            className="search-result-item"
            onClick={() => onSelect(parcel)}
          >
            <div className="search-result-main">
              <span className="search-result-rank">#{index + 1}</span>
              <span className="search-result-location">
                {parcel.county} {parcel.district} {parcel.section} {parcel.land_number}
              </span>
            </div>
            <div className="search-result-meta">
              <span className="search-result-area">{formatAreaPing(parcel.area_sqm)}</span>
              {parcel.urban_zoning && (
                <span className="search-result-zoning">{parcel.urban_zoning}</span>
              )}
            </div>
          </button>
        ))}
      </div>
    </div>
  );
};
