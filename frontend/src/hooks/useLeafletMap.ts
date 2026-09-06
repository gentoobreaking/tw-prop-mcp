/**
 * Hook that initializes and manages a Leaflet map instance directly.
 * Mirrors useMap.ts logic but uses Leaflet API instead of Google Maps.
 */
import { useEffect, useCallback, useRef, RefObject } from 'react';
import L from 'leaflet';
import type { ViewData, ParcelGeometry, Transaction, RoadSegment, MapContext } from '../types';

interface UseLeafletMapOptions {
  mapRef: RefObject<HTMLDivElement>;
  data: ViewData;
  showSatellite: boolean;
  showStreetView: boolean;
  showNLSC: boolean;
}

export function useLeafletMap({ mapRef, data, showSatellite, showStreetView, showNLSC }: UseLeafletMapOptions) {
  // Track map instance to prevent re-initialization
  const mapInstanceRef = useRef<L.Map | null>(null);
  // Store layer refs for update/cleanup
  const layerRefsRef = useRef<{
    osm: L.TileLayer | null;
    satellite: L.TileLayer | null;
    nslc: L.TileLayer | null;
  }>({ osm: null, satellite: null, nslc: null });

  const initializeMap = useCallback(async () => {
    if (!mapRef.current || mapInstanceRef.current) return;

    try {
      // Initialize Leaflet map
      const map = L.map(mapRef.current, {
        center: [23.6936, 120.9815], // Taiwan default
        zoom: 8,
        zoomSnap: 0.5,
        zoomDelta: 0.5,
      });
      mapInstanceRef.current = map;

      // Base layer - OSM Standard
      const osmLayer = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
        maxZoom: 19,
      });

      // Satellite layer - ESRI WorldImagery
      const satelliteLayer = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}', {
        attribution: 'Tiles &copy; Esri',
        maxZoom: 16,
      });

      // NLSC layer — proxied via nginx to bypass CORS
      const nslcLayer = L.tileLayer('/proxy/nlsc/map.ashx?z={z}&x={x}&y={y}', {
        opacity: 0.5,
        maxZoom: 18,
      });

      // Store layer refs
      layerRefsRef.current = { osm: osmLayer, satellite: satelliteLayer, nslc: nslcLayer };

      // Set default base layer
      if (showSatellite) {
        satelliteLayer.addTo(map);
      } else {
        osmLayer.addTo(map);
      }

      if (showNLSC) {
        nslcLayer.addTo(map);
      }

      // Fit bounds if mapContext available
      if (data.map_context) {
        const ctx = data.map_context;
        if (ctx.bounds) {
          const bounds: L.LatLngBoundsExpression = [
            [ctx.bounds.south, ctx.bounds.west],
            [ctx.bounds.north, ctx.bounds.east],
          ];
          map.flyToBounds(bounds, { padding: [50, 50] });
        } else {
          map.setView([ctx.latitude, ctx.longitude], ctx.zoom);
        }
      }

      if (showStreetView) {
        console.warn('[Leaflet] Street View not supported — requires Google Maps');
      }

      window.dispatchEvent(new CustomEvent('map-ready', { detail: { map } }));
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error('[Leaflet] Failed to initialize map:', err);
      window.dispatchEvent(new CustomEvent('map-error', { detail: { error: msg } }));
    }
  }, [mapRef, showSatellite, showNLSC]);

  // Update layer visibility based on toggle changes
  useEffect(() => {
    const map = mapInstanceRef.current;
    const { osm, satellite, nslc } = layerRefsRef.current;
    if (!map || !osm || !satellite) return;

    const baseLayers = [osm, satellite];
    baseLayers.forEach(layer => {
      if (map.hasLayer(layer)) {
        map.removeLayer(layer);
      }
    });

    if (showSatellite) {
      satellite.addTo(map);
    } else {
      osm.addTo(map);
    }

    // Handle NLSC layer
    if (nslc) {
      if (showNLSC && !map.hasLayer(nslc)) {
        nslc.addTo(map);
      } else if (!showNLSC && map.hasLayer(nslc)) {
        map.removeLayer(nslc);
      }
    }
  }, [showSatellite, showNLSC]);

  // Fit to data when mapContext changes
  useEffect(() => {
    const map = mapInstanceRef.current;
    if (!map || !data.map_context) return;

    const ctx = data.map_context;
    if (ctx.bounds) {
      const bounds: L.LatLngBoundsExpression = [
        [ctx.bounds.south, ctx.bounds.west],
        [ctx.bounds.north, ctx.bounds.east],
      ];
      map.flyToBounds(bounds, { padding: [50, 50] });
    } else {
      map.setView([ctx.latitude, ctx.longitude], ctx.zoom);
    }
  }, [data.map_context]);

  // Initial map setup — runs once
  useEffect(() => {
    void initializeMap();
    return () => {
      // Cleanup on unmount
      if (mapInstanceRef.current) {
        mapInstanceRef.current.remove();
        mapInstanceRef.current = null;
      }
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return { initializeMap };
}
