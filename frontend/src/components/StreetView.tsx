/**
 * StreetView — Google Maps Street View panorama panel.
 *
 * Per SPEC §36-37: accessible from Parcel Inspector, Road Panel, and Map.
 * Hidden when MAP_PROVIDER=leaflet (no Street View support).
 * Per SPEC §1093: if no panorama, show friendly message — never broken iframe.
 * Per SPEC §85: no hardcoded production addresses.
 */

import React, { useEffect, useRef, useState } from 'react';
import './StreetView.css';

interface StreetViewProps {
  visible: boolean;
  location: { lat: number; lng: number } | null;
  google: typeof google | null;
  isGoogleLoaded: boolean;
  provider: 'google' | 'leaflet';
}

export const StreetView: React.FC<StreetViewProps> = ({
  visible,
  location,
  google,
  isGoogleLoaded,
  provider,
}) => {
  const panoramaRef = useRef<HTMLDivElement>(null);
  const panoramaInstanceRef = useRef<google.maps.StreetViewPanorama | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  const [loading, setLoading] = useState(false);

  // All hooks must be called unconditionally — moved before any early returns
  useEffect(() => {
    // Guard inside the effect: skip if any prerequisite is missing
    if (
      !visible ||
      !isGoogleLoaded ||
      !google ||
      !location ||
      !panoramaRef.current ||
      provider !== 'google'
    ) {
      return;
    }

    setLoading(true);
    setUnavailable(false);

    // Clean up previous panorama instance
    if (panoramaInstanceRef.current) {
      panoramaInstanceRef.current.setVisible(false);
      panoramaInstanceRef.current = null;
    }

    try {
      const panorama = new google.maps.StreetViewPanorama(
        panoramaRef.current,
        {
          position: { lat: location.lat, lng: location.lng },
          pov: { heading: 0, pitch: 0 },
          visible: true,
          addressControl: true,
          linksControl: true,
          panControl: true,
          enableCloseButton: false,
        },
      );

      // Check for panorama availability
      const sv = new google.maps.StreetViewService();
      let checkCancelled = false;

      sv.getPanorama(
        { location: { lat: location.lat, lng: location.lng }, radius: 50 },
        (data, status) => {
          if (checkCancelled) return;
          if (status === 'OK') {
            setUnavailable(false);
          } else {
            // No panorama available — show friendly message (SPEC §1093)
            setUnavailable(true);
            panorama.setVisible(false);
          }
          setLoading(false);
        },
      );

      panoramaInstanceRef.current = panorama;

      return () => {
        checkCancelled = true;
        if (panoramaInstanceRef.current) {
          panoramaInstanceRef.current.setVisible(false);
          panoramaInstanceRef.current = null;
        }
      };
    } catch {
      setUnavailable(true);
      setLoading(false);
    }
  }, [visible, isGoogleLoaded, google, location, provider]);

  // Early returns for display logic (hooks already called above)
  if (provider !== 'google') {
    return null;
  }

  if (!visible) {
    return null;
  }

  if (!isGoogleLoaded || !google) {
    return (
      <div className="street-view-panel">
        <div className="street-view-loading">Loading Street View…</div>
      </div>
    );
  }

  if (!location) {
    return (
      <div className="street-view-panel">
        <div className="street-view-empty">
          <p>Street View unavailable — no location selected.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="street-view-panel">
      {loading && <div className="street-view-loading">Searching for panorama…</div>}
      {unavailable && (
        <div className="street-view-unavailable">
          <p>Street View unavailable at this location.</p>
        </div>
      )}
      {!loading && !unavailable && (
        <div ref={panoramaRef} className="street-view-container" />
      )}
    </div>
  );
};
