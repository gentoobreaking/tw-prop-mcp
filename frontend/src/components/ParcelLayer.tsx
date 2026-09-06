/**
 * ParcelLayer — renders the target parcel polygon on the Google Map.
 *
 * Geometry is sourced from MCP `get_parcel_geometry` as WKT MULTIPOLYGON string.
 * The frontend only converts coordinate representation, never computes geometry.
 * (SPEC §4.8: Google Maps is visualization only.)
 */

import { useEffect, useRef } from 'react';
import { parcelGeometryToLatLngPaths, parcelGeometryToGoogleBounds } from '../services/mapService';
import type { ParcelGeometry } from '../types';

export interface ParcelLayerProps {
  map: google.maps.Map | null;
  google: typeof google | null;
  geometry: ParcelGeometry | null;
  visible: boolean;
  fitBounds?: boolean;
  onBoundsFit?: (bounds: { north: number; south: number; east: number; west: number }) => void;
}

export function ParcelLayer({
  map,
  google,
  geometry,
  visible,
  fitBounds = true,
  onBoundsFit,
}: ParcelLayerProps) {
  const polygonRef = useRef<google.maps.Polygon | null>(null);

  useEffect(() => {
    if (!map || !google) return;

    // Clean up previous polygon
    if (polygonRef.current) {
      polygonRef.current.setMap(null);
      polygonRef.current = null;
    }

    if (!visible || !geometry) return;

    // Parse WKT MULTIPOLYGON into LatLng paths
    const paths = parcelGeometryToLatLngPaths(geometry);
    if (!paths || paths.length === 0) return;

    // Convert LatLng to google.maps.LatLngLiteral
    const googlePaths = paths.map((path) =>
      path.map((p) => ({ lat: p.lat, lng: p.lng })),
    );

    polygonRef.current = new google.maps.Polygon({
      paths: googlePaths,
      strokeColor: '#1976d2',
      strokeOpacity: 0.85,
      strokeWeight: 3,
      fillColor: '#1976d2',
      fillOpacity: 0.25,
      map,
      clickable: false,
      zIndex: 10,
    });

    // Optionally fit the map viewport to the parcel bounds
    if (fitBounds) {
      const gBounds = parcelGeometryToGoogleBounds(geometry);
      if (gBounds) {
        const googleBounds = new google.maps.LatLngBounds(
          { lat: gBounds.southwest.lat, lng: gBounds.southwest.lng },
          { lat: gBounds.northeast.lat, lng: gBounds.northeast.lng },
        );
        map.fitBounds(googleBounds, 48);
        onBoundsFit?.({
          north: gBounds.northeast.lat,
          south: gBounds.southwest.lat,
          east: gBounds.northeast.lng,
          west: gBounds.southwest.lng,
        });
      }
    }

    return () => {
      if (polygonRef.current) {
        polygonRef.current.setMap(null);
        polygonRef.current = null;
      }
    };
  }, [map, google, geometry, visible, fitBounds, onBoundsFit]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (polygonRef.current) {
        polygonRef.current.setMap(null);
      }
    };
  }, []);

  return null;
}
