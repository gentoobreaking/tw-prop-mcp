/**
 * Type contracts mirror SPEC.md §3 (Tool Categories) and T017 acceptance criteria.
 * All types represent structured MCP tool outputs — the frontend never
 * constructs raw SQL or PostGIS queries (P4/P18 AI Isolation).
 */

// --- Geometry Types ---

/** GeoJSON Polygon (coordinates in [lng, lat] = EPSG:4326) */
export interface GeoPolygon {
  type: 'Polygon';
  coordinates: number[][][];
}

/** GeoJSON MultiPolygon */
export interface GeoMultiPolygon {
  type: 'MultiPolygon';
  coordinates: number[][][][];
}

/** GeoJSON Point */
export interface GeoPoint {
  type: 'Point';
  coordinates: [number, number];
}

/** GeoJSON MultiLineString for roads */
export interface GeoMultiLineString {
  type: 'MultiLineString';
  coordinates: number[][][];
}

/** Any GeoJSON geometry the frontend renders */
export type GeoGeometry = GeoPoint | GeoPolygon | GeoMultiPolygon | GeoMultiLineString;

/** WGS84 coordinate pair — MCP returns lat/lng (EPSG:4326) */
export interface LatLng {
  lat: number;
  lng: number;
}

/** Bounding box for map framing */
export interface LatLngBounds {
  northeast: LatLng;
  southwest: LatLng;
}

// --- Domain Types ---

/** Parcel info from get_parcel / search_parcels */
export interface Parcel {
  parcel_id: string;
  county: string;
  district: string;
  section: string;
  land_number: string;
  area_sqm: number;
  urban_zoning?: string;
  land_use_category?: string;
  centroid?: LatLng;
  bbox?: LatLngBounds;
}

/** Parcel geometry from get_parcel_geometry */
export interface ParcelGeometry {
  parcel_id: string;
  geometry: GeoMultiPolygon;
  centroid: LatLng;
  bbox: LatLngBounds;
  area_sqm: number;
}

/** Single transaction record */
export interface Transaction {
  transaction_id: string;
  snapshot_id: string;
  transaction_date: string; // ISO date
  transaction_type: string;
  county: string;
  district: string;
  section?: string;
  land_number?: string;
  total_price: number;
  unit_price: number;
  land_area_sqm?: number;
  building_area_sqm?: number;
  urban_zoning?: string;
  non_urban_zoning?: string;
  land_use_category?: string;
  building_type?: string;
  floor?: string;
  age?: number;
  parking_area_sqm?: number;
  parking_price?: number;
  location?: LatLng; // derived from section/land_number centroid
}

/** Road segment from find_nearby_roads / road_access */
export interface RoadSegment {
  road_id: string;
  name?: string;
  road_class?: string;
  width_m?: number;
  width_source: string;
  geometry: GeoMultiLineString;
  distance_m?: number;
  access_type?: 'ROAD_ADJACENT' | 'ROAD_NEARBY' | 'NO_ROAD_DETECTED' | 'UNKNOWN';
}

/** Comparable transaction result */
export interface ComparableResult {
  transaction: Transaction;
  score: number;
  area_similarity: number;
  distance_m: number;
  time_score: number;
  distance_score: number;
  zoning_match: boolean;
  land_use_match: boolean;
  road_access_match: boolean;
}

/** Valuation result */
export interface ValuationResult {
  valuation_id: string;
  target_parcel_id: string;
  snapshot_id: string;
  bear_value: number;
  base_value: number;
  bull_value: number;
  confidence: 'HIGH' | 'MEDIUM' | 'LOW' | 'INSUFFICIENT';
  comparable_count: number;
  algorithm_version: string;
  configuration_version: string;
  outlier_method: string;
  statistics: Record<string, unknown>;
  comparable_ids: string[];
}

/** Map context from get_parcel_map_context */
export interface MapContext {
  latitude: number;
  longitude: number;
  zoom: number;
  bounds?: {
    north: number;
    south: number;
    east: number;
    west: number;
  };
}

/** MCP response metadata envelope */
export interface ResponseMetadata {
  algorithm_version: string;
  snapshot_id: string;
  generatedAt: string; // RFC3339 — excluded from hash
  query_hash: string;
  configuration_version?: string;
  outlier_method?: string;
}

/** Combined data loaded from MCP for the MapView */
export interface ViewData {
  parcel?: ParcelGeometry;
  transactions: Transaction[];
  roads: RoadSegment[];
  comparables: ComparableResult[];
  valuation?: ValuationResult;
  map_context?: MapContext;
  metadata: ResponseMetadata;
}

/** NLSC GIS layer config */
export interface NLSCLayerConfig {
  baseUrl: string;
  layers: string[];
  zoomMin: number;
  zoomMax: number;
}
