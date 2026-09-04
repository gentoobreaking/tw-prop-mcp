import React from 'react';
import type { RoadSegment } from '../types';

interface RoadLayerProps {
  google: typeof google;
  map: google.maps.Map;
  roads: RoadSegment[];
}

/**
 * Renders road segments as polylines on the map.
 * Width proportional to road width_m.
 */
export const RoadLayer: React.FC<RoadLayerProps> = ({ google, map, roads }) => {
  React.useEffect(() => {
    if (!roads.length || !google || !map) return;

    const polylines: google.maps.Polyline[] = [];

    roads.forEach((road) => {
      if (road.geometry) {
        road.geometry.coordinates.forEach((line) => {
          const polyline = new google.maps.Polyline({
            map,
            path: line.map((coord) => new google.maps.LatLng(coord[1], coord[0])),
            strokeColor: '#333',
            strokeWeight: road.width_m ? Math.max(1, road.width_m / 2) : 2,
            strokeOpacity: 0.7,
          });
          polylines.push(polyline);
        });
      }
    });

    return () => {
      polylines.forEach((p) => p.setMap(null));
    };
  }, [google, map, roads]);

  return null;
};
