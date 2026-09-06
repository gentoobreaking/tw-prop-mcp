/**
 * EmptyState — per SPEC §42 empty state patterns for no-data scenarios.
 * Shows context-appropriate messages with suggestions.
 */

import React from 'react';
import './EmptyState.css';

interface EmptyStateProps {
  title?: string;
  message?: string;
  suggestions?: string[];
  icon?: React.ReactNode;
  size?: 'sm' | 'md' | 'lg';
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  title = 'No data available',
  message,
  suggestions,
  icon,
  size = 'md',
}) => {
  return (
    <div className={`empty-state ${size}`}>
      <div className="empty-state-icon">
        {icon ?? (
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="16" x2="12" y2="12" />
            <line x1="12" y1="12" x2="12.01" y2="12" />
          </svg>
        )}
      </div>
      <h3 className="empty-state-title">{title}</h3>
      {message && <p className="empty-state-message">{message}</p>}
      {suggestions && suggestions.length > 0 && (
        <ul className="empty-state-suggestions">
          {suggestions.map((suggestion, index) => (
            <li key={index}>{suggestion}</li>
          ))}
        </ul>
      )}
    </div>
  );
};

/** Predefined empty states for common scenarios */

export const EmptyTransactions: React.FC = () => (
  <EmptyState
    title="No transactions found"
    message="Try:"
    suggestions={[
      'expanding date range',
      'increasing area range',
      'changing section',
    ]}
  />
);

export const EmptyComparables: React.FC = () => (
  <EmptyState
    title="No comparable transactions found"
    message="Comparable analysis requires sufficient nearby transactions."
  />
);

export const EmptySearchResults: React.FC = () => (
  <EmptyState
    title="No parcels found"
    message="Try a broader search with county and district."
  />
);

export const EmptyValuation: React.FC = () => (
  <EmptyState
    title="No valuation data available"
    message="Valuation will be calculated when a parcel with sufficient comparables is selected."
  />
);
