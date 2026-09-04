import React from 'react';
import type { ParcelGeometry } from '../types';

interface ParcelLayerProps {
  google: typeof google;
  map: google.maps.Map;
  parcel: ParcelGeometry | null;
}

export const ParcelLayer: React.FC<ParcelLayerProps> = ({ google, map, parcel }) => {
  React.useEffect(() => {
    if (!parcel || !google || !map) return;

    const paths = parcel.geometry.coordinates.map((poly) =>
      poly[0].map((coord) => new google.maps.LatLng(coord[1], coord[0])),
    );

    const polygons = paths.map(
      (path) =>
        new google.maps.Polygon({
          map,
          paths: [path],
          strokeColor: '#e94560',
          strokeWeight: 3,
          strokeOpacity: 0.8,
          fillColor: '#e94560',
          fillOpacity: 0.2,
        }),
    );

    return () => {
      polygons.forEach((p) => p.setMap(null));
    };
  }, [google, map, parcel]);

  return null;
};
