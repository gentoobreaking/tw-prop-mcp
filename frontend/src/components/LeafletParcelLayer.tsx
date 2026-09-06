import React from 'react';
import L from 'leaflet';
import type { ParcelGeometry } from '../types';

interface LeafletParcelLayerProps {
  map: L.Map | null;
  parcel: ParcelGeometry | null;
}

export const LeafletParcelLayer: React.FC<LeafletParcelLayerProps> = ({ map, parcel }) => {
  React.useEffect(() => {
    if (!parcel || !map) return;

    const paths = parcel.geometry.coordinates.map((poly) =>
      poly[0].map((coord) => [coord[1], coord[0]] as [number, number])
    );

    const polygons = paths.map((path) =>
      L.polygon(path, {
        color: '#e94560',
        weight: 3,
        opacity: 0.8,
        fillColor: '#e94560',
        fillOpacity: 0.2,
      }).addTo(map)
    );

    return () => {
      polygons.forEach((p) => map.removeLayer(p));
    };
  }, [map, parcel]);

  return null;
};
