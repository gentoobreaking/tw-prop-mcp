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
 */
export const ComparableLayer: React.FC<ComparableLayerProps> = ({
  google,
  map,
  comparables,
}) => {
  React.useEffect(() => {
    if (!comparables.length || !google || !map) return;

    const markers: google.maps.Marker[] = [];

    comparables.forEach((comp) => {
      if (comp.transaction?.location) {
        const marker = new google.maps.Marker({
          map,
          position: { lat: comp.transaction.location.lat, lng: comp.transaction.location.lng },
          title: `Comparable: score ${comp.score.toFixed(2)}`,
          icon: {
            path: google.maps.SymbolPath.CIRCLE,
            fillColor: '#1e88e5',
            fillOpacity: 0.9,
            strokeColor: '#0d47a1',
            strokeWeight: 1,
            scale: 6,
          },
        });
        markers.push(marker);
      }
    });

    return () => {
      markers.forEach((m) => m.setMap(null));
    };
  }, [google, map, comparables]);

  return null;
};
