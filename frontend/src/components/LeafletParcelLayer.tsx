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

    // MCP returns geometry as WKT string; Leaflet needs LatLng[][]
    let geojson: GeoJSON.Polygon | GeoJSON.MultiPolygon;
    if (typeof parcel.geometry === 'string') {
      // Parse WKT
      const wkt = parcel.geometry;
      const coords = wkt
        .replace('MULTIPOLYGON(((', '')
        .replace(')))', '')
        .split(',')
        .map(pair => pair.trim().split(' ').map(Number) as [number, number]);
      geojson = {
        type: 'MultiPolygon',
        coordinates: [[coords.map(c => [c[0], c[1]] as [number, number])]]
      };
    } else {
      geojson = parcel.geometry as unknown as GeoJSON.MultiPolygon;
    }

    const layer = L.geoJSON(geojson, {
      style: {
        color: '#e94560',
        weight: 3,
        opacity: 0.8,
        fillColor: '#e94560',
        fillOpacity: 0.2,
      },
    }).addTo(map);

    return () => { map.removeLayer(layer); };
  }, [map, parcel]);

  return null;
};
