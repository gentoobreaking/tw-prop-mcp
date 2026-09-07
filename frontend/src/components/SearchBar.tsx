/**
 * SearchBar — parcel/address/section search input.
 *
 * Per SPEC §7: supports parcel / address / section / transaction search.
 * Per SPEC §7.3: partial search supported (e.g., "竹篙灣" returns multiple matches).
 * Per SPEC §46: no hardcoded production data.
 */

import React, { useState, useRef, useEffect } from 'react';
import './SearchBar.css';

interface SearchBarProps {
  onSearch: (query: string) => void;
  loading: boolean;
  error: string | null;
  placeholder?: string;
}

export const SearchBar: React.FC<SearchBarProps> = ({
  onSearch,
  loading,
  error,
  placeholder = '搜尋地號 (例: 澎湖縣 西嶼鄉 或 臺北市 中正區)',
}) => {
  const [query, setQuery] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  // Clear error when user starts typing
  useEffect(() => {
    if (error) {
      // Don't auto-clear; let the parent handle it
    }
  }, [query, error]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = query.trim();

    // FE-SEARCH-003: Empty search → validation message, no API request
    if (!trimmed) {
      return;
    }

    void onSearch(trimmed);
  };

  const handleClear = () => {
    setQuery('');
    inputRef.current?.focus();
  };

  return (
    <div className="search-bar-container">
      <form className="search-form" onSubmit={handleSubmit}>
        <input
          ref={inputRef}
          type="text"
          className={`search-input ${error ? 'error' : ''}`}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={placeholder}
          disabled={loading}
          aria-label="搜尋地號、地址或段名"
        />
        {query && (
          <button
            type="button"
            className="search-clear"
            onClick={handleClear}
            aria-label="清除搜尋"
            disabled={loading}
          >
            ×
          </button>
        )}
        <button
          type="submit"
          className="search-submit"
          disabled={loading || !query.trim()}
          aria-label="搜尋"
        >
          {loading ? '搜尋中…' : '搜尋'}
        </button>
      </form>
      {error && <div className="search-error">{error}</div>}
    </div>
  );
};
