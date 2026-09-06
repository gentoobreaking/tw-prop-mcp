/**
 * RoadPanel — road access and GIS information.
 *
 * Per SPEC §31-34: nearest road, distance, road width (with source),
 * road adjacency (ADJACENT / NOT_ADJACENT / UNKNOWN — never convert),
 * GIS status (VALID / GIS_AREA_MISMATCH / GEOMETRY_NOT_FOUND / etc.).
 * Per SPEC §74: frontend does not compute road classification.
 */

import React from 'react';
import type { RoadAccessResult, NearbyRoad } from '../types';
import './RoadPanel.css';

interface RoadPanelProps {
  roadAccess: RoadAccessResult | null;
  nearbyRoads: NearbyRoad[];
  loading: boolean;
  error: string | null;
  parcelLocation?: { lat: number; lng: number } | null;
  onOpenStreetView?: (location: { lat: number; lng: number }) => void;
}

const ACCESS_LABELS: Record<string, string> = {
  ROAD_ADJACENT: 'ADJACENT',
  ROAD_NEARBY: 'NEARBY',
  NO_ROAD_DETECTED: 'NO_ROAD_DETECTED',
  UNKNOWN: 'UNKNOWN',
};

const STATUS_COLORS: Record<string, string> = {
  VALID: '#16a34a',
  GIS_AREA_MISMATCH: '#ca8a04',
  GEOMETRY_NOT_FOUND: '#dc2626',
  SOURCE_UNAVAILABLE: '#dc2626',
  UNKNOWN: '#9ca3af',
};

export const RoadPanel: React.FC<RoadPanelProps> = ({
  roadAccess,
  nearbyRoads,
  loading,
  error,
  parcelLocation,
  onOpenStreetView,
}) => {
  if (loading) {
    return (
      <div className="road-panel">
        <div className="road-loading">載入道路資訊中…</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="road-panel">
        <div className="road-error">{error}</div>
      </div>
    );
  }

  return (
    <div className="road-panel">
      <h3 className="road-section-title">道路 / GIS 資訊</h3>

      {/* Road Adjacency — SPEC §32 */}
      <div className="road-section">
        <h4 className="road-sub-title">道路臨接</h4>
        <div className="road-adjacency">
          <span
            className="adjacency-badge"
            style={{
              backgroundColor: getStatusColor(roadAccess?.status ?? 'UNKNOWN'),
              color: 'white',
            }}
          >
            {ACCESS_LABELS[roadAccess?.status ?? 'UNKNOWN'] ?? roadAccess?.status ?? 'UNKNOWN'}
          </span>
        </div>
      </div>

      {/* Road Distance — SPEC §33 */}
      {roadAccess && (
        <div className="road-section">
          <h4 className="road-sub-title">道路資訊</h4>
          <div className="road-info-grid">
            {roadAccess.distance_m !== undefined && (
              <InfoRow label="距離" value={`${roadAccess.distance_m.toFixed(1)} m`} />
            )}
            {roadAccess.road_width_m !== undefined && (
              <InfoRow
                label="道路寬度"
                value={`${roadAccess.road_width_m.toFixed(1)} m`}
                source={roadAccess.source}
              />
            )}
            {roadAccess.road_width_m === undefined && (
              <InfoRow label="道路寬度" value="Not Available" />
            )}
            {roadAccess.source && (
              <InfoRow label="資料來源" value={roadAccess.source} />
            )}
          </div>
        </div>
      )}

      {/* Nearby Roads */}
      {nearbyRoads && nearbyRoads.length > 0 && (
        <div className="road-section">
          <h4 className="road-sub-title">附近道路</h4>
          <div className="nearby-roads">
            {nearbyRoads.map((road, index) => (
              <div key={road.road_id || index} className="nearby-road-item">
                <span className="nearby-road-name">{road.name ?? road.road_id}</span>
                {road.distance_m !== undefined && (
                  <span className="nearby-road-distance">{road.distance_m.toFixed(0)} m</span>
                )}
                {road.width_m !== undefined && (
                  <span className="nearby-road-width">{road.width_m.toFixed(1)} m 寬</span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Street View — SPEC §36-37 */}
      {parcelLocation && onOpenStreetView && (
        <div className="road-section">
          <h4 className="road-sub-title">街景</h4>
          <button
            className="street-view-button"
            onClick={() => onOpenStreetView(parcelLocation)}
          >
            開啟街景
          </button>
        </div>
      )}
    </div>
  );
};

const InfoRow: React.FC<{
  label: string;
  value: string | number;
  source?: string;
}> = ({ label, value, source }) => (
  <div className="road-info-row">
    <span className="road-info-label">{label}</span>
    <span className="road-info-value">{value}</span>
    {source && <span className="road-info-source">Source: {source}</span>}
  </div>
);

function getStatusColor(status: string): string {
  return STATUS_COLORS[status] ?? STATUS_COLORS.UNKNOWN;
}
