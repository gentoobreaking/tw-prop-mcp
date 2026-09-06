/**
 * ErrorState — per SPEC §43-44, §72 error display.
 *
 * Errors are categorized by code (INVALID_ARGUMENT, PARCEL_NOT_FOUND,
 * NETWORK_ERROR, API_ERROR, etc.) and displayed with user-friendly messages.
 * Developer details (request_id, error_code) shown only in debug mode.
 */

import React from 'react';
import type { McpError } from '../types';
import './ErrorState.css';

export type ErrorCategory =
  | 'NETWORK_ERROR'
  | 'API_ERROR'
  | 'NOT_FOUND'
  | 'INVALID_REQUEST'
  | 'DATA_UNAVAILABLE'
  | 'GIS_ERROR'
  | 'STREET_VIEW_UNAVAILABLE'
  | 'VALUATION_INSUFFICIENT_DATA';

interface ErrorStateProps {
  error: string | null;
  mcpError?: McpError | null;
  onRetry?: () => void;
  onClear?: () => void;
  debug?: boolean;
}

/** Map MCP error codes to frontend error categories per SPEC §43 */
const ERROR_CODE_MAP: Record<string, ErrorCategory> = {
  INVALID_ARGUMENT: 'INVALID_REQUEST',
  PARCEL_NOT_FOUND: 'NOT_FOUND',
  TRANSACTION_NOT_FOUND: 'NOT_FOUND',
  DATA_NOT_AVAILABLE: 'DATA_UNAVAILABLE',
  GIS_NOT_AVAILABLE: 'GIS_ERROR',
  SNAPSHOT_NOT_FOUND: 'NOT_FOUND',
  VALUATION_NOT_AVAILABLE: 'VALUATION_INSUFFICIENT_DATA',
  SOURCE_UNAVAILABLE: 'DATA_UNAVAILABLE',
  INTERNAL_ERROR: 'API_ERROR',
};

/** User-friendly messages per spec §44 */
const CATEGORY_MESSAGES: Record<ErrorCategory, string> = {
  NETWORK_ERROR: 'Unable to connect to the server. Please check your network and try again.',
  API_ERROR: 'An unexpected server error occurred. Please try again.',
  NOT_FOUND: 'The requested data was not found.',
  INVALID_REQUEST: 'Invalid search input. Please check your search terms.',
  DATA_UNAVAILABLE: 'Requested data is not currently available.',
  GIS_ERROR: 'GIS data is unavailable for this parcel.',
  STREET_VIEW_UNAVAILABLE: 'Street View is not available at this location.',
  VALUATION_INSUFFICIENT_DATA: 'Insufficient comparable data for valuation.',
};

export const ErrorState: React.FC<ErrorStateProps> = ({
  error,
  mcpError,
  onRetry,
  onClear,
  debug = false,
}) => {
  if (!error && !mcpError) return null;

  const category = mcpError ? ERROR_CODE_MAP[mcpError.error.code] : 'API_ERROR';
  const userMessage = mcpError
    ? mcpError.error.message
    : error ?? 'An unexpected error occurred.';

  const categoryMessage = category ? CATEGORY_MESSAGES[category] : userMessage;

  return (
    <div className="error-state">
      <div className="error-state-content">
        <div className="error-icon">!</div>
        <h3 className="error-title">{category ?? 'Error'}</h3>
        <p className="error-message">{categoryMessage}</p>

        {debug && mcpError && (
          <div className="error-debug">
            <div className="error-code">
              Code: {mcpError.error.code}
            </div>
            {mcpError.error.request_id && (
              <div className="error-request-id">
                Request ID: {mcpError.error.request_id}
              </div>
            )}
            <div className="error-detail">{userMessage}</div>
          </div>
        )}

        <div className="error-actions">
          {onRetry && (
            <button className="error-retry" onClick={onRetry}>
              Retry
            </button>
          )}
          {onClear && (
            <button className="error-clear" onClick={onClear}>
              Dismiss
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
