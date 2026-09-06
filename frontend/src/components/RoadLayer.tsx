/**
 * RoadLayer — renders nearby road segments as polylines on the Google Map.
 *
 * Road geometry originates from the MCP `find_nearby_roads` / `check_road_access`
 * tools (GIS engine). The frontend renders but does not compute road widths.
 */

import { useEffect, useRef } from 'react';
import type { LineStringGeometry, NearbyRoad, RoadSegment } from '../types';

export interface RoadLayerProps {
  map: google.maps.Map | null;
  google: typeof google | null;
  roads: (NearbyRoad | RoadSegment)[] | null;
  visible: boolean;
}

export function RoadLayer({ map, google, roads, visible }: RoadLayerProps) {
  const polylinesRef = useRef<google.maps.Polyline[]>([]);

  const clearPolylines = () => {
    polylinesRef.current.forEach((p) => p.setMap(null));
    polylinesRef.current = [];
  };

  useEffect(() => {
    if (!map || !google || !visible || !roads) {
      clearPolylines();
      return;
    }

    const newPolylines: google.maps.Polyline[] = [];

    for (const road of roads) {
      const lineString = extractLineString(road);
      if (!lineString) continue;

      const path = lineString.coordinates.map((coord) => ({
        lat: coord[1],
        lng: coord[0],
      }));

      const width = 'width_m' in road && road.width_m ? road.width_m : undefined;
      const polyline = new google.maps.Polyline({
        path,
        strokeColor: '#616161',
        strokeOpacity: 0.7,
        strokeWeight: width ? Math.max(2, Math.min(width, 8)) : 3,
        map,
        clickable: false,
        zIndex: 5,
      });
      newPolylines.push(polyline);
    }

    polylinesRef.current = newPolylines;

    return () => clearPolylines();
  }, [map, google, roads, visible]);

  // Cleanup on unmount
  useEffect(() => clearPolylines, []);

  return null;
}

/** Extract a LineString geometry from either NearbyRoad or RoadSegment. */
function extractLineString(road: NearbyRoad | RoadSegment): LineStringGeometry | null {
  if ('geometry' in road && road.geometry) {
    return road.geometry;
  }
  return null;
}
