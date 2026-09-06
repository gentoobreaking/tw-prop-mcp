/**
 * LeafletComparableLayer — renders comparable transactions as markers on Leaflet map.
 *
 * Per SPEC §24: comparable transactions displayed on map.
 * Note: ComparableResult does not contain location data — only scoring info.
 * No markers to render since the backend returns comparable IDs, not geometries.
 */

import React from 'react';
import type { ComparableResult } from '../types';

interface LeafletComparableLayerProps {
  map: unknown;
  comparables: ComparableResult[];
}

/**
 * Renders comparable transactions as distinct markers on Leaflet map.
 * Colored differently from regular transactions.
 * Note: ComparableResult does not contain location data — only scoring info.
 * Markers are not rendered since the backend returns comparable IDs, not geometries.
 */
export const LeafletComparableLayer: React.FC<LeafletComparableLayerProps> = ({ map }) => {
  React.useEffect(() => {
    // Comparables do not include geometry data from the backend;
    // no markers to render. Placeholder for future when comparable
    // results include transaction locations.
    void map;
    return () => {};
  }, [map]);

  return null;
};
