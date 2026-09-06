/**
 * ParcelLayer — renders the target parcel polygon on the Google Map.
 *
 * Geometry is sourced from the MCP `get_parcel_geometry` tool (PostGIS-backed).
 * The frontend only converts coordinate representation, never computes geometry.
 * (SPEC §4.8: Google Maps is visualization only.)
 */

import { useEffect, useRef } from 'react';
import { parcelGeometryToBounds, parcelGeometryToPath } from '../services/mapService';
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

    const paths = parcelGeometryToPath(geometry);
    if (paths.length === 0) return;

    polygonRef.current = new google.maps.Polygon({
      paths,
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
      const bounds = parcelGeometryToBounds(geometry);
      const gBounds = new google.maps.LatLngBounds(
        { lat: bounds.south, lng: bounds.west },
        { lat: bounds.north, lng: bounds.east }
      );
      map.fitBounds(gBounds, 48);
      onBoundsFit?.(bounds);
    }
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
