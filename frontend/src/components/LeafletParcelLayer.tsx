/**
 * LeafletParcelLayer — renders the target parcel polygon on the Leaflet map.
 *
 * Per SPEC §12: selected parcel geometry must be visually distinguishable.
 * Per SPEC §12.1: clicking a parcel identifies it and opens Parcel Inspector.
 *
 * Geometry comes from MCP `get_parcel_geometry` as WKT MULTIPOLYGON string.
 * Coordinates are in EPSG:4326 (lng, lat) when epsg=4326 is requested.
 */

import React from 'react';
import L from 'leaflet';
import type { ParcelGeometry } from '../types';
import { wktMultiPolygonToPaths, parcelCentroid } from '../services/mapService';

interface LeafletParcelLayerProps {
  map: L.Map | null;
  parcel: ParcelGeometry | null;
  onParcelClick?: () => void;
}

export const LeafletParcelLayer: React.FC<LeafletParcelLayerProps> = ({ map, parcel, onParcelClick }) => {
  React.useEffect(() => {
    if (!parcel || !map) return;

    const paths = wktMultiPolygonToPaths(parcel.geometry);
    if (!paths || paths.length === 0) return;

    // Build GeoJSON from the parsed WKT paths
    const geojson: GeoJSON.GeoJSON = {
      type: 'MultiPolygon',
      coordinates: paths.map((path) => [path.map((p) => [p.lng, p.lat])]),
    };

    const layer = L.geoJSON(geojson, {
      style: {
        color: '#1a73e8',
        weight: 3,
        opacity: 0.9,
        fillColor: '#1a73e8',
        fillOpacity: 0.15,
      },
    }).addTo(map);

    // Bind click handler to open Parcel Inspector (SPEC §12.1)
    if (onParcelClick) {
      layer.on('click', () => {
        onParcelClick();
      });
    }

    // Fit map to parcel bounds
    const bounds = layer.getBounds();
    if (bounds.isValid()) {
      map.flyToBounds(bounds, { padding: [50, 50] });
    }

    // Center map on centroid if available (fallback)
    const centroid = parcelCentroid(parcel);
    if (centroid) {
      if (!bounds.isValid()) {
        map.setView([centroid.lat, centroid.lng], 16);
      }
    }

    return () => {
      map.removeLayer(layer);
    };
  }, [map, parcel, onParcelClick]);

  return null;
};
