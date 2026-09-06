/**
 * MapView — map workspace component.
 *
 * Per SPEC §10-12: supports pan, zoom, parcel boundary, selection, layer control.
 * Per SPEC §12.1: clicking a parcel identifies it and opens the Parcel Inspector.
 * Per SPEC §14: map ↔ list synchronization.
 *
 * Works with both Google Maps (provider='google') and Leaflet (provider='leaflet').
 */

import React, { useEffect, useRef, useState } from 'react';
import L from 'leaflet';
import { useMap as useGoogleMap } from '../hooks/useMap';
import { useGoogleMaps } from '../hooks/useGoogleMaps';
import { useLeafletMap } from '../hooks/useLeafletMap';
import type {
  ParcelGeometry,
  Transaction,
  RoadSegment,
  ComparableResult,
  MapContext,
  ViewData,
  ResponseMetadata,
} from '../types';
import { LeafletParcelLayer } from './LeafletParcelLayer';
import { LeafletRoadLayer } from './LeafletRoadLayer';
import { LeafletComparableLayer } from './LeafletComparableLayer';
import { ComparableLayer } from './ComparableLayer';
import { LeafletTransactionMarkers } from './LeafletTransactionMarkers';
import { ParcelLayer } from './ParcelLayer';
import { RoadLayer } from './RoadLayer';
import { TransactionMarkers } from './TransactionMarkers';
import './MapView.css';
import 'leaflet/dist/leaflet.css';

interface MapViewProps {
  parcel?: ParcelGeometry;
  transactions: Transaction[];
  roads: RoadSegment[];
  comparables: ComparableResult[];
  showSatellite: boolean;
  showStreetView: boolean;
  showNLSC: boolean;
  showRoads: boolean;
  showComparables: boolean;
  showTransactions: boolean;
  mapContext?: MapContext;
}

const MAP_PROVIDER = (typeof window !== 'undefined' &&
  (window as unknown as { RUNTIME_CONFIG?: { MAP_PROVIDER?: string } }).RUNTIME_CONFIG?.MAP_PROVIDER) ||
  'leaflet';

const MapView: React.FC<MapViewProps> = ({
  parcel,
  transactions,
  roads,
  comparables,
  showSatellite,
  showStreetView,
  showNLSC,
  showRoads,
  showComparables,
  showTransactions,
  mapContext,
}) => {
  const mapRef = useRef<HTMLDivElement>(null);
  const [leafletMap, setLeafletMap] = useState<L.Map | null>(null);
  const [googleMapRef, setGoogleMapRef] = useState<google.maps.Map | null>(null);
  const [googleApi, setGoogleApi] = useState<typeof google | null>(null);

  const data: ViewData = {
    parcel,
    transactions,
    roads,
    comparables,
    map_context: mapContext,
    metadata: {} as ResponseMetadata,
  };

  // Google Maps hooks — only used when provider is 'google'
  const { initializeMap: initGoogleMap } = useGoogleMap({
    mapRef,
    data,
    showSatellite,
    showStreetView,
    showNLSC,
  });
  const { isLoaded, error: mapsError } = useGoogleMaps();

  // Leaflet hooks — only used when provider is 'leaflet'
  const { initializeMap: initLeafletMap } = useLeafletMap({
    mapRef,
    data,
    showSatellite,
    showStreetView,
    showNLSC,
  });

  // Initialize the appropriate map provider
  useEffect(() => {
    if (MAP_PROVIDER === 'google') {
      if (isLoaded && mapRef.current) {
        void initGoogleMap();
      }
    } else {
      void initLeafletMap();
    }
  }, [isLoaded, initGoogleMap, initLeafletMap, showSatellite, showStreetView, showNLSC, parcel, transactions, roads, comparables, mapContext]);

  // Capture map instance from map-ready event
  useEffect(() => {
    const handleMapReady = (e: CustomEvent) => {
      const mapInstance = e.detail?.map;
      if (mapInstance instanceof L.Map) {
        setLeafletMap(mapInstance);
      } else if (mapInstance && typeof mapInstance.setCenter === 'function') {
        setGoogleMapRef(mapInstance);
        setGoogleApi(window.google);
      }
    };
    window.addEventListener('map-ready', handleMapReady as EventListener);
    return () => {
      window.removeEventListener('map-ready', handleMapReady as EventListener);
    };
  }, []);

  // Render Google Maps layers
  const renderGoogleLayers = () => {
    if (MAP_PROVIDER !== 'google' || !isLoaded || !googleMapRef || !googleApi || !parcel) return null;
    return (
      <>
        <ParcelLayer google={googleApi} map={googleMapRef} geometry={parcel} visible={true} />
        {showRoads && <RoadLayer google={googleApi} map={googleMapRef} roads={roads} visible={true} />}
        {showTransactions && (
          <TransactionMarkers google={googleApi} map={googleMapRef} transactions={transactions} visible={true} />
        )}
        {comparables && <ComparableLayer google={googleApi} map={googleMapRef} comparables={comparables} />}
      </>
    );
  };

  // Render Leaflet layers
  const renderLeafletLayers = () => {
    if (MAP_PROVIDER !== 'leaflet' || !leafletMap) return null;
    return (
      <>
        <LeafletParcelLayer map={leafletMap} parcel={parcel ?? null} />
        {showRoads && <LeafletRoadLayer map={leafletMap} roads={roads} />}
        {showTransactions && (
          <LeafletTransactionMarkers map={leafletMap} transactions={transactions} />
        )}
        {showComparables && <LeafletComparableLayer map={leafletMap} comparables={comparables} />}
      </>
    );
  };

  if (MAP_PROVIDER === 'google' && mapsError) {
    return (
      <div className="map-error">
        <p>Google Maps could not be loaded:</p>
        <p>{mapsError}</p>
        <p>Set VITE_GOOGLE_MAPS_API_KEY or use MAP_PROVIDER=leaflet.</p>
      </div>
    );
  }

  return (
    <div className="map-container">
      {!isLoaded && MAP_PROVIDER === 'google' && (
        <div className="map-loading">Loading map…</div>
      )}
      <div ref={mapRef} className="map-canvas" />
      {renderGoogleLayers()}
      {renderLeafletLayers()}
    </div>
  );
};

export default MapView;
