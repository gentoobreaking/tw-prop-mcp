/**
 * LoadingState — per-SPEC §41 loading indicator for async operations.
 * Every async operation must have an explicit loading state — no silent waiting.
 */

import React from 'react';
import './LoadingState.css';

interface LoadingStateProps {
  message?: string;
  size?: 'sm' | 'md' | 'lg';
  inline?: boolean;
}

export const LoadingState: React.FC<LoadingStateProps> = ({
  message = '載入中…',
  size = 'md',
  inline = false,
}) => {
  return (
    <div className={`loading-state ${size} ${inline ? 'inline' : ''}`}>
      <div className="loading-spinner">
        <div className="spinner-ring"></div>
      </div>
      {message && <span className="loading-text">{message}</span>}
    </div>
  );
};
