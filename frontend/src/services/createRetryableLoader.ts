/**
 * Creates a retryable loader for Google Maps API calls.
 * Retries failed loads with exponential backoff.
 */
import { loadGoogleMaps } from './googleMapsLoader';

interface RetryOptions {
  maxRetries?: number;
  initialDelayMs?: number;
  maxDelayMs?: number;
}

const DEFAULT_RETRY_OPTIONS: Required<RetryOptions> = {
  maxRetries: 3,
  initialDelayMs: 1000,
  maxDelayMs: 10000,
};

/**
 * Load Google Maps with automatic retry and exponential backoff.
 */
export async function loadGoogleMapsWithRetry(options: RetryOptions = {}): Promise<typeof google> {
  const opts = { ...DEFAULT_RETRY_OPTIONS, ...options };
  let delay = opts.initialDelayMs;
  let lastError: Error | null = null;

  for (let attempt = 0; attempt <= opts.maxRetries; attempt++) {
    try {
      return await loadGoogleMaps();
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));
      if (attempt < opts.maxRetries) {
        await new Promise((resolve) => setTimeout(resolve, delay));
        delay = Math.min(delay * 2, opts.maxDelayMs);
      }
    }
  }

  throw lastError ?? new Error('Failed to load Google Maps API');
}
