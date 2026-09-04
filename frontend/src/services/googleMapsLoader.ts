/**
 * Dynamic Google Maps JS API loader.
 *
 * API key is read from the Vite-exposed environment variable
 * VITE_GOOGLE_MAPS_API_KEY — never hardcoded.
 *
 * @see https://developers.google.com/maps/documentation/javascript/load-maps-js
 */

const API_KEY = import.meta.env.VITE_GOOGLE_MAPS_API_KEY;
const API_BASE = 'https://maps.googleapis.com/maps/api/js';
const LIBRARIES: string[] = ['marker'];

let loadPromise: Promise<typeof google> | null = null;
let isLoaded = false;

/**
 * Loads the Google Maps JavaScript API with the configured API key.
 * Idempotent: returns a cached promise if already loading.
 */
export function loadGoogleMaps(): Promise<typeof google> {
  if (loadPromise) {
    return loadPromise;
  }

  if (!API_KEY) {
    return Promise.reject(
      new Error(
        'Google Maps API key not configured. Set VITE_GOOGLE_MAPS_API_KEY in your .env file.',
      ),
    );
  }

  loadPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script');
    const params = new URLSearchParams({
      key: API_KEY,
      libraries: LIBRARIES.join(','),
      v: 'weekly',
      callback: 'initGoogleMaps',
      region: 'TW',
      language: 'zh-TW',
    });

    (window as unknown as { initGoogleMaps: () => void }).initGoogleMaps = () => {
      isLoaded = true;
      resolve(google);
    };

    script.src = `${API_BASE}?${params.toString()}`;
    script.async = true;
    script.defer = true;
    script.onerror = () => {
      reject(new Error('Failed to load Google Maps API script'));
    };
    document.head.appendChild(script);
  });

  return loadPromise;
}

/**
 * Check whether the Google Maps API is already loaded.
 */
export function isGoogleMapsLoaded(): boolean {
  return isLoaded;
}

/**
 * Get the google namespace (throws if not loaded yet).
 */
export function getGoogleMaps(): typeof google {
  if (!isLoaded) {
    throw new Error('Google Maps API not loaded. Call loadGoogleMaps() first.');
  }
  return google;
}
