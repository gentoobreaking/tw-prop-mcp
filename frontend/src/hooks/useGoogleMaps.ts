import { useEffect, useState } from 'react';
import { loadGoogleMapsWithRetry } from '../services/createRetryableLoader';

interface UseGoogleMapsResult {
  isLoaded: boolean;
  error: string | null;
}

export function useGoogleMaps(): UseGoogleMapsResult {
  const [isLoaded, setIsLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    loadGoogleMapsWithRetry()
      .then(() => {
        if (!cancelled) {
          setIsLoaded(true);
          setError(null);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
          setIsLoaded(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return { isLoaded, error };
}
