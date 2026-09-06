/**
 * LeafletParcelLayer — renders the target parcel polygon on the Leaflet map.
 *
 * Per SPEC §12: selected parcel geometry must be visually distinguishable.
 * Per SPEC §12.1: clicking a parcel identifies it and opens Parcel Inspector.
 *
 * Geometry comes from MCP `get_parcel_geometry` as GeoJSON MultiPolygon
 * (coordinates in [lng, lat] = EPSG:4326).
 */

import React from 'react';
import L from 'leaflet';
import type { ParcelGeometry } from '../types';

interface LeafletParcelLayerProps {
  map: L.Map | null;
  parcel: ParcelGeometry | null;
  onParcelClick?: () => void;
}

export const LeafletParcelLayer: React.FC<LeafletParcelLayerProps> = ({ map, parcel, onParcelClick }) => {
  React.useEffect(() => {
    if (!parcel || !map) return;

    const geometry = parcel.geometry;

    // Build GeoJSON from the geometry
    let geojson: GeoJSON.GeoJSON;

    if (geometry && typeof geometry === 'object' && 'type' in geometry) {
      geojson = geometry as GeoJSON.GeoJSON;
    } else {
      // Fallback: if geometry is a WKT string, parse it
      // This handles backward compat with older MCP responses
      const wkt = String(geometry);
      if (wkt.startsWith('MULTIPOLYGON')) {
        const cleaned = wkt
          .replace('MULTIPOLYGON((', '')
          .replace('))', '')
          .trim();
        const coords = cleaned
          .split('),(')
          .map((ring) =>
            ring
              .replace('(', '')
              .replace(')', '')
              .trim()
              .split(',')
              .map((pair) => {
                const [lng, lat] = pair.trim().split(' ').map(Number);
                return [lng, lat] as [number, number];
              }),
          );
        geojson = {
          type: 'MultiPolygon',
          coordinates: [coords],
        };
      } else {
        return; // Cannot parse
      }
    }

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

    return () => {
      map.removeLayer(layer);
    };
  }, [map, parcel, onParcelClick]);

  return null;
};
