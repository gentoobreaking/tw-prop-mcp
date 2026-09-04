/**
 * Map service — converts MCP domain data into Google Maps renderable types.
 *
 * Converts GeoJSON [lng, lat] coordinate tuples to google.maps.LatLngLiteral.
 * MCP server returns EPSG:4326 (lat/lng) — no coordinate transformation needed.
 */

import type {
  GeoPolygon,
  GeoMultiPolygon,
  GeoMultiLineString,
  GeoPoint,
  LatLng,
  LatLngBounds,
  ParcelGeometry,
  Transaction,
  RoadSegment,
  ComparableResult,
} from '../types';

/** Convert [lng, lat] pair to google.maps.LatLngLiteral */
export function toLatLng(coords: [number, number]): LatLng {
  return { lat: coords[1], lng: coords[0] };
}

/** Convert GeoJSON coordinates array to LatLng literals */
export function toLatLngs(coords: number[][]): LatLng[] {
  return coords.map((c) => toLatLng([c[0], c[1]]));
}

/** Convert a GeoJSON polygon to google.maps path */
export function polygonToPath(poly: GeoPolygon): LatLng[] {
  return toLatLngs(poly.coordinates[0]);
}

/** Convert a GeoJSON MultiPolygon to array of paths */
export function multiPolygonToPaths(mpoly: GeoMultiPolygon): LatLng[][] {
  return mpoly.coordinates.map((poly) => toLatLngs(poly[0]));
}

/** Convert a GeoJSON MultiLineString to array of paths */
export function multiLineStringToPaths(mls: GeoMultiLineString): LatLng[][] {
  return mls.coordinates.map((line) => toLatLngs(line));
}

/** Convert a GeoJSON Point to google.maps.LatLng */
export function pointToLatLng(point: GeoPoint): LatLng {
  return toLatLng(point.coordinates);
}

/** Build google.maps.LatLngBounds from bounds object */
export function boundsToGoogleBounds(bounds: {
  northeast: LatLng;
  southwest: LatLng;
}): google.maps.LatLngBounds {
  return new google.maps.LatLngBounds(bounds.southwest, bounds.northeast);
}

/** Extract all LatLngs from a parcel geometry */
export function parcelGeometryToPaths(parcel: ParcelGeometry): LatLng[][] {
  return multiPolygonToPaths(parcel.geometry);
}

/** Extract marker positions from transactions */
export function transactionsToMarkers(transactions: Transaction[]): LatLng[] {
  return transactions.filter((t) => t.location).map((t) => t.location!);
}

/** Extract road paths from road segments */
export function roadsToPaths(roads: RoadSegment[]): LatLng[][] {
  return roads.filter((r) => r.geometry).flatMap((r) => multiLineStringToPaths(r.geometry));
}

/** Extract comparable marker positions */
export function comparablesToMarkers(comps: ComparableResult[]): LatLng[] {
  return comps.filter((c) => c.transaction?.location).map((c) => c.transaction.location!);
}

/** Compute bounds that encompass all given lat/lng points */
export function computeUnionBounds(points: LatLng[]): LatLngBounds | null {
  if (points.length === 0) return null;

  let minLat = Infinity,
    maxLat = -Infinity;
  let minLng = Infinity,
    maxLng = -Infinity;

  for (const p of points) {
    minLat = Math.min(minLat, p.lat);
    maxLat = Math.max(maxLat, p.lat);
    minLng = Math.min(minLng, p.lng);
    maxLng = Math.max(maxLng, p.lng);
  }

  return {
    northeast: { lat: maxLat, lng: maxLng },
    southwest: { lat: minLat, lng: minLng },
  };
}
