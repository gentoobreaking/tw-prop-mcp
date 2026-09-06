/**
 * Map service — converts MCP WKT geometry data into Google Maps renderable types.
 *
 * MCP server returns WKT strings (EPSG:4326 lat/lng). No coordinate transform needed.
 */

import type {
  LatLng,
  ParcelGeometry,
  Transaction,
} from '../types';

/** Convert a single transaction to a marker LatLng position */
export function transactionToMarker(tx: Transaction): LatLng | null {
  return tx.location ?? null;
}

/** Parse WKT POINT string "POINT(lng lat)" → LatLng */
export function wktPointToLatLng(wkt: string): LatLng | null {
  const match = wkt.match(/POINT\s*\(\s*([-\d.]+)\s+([-\d.]+)\s*\)/i);
  if (!match) return null;
  const lng = parseFloat(match[1]);
  const lat = parseFloat(match[2]);
  if (isNaN(lng) || isNaN(lat)) return null;
  return { lat, lng };
}

/** Parse WKT POLYGON string → array of LatLng */
export function wktPolygonToLatLngs(wkt: string): LatLng[] | null {
  const match = wkt.match(/POLYGON\s*\(\s*\((.+)\)\s*\)/is);
  if (!match) return null;
  const ring = match[1].trim();
  return ring.split(',').map((pair) => {
    const [lng, str] = pair.trim().split(/\s+/);
    const lat = parseFloat(str);
    const lngNum = parseFloat(lng);
    return { lat, lng: lngNum };
  });
}

/** Parse WKT MULTIPOLYGON string → array of paths (each path is LatLng[]) */
export function wktMultiPolygonToPaths(wkt: string): LatLng[][] | null {
  // Strip SRID prefix: SRID=3826;MULTIPOLYGON → MULTIPOLYGON
  const cleanWkt = wkt.replace(/^SRID=\d+;/i, '');
  const match = cleanWkt.match(/MULTIPOLYGON\s*\(\s*(.+)\s*\)/is);
  if (!match) return null;

  const inner = match[1].trim();
  // Split on )),( to separate polygons
  const polygons = inner.split(/\)\s*,\s*\(\s*\(/).map((p) => {
    const ring = p.replace(/^\(\(/, '').replace(/\)\s*$/, '').trim();
    return ring.split(',').map((coord) => {
      const [lng, str] = coord.trim().split(/\s+/);
      const lat = parseFloat(str);
      const lngNum = parseFloat(lng);
      return { lat, lng: lngNum };
    });
  }).filter((path) => path.length > 0);

  return polygons.length > 0 ? polygons : null;
}

/** Extract LatLng from a ParcelGeometry's centroid (WKT POINT string) */
export function parcelCentroid(parcel: ParcelGeometry): LatLng | null {
  if (parcel.centroidLatLng) return parcel.centroidLatLng;
  if (parcel.centroid_4326) {
    return wktPointToLatLng(parcel.centroid_4326);
  }
  if (typeof parcel.centroid === 'string') {
    return wktPointToLatLng(parcel.centroid);
  }
  return null;
}

/** Parse WKT MULTIPOLYGON and return LatLng paths for map rendering */
export function parcelGeometryToLatLngPaths(parcel: ParcelGeometry): LatLng[][] | null {
  const wkt = parcel.geometry;
  if (typeof wkt === 'string') {
    return wktMultiPolygonToPaths(wkt);
  }
  return null;
}

