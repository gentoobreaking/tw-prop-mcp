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

/** GeoJSON Feature wrapper */
export interface GeoJSONFeature<T = Record<string, unknown>> {
  type: 'Feature';
  geometry: GeoGeometry;
  properties: T;
}

// --- Domain Types ---

/** Parcel info from get_parcel / search_parcels.
 *  MCP `get_parcel` returns domain.Parcel with fields:
 *  id, county, district, section, land_number, area_sqm, urban_zoning,
 *  land_use_category, geometry, centroid, source, source_version
 */
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
  source?: string;
  source_version?: string;
}

/** Parcel geometry from get_parcel_geometry.
 *  MCP returns: geometry (WKT or GeoJSON), centroid, bbox, area_sqm
 *  When epsg=4326, geometry is in EPSG:4326; otherwise EPSG:3826.
 */
export interface ParcelGeometry {
  parcel_id: string;
  geometry: GeoMultiPolygon;
  centroid: LatLng;
  bbox: LatLngBounds;
  area_sqm: number;
  /** Official cadastral area from the database */
  official_area_sqm?: number;
  /** GIS-calculated area; visible mismatch if differs from official */
  gis_area_sqm?: number;
}

/** Single transaction record from search_transactions / get_transaction.
 *  Maps to service.TransactionData.
 */
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
  price_per_ping?: number; // unit_price * 3.305785
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
  location?: LatLng;
  /** Derived area in ping for display: 1 ping = 3.305785 m² */
  area_ping?: number;
}

/** Road segment from find_nearby_roads / check_road_access */
export interface RoadSegment {
  road_id: string;
  name?: string;
  road_class?: string;
  width_m?: number;
  width_source: string;
  geometry: GeoMultiLineString;
  distance_m?: number;
  access_type?: 'ROAD_ADJACENT' | 'ROAD_NEARBY' | 'NO_ROAD_DETECTED' | 'UNKNOWN';
  /** Nearest point as GeoJSON Point */
  nearest_point?: GeoPoint;
}

/** Road access result from check_road_access */
export interface RoadAccessResult {
  status: 'ROAD_ADJACENT' | 'ROAD_NEARBY' | 'NO_ROAD_DETECTED' | 'UNKNOWN';
  distance_m?: number;
  road_width_m?: number;
  source?: string;
  /** Nearest road GeoJSON line */
  geometry?: GeoMultiLineString;
}

/** Nearby road from find_nearby_roads */
export interface NearbyRoad {
  road_id: string;
  name?: string;
  distance_m?: number;
  width_m?: number;
  width_source?: string;
  road_class?: string;
  geometry?: GeoMultiLineString;
}

/** Comparable transaction result from find_comparable_transactions.
 *  Backend ranking is authoritative — frontend must not reorder.
 */
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
  /** Rank: 1-based, from backend ranking */
  rank?: number;
}

/** Valuation result from estimate_land_value.
 *  Per spec §26: Bear/Base/Bull (per ping, 元/坪), Confidence, Comparable Count.
 */
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
  /** Status: "COMPLETED" or "INSUFFICIENT_DATA" */
  status: 'COMPLETED' | 'INSUFFICIENT_DATA';
  query_hash: string;
  /** Explanation string from explain_valuation */
  explanation?: string;
  /** Methodology description */
  methodology?: string;
  /** Valuation weights info */
  weights?: Record<string, unknown>;
}

/** Map context from get_parcel_map_context */
export interface MapContext {
  latitude: number;
  longitude: number;
  zoom: number;
  parcel_id?: string;
  bounds?: {
    north: number;
    south: number;
    east: number;
    west: number;
  };
}

/** Data provenance chain: Transaction → Snapshot → Official Source */
export interface ProvenanceInfo {
  source: string;
  source_version: string;
  snapshot_id: string;
  downloaded_at?: string;
  file_sha256?: string;
  record_count?: number;
  record_id?: string;
}

/** Provenance chain for a specific data category */
export interface ProvenanceChain {
  target: 'parcel' | 'transaction' | 'gis' | 'road' | 'comparable' | 'valuation';
  chain: ProvenanceInfo[];
  query_hash?: string;
}

/** MCP response metadata envelope (from spec §3.6) */
export interface ResponseMetadata {
  algorithm_version: string;
  snapshot_id: string;
  generated_at: string;
  query_hash: string;
  configuration_version?: string;
  outlier_method?: string;
  request_id?: string;
}

/** Combined data loaded from MCP for a selected parcel view */
export interface ParcelViewData {
  parcel?: ParcelGeometry;
  transactions: Transaction[];
  transaction_statistics?: PriceStats;
  roads: RoadAccessResult[];
  nearby_roads?: NearbyRoad[];
  comparables: ComparableResult[];
  valuation?: ValuationResult;
  map_context?: MapContext;
  metadata: ResponseMetadata;
  provenance: Record<string, ProvenanceChain>;
}

/** Search result from search_parcels */
export interface SearchParcelResult {
  parcels: ParcelSummary[];
  total_count: number;
  limit: number;
  offset: number;
}

/** Compact parcel summary for search results */
export interface ParcelSummary {
  parcel_id: string;
  county: string;
  district: string;
  section: string;
  land_number: string;
  area_sqm: number;
  urban_zoning?: string;
  land_use_category?: string;
  centroid?: LatLng;
}

/** Price statistics from get_transaction_statistics */
export interface PriceStats {
  min: number;
  p10: number;
  p25: number;
  median: number;
  mean: number;
  p75: number;
  p90: number;
  max: number;
}

/** Transaction statistics result */
export interface TransactionStatistics {
  count: number;
  price_per_ping: PriceStats;
  total_price: PriceStats;
  unit_price: PriceStats;
  land_area_sqm: PriceStats;
  building_area_sqm: PriceStats;
  metadata?: ResponseMetadata;
}

/** Valuation explanation from explain_valuation */
export interface ValuationExplanation {
  explanation: string;
  methodology: string;
  comparables?: ComparableResult[];
  provenance?: ProvenanceChain;
}

/** MCP error structure (spec §3.9) */
export interface McpError {
  error: {
    code:
      | 'INVALID_ARGUMENT'
      | 'PARCEL_NOT_FOUND'
      | 'TRANSACTION_NOT_FOUND'
      | 'DATA_NOT_AVAILABLE'
      | 'GIS_NOT_AVAILABLE'
      | 'SNAPSHOT_NOT_FOUND'
      | 'VALUATION_NOT_AVAILABLE'
      | 'SOURCE_UNAVAILABLE'
      | 'INTERNAL_ERROR';
    message: string;
    retryable?: boolean;
    request_id?: string;
  };
}

/** Combined data loaded from MCP for the MapView (legacy compatibility) */
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

/** Normalized parcel identity for search/lookup */
export interface ParcelIdentity {
  county: string;
  district: string;
  section: string;
  landNumber: string;
}
export type MapProvider = 'leaflet' | 'google';


/** Layer visibility state */
export interface LayerState {
  showSatellite: boolean;
  showNLSC: boolean;
  showStreetView: boolean;
  showRoads: boolean;
  showComparables: boolean;
  showTransactions: boolean;
}
