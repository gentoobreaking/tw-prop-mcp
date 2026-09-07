import React from 'react';
import L from 'leaflet';
import type { RoadSegment, NearbyRoad } from '../types';

interface LeafletRoadLayerProps {
  map: L.Map | null;
  roads: (RoadSegment | NearbyRoad)[];
}

export const LeafletRoadLayer: React.FC<LeafletRoadLayerProps> = ({ map, roads }) => {
  React.useEffect(() => {
    if (!roads.length || !map) return;

    const layers: L.Polyline[] = [];

    roads.forEach((road) => {
      if (road.geometry) {
        road.geometry.coordinates.forEach((line) => {
          const latlngs = line.map((coord) => [coord[1], coord[0]] as [number, number]);
          const polyline = L.polyline(latlngs, {
            color: '#333',
            weight: road.width_m ? Math.max(1, road.width_m / 2) : 2,
            opacity: 0.7,
          }).addTo(map);
          layers.push(polyline);
        });
      }
    });

    return () => {
      layers.forEach((l) => map.removeLayer(l));
    };
  }, [map, roads]);

  return null;
};
