import React from 'react';
import type { Transaction } from '../types';

interface TransactionMarkersProps {
  google: typeof google;
  map: google.maps.Map;
  transactions: Transaction[];
}

/**
 * Renders transaction locations as markers on the map.
 * Clustered for performance with many transactions.
 */
export const TransactionMarkers: React.FC<TransactionMarkersProps> = ({
  google,
  map,
  transactions,
}) => {
  React.useEffect(() => {
    if (!transactions.length || !google || !map) return;

    const markers = transactions
      .filter((t) => t.location)
      .map((t) => {
        const marker = new google.maps.Marker({
          map,
          position: { lat: t.location!.lat, lng: t.location!.lng },
          title: `${t.district} ${t.section || ''} ${t.land_number || ''}`,
        });

        const info = new google.maps.InfoWindow({
          content: `
            <div style="padding: 8px;">
              <strong>${t.transaction_id}</strong><br/>
              ${t.county} ${t.district} ${t.section || ''} ${t.land_number || ''}<br/>
              價格: NT$ ${t.total_price.toLocaleString()}<br/>
              面積: ${t.land_area_sqm || 0} ㎡
            </div>
          `,
        });

        marker.addListener('click', () => {
          info.open(map, marker);
        });

        return { marker, info };
      });

    return () => {
      markers.forEach(({ marker, info }) => {
        marker.setMap(null);
        info.close();
      });
    };
  }, [google, map, transactions]);

  return null;
};
