import React, { useEffect, useRef } from 'react';
import { useMap } from '../hooks/useMap';
import { useGoogleMaps } from '../hooks/useGoogleMaps';
import type { ParcelGeometry, Transaction, RoadSegment, ComparableResult, MapContext, ResponseMetadata } from '../types';
import './MapView.css';

interface MapViewProps {
  parcel?: ParcelGeometry;
  transactions: Transaction[];
  roads: RoadSegment[];
  comparables: ComparableResult[];
  showSatellite: boolean;
  showStreetView: boolean;
  showNLSC: boolean;
  mapContext?: MapContext;
}

const MapView: React.FC<MapViewProps> = ({
  parcel,
  transactions,
  roads,
  comparables,
  showSatellite,
  showStreetView,
  showNLSC,
  mapContext,
}) => {
  const mapRef = useRef<HTMLDivElement>(null);
  const { isLoaded, error: mapsError } = useGoogleMaps();

  const { initializeMap } = useMap({
    mapRef,
    data: {
      parcel,
      transactions,
      roads,
      comparables,
      map_context: mapContext,
      metadata: {} as ResponseMetadata,
    },
    showSatellite,
    showStreetView,
    showNLSC,
  });

  useEffect(() => {
    if (isLoaded && mapRef.current) {
      void initializeMap();
    }
  }, [isLoaded, initializeMap]);

  if (mapsError) {
    return (
      <div className="map-error">
        <p>Google Maps could not be loaded:</p>
        <p>{mapsError}</p>
        <p>Set VITE_GOOGLE_MAPS_API_KEY in your .env file.</p>
      </div>
    );
  }

  return (
    <div className="map-container">
      {!isLoaded && <div className="map-loading">Loading map…</div>}
      <div ref={mapRef} className="map-canvas" />
    </div>
  );
};

export default MapView;
