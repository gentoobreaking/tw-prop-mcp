import React, { useState, useEffect } from 'react';
import MapView from './components/MapView';
import ValuationPanel from './components/ValuationPanel';
import ErrorBoundary from './components/ErrorBoundary';
import { useMCP } from './hooks/useMCP';
import './App.css';

const App: React.FC = () => {
  const { data, loading, error, clearError } = useMCP();
  const [showStreetView, setShowStreetView] = useState(false);
  const [showSatellite, setShowSatellite] = useState(false);
  const [showNLSC, setShowNLSC] = useState(false);

  useEffect(() => {
    document.title = data
      ? `tw-prop-mcp — ${data.metadata.snapshot_id || 'Property Map'}`
      : 'tw-prop-mcp — Taiwan Property Valuation';
  }, [data]);

  const transactions = data?.transactions ?? [];
  const roads = data?.roads ?? [];
  const comparables = data?.comparables ?? [];

  return (
    <ErrorBoundary>
      <div className="app-container">
        <header className="app-header">
          <h1 className="app-title">Taiwan Property Valuation</h1>
          {loading && <span className="loading-badge">Loading…</span>}
        </header>

        {error && (
          <div className="error-banner">
            <span>{error}</span>
            <button onClick={clearError} className="error-dismiss">
              ×
            </button>
          </div>
        )}

        <main className="main-content">
          <aside className="sidebar">
            <div className="layer-controls">
              <h2>Layers</h2>
              <label>
                <input
                  type="checkbox"
                  checked={showSatellite}
                  onChange={(e) => setShowSatellite(e.target.checked)}
                />
                Satellite
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={showNLSC}
                  onChange={(e) => setShowNLSC(e.target.checked)}
                />
                NLSC Cadastral
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={showStreetView}
                  onChange={(e) => setShowStreetView(e.target.checked)}
                />
                Street View
              </label>
            </div>

            {data && (
              <ValuationPanel
                valuation={data.valuation}
                comparables={data.comparables}
                metadata={data.metadata}
              />
            )}
          </aside>

          <div className="map-wrapper">
            <MapView
              parcel={data?.parcel}
              transactions={transactions}
              roads={roads}
              comparables={comparables}
              showSatellite={showSatellite}
              showStreetView={showStreetView}
              showNLSC={showNLSC}
              mapContext={data?.map_context}
            />
          </div>
        </main>
      </div>
    </ErrorBoundary>
  );
};

export default App;
