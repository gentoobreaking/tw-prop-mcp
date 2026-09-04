import { useEffect, useCallback, RefObject } from 'react';
import { loadGoogleMapsWithRetry } from '../services/createRetryableLoader';
import type { ViewData } from '../types';

interface UseMapOptions {
  mapRef: RefObject<HTMLDivElement>;
  data: ViewData;
  showSatellite: boolean;
  showStreetView: boolean;
  showNLSC: boolean;
}

export function useMap({ mapRef, data, showSatellite, showStreetView, showNLSC }: UseMapOptions) {
  const initializeMap = useCallback(async () => {
    if (!mapRef.current) return;

    try {
      const google = await loadGoogleMapsWithRetry();

      const map = new google.maps.Map(mapRef.current, {
        center: { lat: 23.5, lng: 121.0 },
        zoom: 8,
        mapTypeId: google.maps.MapTypeId.ROADMAP,
        streetViewControl: false,
      });

      if (showStreetView) {
        const panorama = new google.maps.StreetViewPanorama(document.createElement('div'), {
          position: { lat: 25.03, lng: 121.51 },
          pov: { heading: 0, pitch: 0 },
          visible: true,
        });
        map.setStreetView(panorama);
      }

      if (showSatellite) {
        map.setMapTypeId(google.maps.MapTypeId.SATELLITE);
      }

      if (showNLSC) {
        const nslcLayer = new google.maps.ImageMapType({
          getTileUrl: (coord: google.maps.Point, zoom: number) => {
            return `https://maps.nlsc.gov.tw/SMCSHORT/map.ashx?z=${zoom}&x=${coord.x}&y=${coord.y}`;
          },
          tileSize: new google.maps.Size(256, 256),
          opacity: 0.5,
        });
        map.overlayMapTypes.push(nslcLayer);
      }

      window.dispatchEvent(new CustomEvent('map-ready', { detail: { map } }));
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error('Failed to initialize map:', err);
      window.dispatchEvent(new CustomEvent('map-error', { detail: { error: msg } }));
    }
  }, [mapRef, showSatellite, showStreetView, showNLSC]);

  useEffect(() => {
    if (data && mapRef.current) {
      void initializeMap();
    }
  }, [showSatellite, showStreetView, showNLSC, data, initializeMap, mapRef]);

  return { initializeMap };
}
