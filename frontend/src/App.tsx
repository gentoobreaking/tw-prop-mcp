/**
 * Main Application — per SPEC §5 Global Layout and §6 Main Application Areas.
 *
 * Layout:
 * ┌────────── Header (App Name + Search + Snapshot + System Status) ──────────┐
 * ├──────────────────┬──────────────────────────────────────────────────────────┤
 * │ Search/Filter    │  MAP                                                     │
 * │ Results          │                                                          │
 * ├──────────────────┴──────────────────────────────────────────────────────────┤
 * │ Detail / Analysis Panel (Parcel Inspector / Transactions / Comparables /  │
 * │ Road / Valuation / Provenance)                                              │
 * └─────────────────────────────────────────────────────────────────────────────┘
 *
 * Desktop-first, minimum 1280×720 (SPEC §4).
 */

import React, { useState, useEffect } from 'react';
import MapView from './components/MapView';
import { SearchBar } from './components/SearchBar';
import { SearchResults } from './components/SearchResults';
import { FilterPanel } from './components/FilterPanel';
import { ParcelInspector } from './components/ParcelInspector';
import { TransactionPanel } from './components/TransactionPanel';
import { ComparablePanel } from './components/ComparablePanel';
import { RoadPanel } from './components/RoadPanel';
import { ValuationPanel } from './components/ValuationPanel';
import { ErrorState } from './components/ErrorState';
import ErrorBoundary from './components/ErrorBoundary';
import { useAppState } from './hooks/useAppState';
import { disconnectMCP } from './services/mcpApi';
import './App.css';

const MAP_PROVIDER = (typeof window !== 'undefined' &&
  (window as unknown as { RUNTIME_CONFIG?: { MAP_PROVIDER?: string } }).RUNTIME_CONFIG?.MAP_PROVIDER) ||
  'leaflet';

type ActivePanel =
  | 'parcel'
  | 'transactions'
  | 'comparables'
  | 'road'
  | 'valuation'
  | 'provenance';

