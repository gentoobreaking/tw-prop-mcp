/**
 * ProvenancePanel — data provenance chain display.
 *
 * Per SPEC §40: expandable "Data Provenance" drawer.
 * Per SPEC §39: every major data category must expose provenance.
 * Per SPEC §23: Transaction → Snapshot → Official Source chain.
 */

import React, { useState } from 'react';
import type { ProvenanceChain, ResponseMetadata } from '../types';
import './ProvenancePanel.css';

interface ProvenancePanelProps {
  provenance: Record<string, ProvenanceChain>;
  metadata?: ResponseMetadata;
  valuation?: {
    query_hash?: string;
    algorithm_version?: string;
    configuration_version?: string;
  };
  expanded?: boolean;
}

export const ProvenancePanel: React.FC<ProvenancePanelProps> = ({
  provenance,
  metadata,
  valuation,
  expanded = false,
}) => {
  const [isExpanded, setIsExpanded] = useState(expanded);

  const toggleExpand = () => setIsExpanded((prev) => !prev);

  return (
    <div className="provenance-panel">
      <button className="provenance-toggle" onClick={toggleExpand}>
        <span className="provenance-title">資料溯源</span>
        <span className={`provenance-chevron ${isExpanded ? 'expanded' : ''}`}>
          ▼
        </span>
      </button>

      {isExpanded && (
        <div className="provenance-content">
          {/* Valuation provenance — SPEC §28 */}
          {valuation && (
            <div className="provenance-section">
              <h4 className="provenance-section-title">估價</h4>
              <div className="provenance-grid">
                {valuation.algorithm_version && (
                  <ProvenanceRow label="Algorithm Version" value={valuation.algorithm_version} />
                )}
                {valuation.configuration_version && (
                  <ProvenanceRow label="Config Version" value={valuation.configuration_version} />
                )}
                {valuation.query_hash && (
                  <ProvenanceRow
                    label="Query Hash"
                    value={
                      <code className="provenance-hash">{valuation.query_hash}</code>
                    }
                  />
                )}
              </div>
            </div>
          )}

          {/* General metadata — SPEC §3.6, §3.7 */}
          {metadata && (
            <div className="provenance-section">
              <h4 className="provenance-section-title">資料快照</h4>
              <div className="provenance-grid">
                {metadata.snapshot_id && (
                  <ProvenanceRow label="Snapshot ID" value={metadata.snapshot_id} />
                )}
                {metadata.algorithm_version && (
                  <ProvenanceRow label="Algorithm" value={metadata.algorithm_version} />
                )}
                {metadata.generated_at && (
                  <ProvenanceRow label="Generated" value={metadata.generated_at} />
                )}
                {metadata.query_hash && (
                  <ProvenanceRow
                    label="Query Hash"
                    value={<code className="provenance-hash">{metadata.query_hash.slice(0, 32)}…</code>}
                  />
                )}
                {metadata.request_id && (
                  <ProvenanceRow label="Request ID" value={metadata.request_id} />
                )}
              </div>
            </div>
          )}

          {/* Provenance chains from MCP tools */}
          {provenance && Object.entries(provenance).map(([key, chain]) => {
            if (!chain || !chain.chain || chain.chain.length === 0) return null;
            return (
              <div key={key} className="provenance-section">
                <h4 className="provenance-section-title">
                  {key === 'parcel' ? '地號' :
                   key === 'transaction' ? '交易' :
                   key === 'comparable' ? '可比交易' :
                   key === 'gis' ? 'GIS' :
                   key === 'road' ? '道路' :
                   key === 'valuation' ? '估價' : key}
                </h4>
                <div className="provenance-chain">
                  {chain.chain.map((info, index) => (
                    <div key={index} className="provenance-chain-item">
                      <div className="chain-arrow">
                        {index > 0 && <span className="arrow">→</span>}
                      </div>
                      <div className="provenance-info">
                        {info.source && (
                          <ProvenanceRow label="Source" value={info.source} />
                        )}
                        {info.source_version && (
                          <ProvenanceRow label="Version" value={info.source_version} />
                        )}
                        {info.snapshot_id && (
                          <ProvenanceRow label="Snapshot" value={info.snapshot_id} />
                        )}
                        {info.downloaded_at && (
                          <ProvenanceRow label="Downloaded" value={info.downloaded_at} />
                        )}
                        {info.file_sha256 && (
                          <ProvenanceRow label="File SHA256" value={info.file_sha256} />
                        )}
                        {info.record_count !== undefined && (
                          <ProvenanceRow label="Record Count" value={String(info.record_count)} />
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

const ProvenanceRow: React.FC<{
  label: string;
  value: string | React.ReactNode;
}> = ({ label, value }) => (
  <div className="provenance-row">
    <span className="provenance-label">{label}</span>
    <span className="provenance-value">{value}</span>
  </div>
);
