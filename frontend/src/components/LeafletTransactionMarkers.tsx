import React from 'react';
import L from 'leaflet';
import type { Transaction } from '../types';

interface LeafletTransactionMarkersProps {
  map: L.Map | null;
  transactions: Transaction[];
}

const transactionIcon = L.divIcon({
  html: '<div class="transaction-marker" style="background:#4299e1;color:white;border-radius:50%;width:20px;height:20px;display:flex;align-items:center;justify-content:center;font-size:10px;"></div>',
  className: 'transaction-icon',
  iconSize: [20, 20],
  iconAnchor: [10, 10],
});

export const LeafletTransactionMarkers: React.FC<LeafletTransactionMarkersProps> = ({ map, transactions }) => {
  React.useEffect(() => {
    if (!transactions.length || !map) return;

    const markers: L.Marker[] = [];

    transactions.forEach((tx) => {
      const pos = tx.location;

      if (pos) {
        const marker = L.marker([pos.lat, pos.lng], { icon: transactionIcon }).addTo(map);
        const popupContent = `
          <div style="min-width:150px;font-size:12px">
            <b>${tx.county} ${tx.district}</b><br/>
            ${tx.section ?? ''} ${tx.land_number ?? ''}<br/>
            $${tx.total_price.toLocaleString()} (${new Date(tx.transaction_date).getFullYear()})
          </div>
        `;
        marker.bindPopup(popupContent);
        markers.push(marker);
      }
    });

    return () => {
      markers.forEach((m) => map.removeLayer(m));
    };
  }, [map, transactions]);

  return null;
};