const App: React.FC = () => {
  const {
    // Search
    searchParcels,
    clearSearch,
    searchLoading,
    searchError,
    searchErrorMcp,
    searchResults,
    searchTotalCount,

    // Parcel
    selectedParcelIdentity,
    selectedParcel,
    parcelInfo,
    parcelLoading,
    parcelError,
    parcelErrorMcp,
    loadParcel,

    // Transactions
    transactions,
    transactionStatistics,
    transactionsLoading,
    transactionsError,
    selectedTransaction,
    selectTransaction,
    loadTransaction,

    // Comparables
    comparables,
    comparablesLoading,
    comparablesError,
    selectedComparable,
    selectComparable,

    // Roads / GIS
    roadAccess,
    nearbyRoads,
    roadsLoading,

    // Valuation
    valuation,

    // Map
    mapContext,

    // Provenance
    provenance,

    // Layers
    layers,
    updateLayers,

    // Filters
    filters,
    updateFilters,
    applyFilters,
    resetFilters,

    // Connection
    connectionStatus,
    systemError,
    clearAllErrors,
  } = useAppState();

  const [activePanel, setActivePanel] = useState<ActivePanel>('parcel');

  // Update document title
  useEffect(() => {
    document.title = selectedParcelIdentity
      ? `tw-prop-mcp — ${selectedParcelIdentity.county}${selectedParcelIdentity.district}${selectedParcelIdentity.section} ${selectedParcelIdentity.landNumber}`
      : 'tw-prop-mcp — Taiwan Property Valuation';
  }, [selectedParcelIdentity]);

  // Cleanup MCP on unmount
  useEffect(() => {
    return () => {
      disconnectMCP();
    };
  }, []);

  // Handle search result selection — SPEC §9.1
  const handleSelectResult = (parcel: {
    parcel_id: string;
    county: string;
    district: string;
    section: string;
    land_number: string;
  }) => {
    const identity = {
      county: parcel.county,
      district: parcel.district,
      section: parcel.section,
      landNumber: parcel.land_number,
    };
    void loadParcel(identity);
    clearSearch();
  };

  // Determine overall loading state

  return (
    <ErrorBoundary>
      <div className="app-container">
        {/* Header — SPEC §5.1: App Name + Search + Data Snapshot + System Status */}
        <header className="app-header">
          <h1 className="app-title">Taiwan Real Estate Intelligence</h1>
          <div className="header-system-status">
            {connectionStatus === 'connecting' && <span className="status-badge connecting">Connecting…</span>}
            {connectionStatus === 'connected' && <span className="status-badge connected">● API Connected</span>}
            {connectionStatus === 'disconnected' && (
              <span className="status-badge disconnected">Connection lost, retrying…</span>
            )}
          </div>
        </header>

        {/* System error banner — SPEC §43 */}
        {systemError && (
          <div className="system-error-banner">
            <ErrorState
              error={systemError}
              mcpError={null}
              onClear={clearAllErrors}
            />
          </div>
        )}

        <main className="main-content">
          {/* Left Panel: Search, Filters, Results, Detail Panels */}
          <aside className="sidebar">
            {/* Search Bar — SPEC §7 */}
            <div className="sidebar-section">
              <SearchBar
                onSearch={searchParcels}
                loading={searchLoading}
                error={searchError ?? searchErrorMcp?.error.message ?? null}
              />
            </div>

            {/* Search Results — SPEC §9 */}
            {searchResults.length > 0 && (
              <div className="sidebar-section">
                <SearchResults
                  results={searchResults}
                  totalCount={searchTotalCount}
                  onSelect={handleSelectResult}
                  loading={searchLoading}
                />
              </div>
            )}

            {/* Search loading indicator */}
            {searchLoading && searchResults.length === 0 && (
              <div className="sidebar-section">
                <div className="search-loading-msg">搜尋中…</div>
              </div>
            )}

            {/* Panel navigation tabs */}
            {selectedParcel && (
              <div className="panel-tabs">
                <button
                  className={`panel-tab ${activePanel === 'parcel' ? 'active' : ''}`}
                  onClick={() => setActivePanel('parcel')}
                >
                  Parcel
                </button>
                <button
                  className={`panel-tab ${activePanel === 'transactions' ? 'active' : ''}`}
                  onClick={() => setActivePanel('transactions')}
                >
                  Transactions
                </button>
                <button
                  className={`panel-tab ${activePanel === 'comparables' ? 'active' : ''}`}
                  onClick={() => setActivePanel('comparables')}
                >
                  Comparables
                </button>
                <button
                  className={`panel-tab ${activePanel === 'road' ? 'active' : ''}`}
                  onClick={() => setActivePanel('road')}
                >
                  Road / GIS
                </button>
                <button
                  className={`panel-tab ${activePanel === 'valuation' ? 'active' : ''}`}
                  onClick={() => setActivePanel('valuation')}
                >
                  Valuation
                </button>
                <button
                  className={`panel-tab ${activePanel === 'provenance' ? 'active' : ''}`}
                  onClick={() => setActivePanel('provenance')}
                >
                  Provenance
                </button>
              </div>
            )}

            {/* Parcel Inspector — SPEC §15-16 */}
            {activePanel === 'parcel' && (
              <div className="sidebar-section">
                <ParcelInspector
                  parcel={selectedParcel}
                  parcelInfo={parcelInfo}
                  parcelLoading={parcelLoading}
                  parcelError={parcelError ?? parcelErrorMcp?.error.message ?? null}
                  transactions={transactions}
                  selectedTransaction={selectedTransaction}
                  onSelectTransaction={(tx) => {
                    selectTransaction(tx);
                    setActivePanel('transactions');
                  }}
                  comparables={comparables}
                  selectedComparable={selectedComparable}
                  onSelectComparable={(comp) => {
                    selectComparable(comp);
                    setActivePanel('comparables');
                  }}
                  roadAccess={roadAccess}
                  valuation={valuation}
                  provenance={provenance.parcel}
                  onViewTransactions={() => setActivePanel('transactions')}
                  onViewComparables={() => setActivePanel('comparables')}
                  onViewRoad={() => setActivePanel('road')}
                  onViewValuation={() => setActivePanel('valuation')}
                  onViewProvenance={() => setActivePanel('provenance')}
                />
              </div>
            )}

            {/* Transaction Panel — SPEC §18-21 */}
            {activePanel === 'transactions' && (
              <div className="sidebar-section">
                <FilterPanel
                  filters={filters}
                  onFiltersChange={updateFilters}
                  onApply={applyFilters}
                  onReset={resetFilters}
                  visible={true}
                />
                <TransactionPanel
                  transactions={transactions}
                  statistics={transactionStatistics}
                  loading={transactionsLoading}
                  error={transactionsError}
                  selectedTransaction={selectedTransaction}
                  onSelect={(tx) => {
                    selectTransaction(tx);
                    loadTransaction(tx.transaction_id);
                  }}
                />
              </div>
            )}

            {/* Comparable Panel — SPEC §22-23 */}
            {activePanel === 'comparables' && (
              <div className="sidebar-section">
                <ComparablePanel
                  comparables={comparables}
                  loading={comparablesLoading}
                  error={comparablesError ?? null}
                  selectedComparable={selectedComparable}
                  onSelect={selectComparable}
                  onComparableTransactionClick={(tx) => {
                    selectTransaction(tx);
                    setActivePanel('transactions');
                  }}
                />
              </div>
            )}

            {/* Road Panel — SPEC §31-34 */}
            {activePanel === 'road' && (
              <div className="sidebar-section">
                <RoadPanel
                  roadAccess={roadAccess}
                  nearbyRoads={nearbyRoads}
                  loading={roadsLoading}
                  error={null}
                  parcelLocation={selectedParcel?.centroid ?? null}
                  onOpenStreetView={() => updateLayers({ showStreetView: true })}
                />
              </div>
            )}

            {/* Valuation Panel — SPEC §26-30 */}
            {activePanel === 'valuation' && (
              <div className="sidebar-section">
                <ValuationPanel
                  valuation={valuation}
                  comparables={comparables}
                  metadata={
                    provenance.parcel
                      ? {
                          algorithm_version: '',
                          snapshot_id: provenance.parcel.snapshot_id ?? '',
                          generated_at: '',
                          query_hash: provenance.parcel.query_hash ?? '',
                          request_id: '',
                        }
                      : undefined
                  }
                  loading={valuationLoading}
                  error={valuationError}
                  onExplain={() => {}}
                  onViewProvenance={() => setActivePanel('provenance')}
                />
              </div>
            )}

            {/* Provenance Panel — SPEC §39-40 */}
            {activePanel === 'provenance' && (
              <div className="sidebar-section">
                <ProvenancePanel
                  provenance={provenance}
                  metadata={{
                    algorithm_version: '',
                    snapshot_id: provenance.parcel?.snapshot_id ?? '',
                    generated_at: '',
                    query_hash: provenance.parcel?.query_hash ?? '',
                    request_id: '',
                  }}
                  valuation={
                    valuation
                      ? {
                          query_hash: valuation.query_hash,
                          algorithm_version: valuation.algorithm_version,
                          configuration_version: valuation.configuration_version,
                        }
                      : undefined
                  }
                />
              </div>
            )}
          </aside>

          {/* Map Workspace */}
          <div className="map-wrapper">
            <MapView
              parcel={selectedParcel ?? undefined}
              transactions={transactions}
              roads={
                roadAccess
                  ? [
                      {
                        road_id: '',
                        name: '',
                        width_source: roadAccess.source ?? '',
                        geometry: { type: 'MultiLineString', coordinates: [] },
                        distance_m: roadAccess.distance_m,
                        access_type: roadAccess.status as
                          | 'ROAD_ADJACENT'
                          | 'ROAD_NEARBY'
                          | 'NO_ROAD_DETECTED'
                          | 'UNKNOWN',
                      },
                    ]
                  : []
              }
              comparables={comparables}
              showSatellite={layers.showSatellite}
              showStreetView={layers.showStreetView}
              showNLSC={layers.showNLSC}
              showRoads={layers.showRoads}
              showComparables={layers.showComparables}
              showTransactions={layers.showTransactions}
              mapContext={mapContext ?? undefined}
            />

            {/* Layer controls overlay */}
            <div className="layer-controls-overlay">
              <div className="layer-control-group">
                <label>
                  <input
                    type="checkbox"
                    checked={layers.showSatellite}
                    onChange={(e) => updateLayers({ showSatellite: e.target.checked })}
                  />
                  Satellite
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={layers.showNLSC}
                    onChange={(e) => updateLayers({ showNLSC: e.target.checked })}
                  />
                  NLSC
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={layers.showRoads}
                    onChange={(e) => updateLayers({ showRoads: e.target.checked })}
                  />
                  Roads
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={layers.showComparables}
                    onChange={(e) => updateLayers({ showComparables: e.target.checked })}
                  />
                  Comparables
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={layers.showTransactions}
                    onChange={(e) => updateLayers({ showTransactions: e.target.checked })}
                  />
                  Transactions
                </label>
                {MAP_PROVIDER === 'google' && (
                  <label>
                    <input
                      type="checkbox"
                      checked={layers.showStreetView}
                      onChange={(e) => updateLayers({ showStreetView: e.target.checked })}
                    />
                    Street View
                  </label>
                )}
              </div>
            </div>
          </div>
        </main>
      </div>
    </ErrorBoundary>
  );
};

export default App;
