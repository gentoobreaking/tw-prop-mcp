/**
 * LeafletComparableLayer — renders comparable transactions as markers on Leaflet map.
 *
 * Per SPEC §24: comparable transactions displayed on map, colored differently
 * from regular transactions.
 */

import React from 'react';
import L from 'leaflet';
import type { ComparableResult } from '../types';

interface LeafletComparableLayerProps {
  map: L.Map | null;
  comparables: ComparableResult[];
}

const comparableIcon = L.divIcon({
  html: '<div style="background:#1e88e5;color:white;border-radius:50%;width:24px;height:24px;display:flex;align-items:center;justify-content:center;font-size:10px;font-weight:bold;"></div>',
  className: 'comparable-icon',
  iconSize: [24, 24],
  iconAnchor: [12, 12],
});

export const LeafletComparableLayer: React.FC<LeafletComparableLayerProps> = ({ map, comparables }) => {
  React.useEffect(() => {
    if (!comparables.length || !map) return;

    const markers: L.Marker[] = [];

    comparables.forEach((comp) => {
      const pos = comp.transaction?.location;
      if (pos) {
        const marker = L.marker([pos.lat, pos.lng], {
          icon: comparableIcon,
          title: `Comparable: score ${(comp.score * 100).toFixed(1)}%`,
        }).addTo(map);

        const popupContent = `
          <div style="min-width:150px;font-size:12px">
            <b>Comparable — Score: ${(comp.score * 100).toFixed(1)}%</b><br/>
            ${comp.transaction.county} ${comp.transaction.district}<br/>
            ${comp.transaction.section ?? ''} ${comp.transaction.land_number ?? ''}<br/>
            $${comp.transaction.total_price.toLocaleString()}<br/>
            ${(comp.distance_m).toFixed(0)} m away
          </div>
        `;
        marker.bindPopup(popupContent);
        markers.push(marker);
      }
    });

    return () => {
      markers.forEach((m) => map.removeLayer(m));
    };
  }, [map, comparables]);

  return null;
};
