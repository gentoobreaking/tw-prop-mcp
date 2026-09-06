/**
 * TransactionMarkers — renders Google Maps markers for real-estate
 * transactions returned by the MCP `search_transactions` tool.
 *
 * Uses google.maps.marker.AdvancedMarkerElement (modern marker library).
 * Clustering is delegated to the MapView layer via a MarkerClusterer when
 * available; individual markers are placed here from MCP transaction data
 * only. The frontend does not compute clustering or geometry.
 *
 * SPEC §4.8: Google Maps is visualization only.
 */

import { useEffect, useRef } from 'react';
import { transactionToMarker } from '../services/mapService';
import type { Transaction } from '../types';

export interface TransactionMarkersProps {
  map: google.maps.Map | null;
  google: typeof google | null;
  transactions: Transaction[];
  visible: boolean;
  /** Optional clusterer to attach markers to (created by MapView). */
  clusterer?: unknown;
  onMarkerClick?: (transaction: Transaction) => void;
}

export function TransactionMarkers({
  map,
  google,
  transactions,
  visible,
  onMarkerClick,
}: TransactionMarkersProps) {
  const markersRef = useRef<Map<string, google.maps.marker.AdvancedMarkerElement>>(new Map());

  const clearMarkers = () => {
    markersRef.current.forEach((marker) => {
      marker.map = null;
    });
    markersRef.current.clear();
  };

  useEffect(() => {
    if (!map || !google || !visible) {
      clearMarkers();
      return;
    }

    const valid = transactions.filter((t) => t.latitude != null && t.longitude != null);

    for (const tx of valid) {
      const position = transactionToMarker(tx);
      if (!position) continue;

      const el = document.createElement('div');
      el.className = 'map-marker transaction-marker';
      const dot = document.createElement('div');
      dot.className = 'map-marker__dot';
      dot.style.backgroundColor = '#ff5722';
      el.appendChild(dot);

      const marker = new google.maps.marker.AdvancedMarkerElement({
        position: { lat: position.lat, lng: position.lng },
        content: el,
        map,
        title: `${tx.county}${tx.district}${tx.section} ${tx.land_number}`,
      });

      const info = new google.maps.InfoWindow({
        content: buildTransactionInfo(tx),
      });

      marker.addListener('click', () => {
        info.open(map, marker);
        onMarkerClick?.(tx);
      });

      markersRef.current.set(tx.transaction_id, marker);
    }

    return () => clearMarkers();
  }, [map, google, transactions, visible, onMarkerClick]);

  useEffect(() => clearMarkers, []);

  return null;
}

function buildTransactionInfo(tx: Transaction): string {
  const price = new Intl.NumberFormat('zh-TW', {
    style: 'currency',
    currency: 'TWD',
  }).format(tx.total_price);

  const ppp = tx.price_per_ping
    ? `${Math.round(tx.price_per_ping).toLocaleString('zh-TW')} 元/坪`
    : '';

  return `
    <div class="info-window">
      <strong>${tx.county}${tx.district}${tx.section} ${tx.land_number}</strong>
      <div>成交日: ${tx.transaction_date}</div>
      <div>總價: ${price}</div>
      ${ppp ? `<div>單價: ${ppp}</div>` : ''}
      ${tx.address ? `<div>地址: ${tx.address}</div>` : ''}
    </div>
  `;
}
