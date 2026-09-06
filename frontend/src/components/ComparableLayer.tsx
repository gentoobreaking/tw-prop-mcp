import React from 'react';
import type { ComparableResult } from '../types';

interface ComparableLayerProps {
  google: typeof google;
  map: google.maps.Map;
  comparables: ComparableResult[];
}

/**
 * Renders comparable transactions as distinct markers on the map.
 * Colored differently from regular transactions.
 * Note: ComparableResult does not contain location data — only scoring info.
 * Markers are not rendered since the backend returns comparable IDs, not geometries.
 */
export const ComparableLayer: React.FC<ComparableLayerProps> = ({ map }) => {
  React.useEffect(() => {
    // Comparables do not include geometry data from the backend;
    // no markers to render. The layer is a placeholder for future
    // when comparable results include transaction locations.
    void map;
    return () => {};
  }, [map]);

  return null;
};
