/**
 * FilterPanel — transaction/comparable filtering UI.
 *
 * Per SPEC §8: supports county, section, date range, land area, zoning,
 * land use, price range, unit price.
 * Per SPEC §8.1: area displayed in ping (坪), API uses m².
 *   Conversion: 1 坡 = 3.305785 m²
 */

import React from 'react';
import type { FilterState } from '../hooks/useAppState';
import './FilterPanel.css';

interface FilterPanelProps {
  filters: FilterState;
  onFiltersChange: (filters: Partial<FilterState>) => void;
  onApply: () => void;
  onReset: () => void;
  visible: boolean;
}

export const FilterPanel: React.FC<FilterPanelProps> = ({
  filters,
  onFiltersChange,
  onApply,
  onReset,
  visible,
}) => {
  if (!visible) return null;

  const handleDateRange = (range: '1y' | '3y' | '5y' | 'custom') => {
    onFiltersChange({ dateRange: range });
    if (range !== 'custom') {
      onFiltersChange({ dateFrom: undefined, dateTo: undefined });
    }
  };

  const handleAreaChange = (field: 'areaMinPing' | 'areaMaxPing', value: string) => {
    const num = value ? parseFloat(value) : undefined;
    onFiltersChange({ [field]: num });
  };

  return (
    <div className="filter-panel">
      <h3 className="filter-panel-title">篩選條件</h3>

      {/* Date Range */}
      <div className="filter-group">
        <label className="filter-label">交易期間</label>
        <div className="filter-pills">
          {(['1y', '3y', '5y', 'custom'] as const).map((range) => (
            <button
              key={range}
              type="button"
              className={`filter-pill ${filters.dateRange === range ? 'active' : ''}`}
              onClick={() => handleDateRange(range)}
            >
              {range === '1y' ? '近1年' : range === '3y' ? '近3年' : range === '5y' ? '近5年' : '自訂'}
            </button>
          ))}
        </div>
        {filters.dateRange === 'custom' && (
          <div className="filter-date-custom">
            <input
              type="date"
              value={filters.dateFrom ?? ''}
              onChange={(e) => onFiltersChange({ dateFrom: e.target.value || undefined })}
              className="filter-input-small"
            />
            <span>至</span>
            <input
              type="date"
              value={filters.dateTo ?? ''}
              onChange={(e) => onFiltersChange({ dateTo: e.target.value || undefined })}
              className="filter-input-small"
            />
          </div>
        )}
      </div>

      {/* Area Filter (in ping) */}
      <div className="filter-group">
        <label className="filter-label">土地面積 (坪)</label>
        <div className="filter-range-inputs">
          <input
            type="number"
            placeholder="最小值"
            value={filters.areaMinPing ?? ''}
            onChange={(e) => handleAreaChange('areaMinPing', e.target.value)}
            className="filter-input-small"
          />
          <span>～</span>
          <input
            type="number"
            placeholder="最大值"
            value={filters.areaMaxPing ?? ''}
            onChange={(e) => handleAreaChange('areaMaxPing', e.target.value)}
            className="filter-input-small"
          />
        </div>
      </div>

      {/* Price Range */}
      <div className="filter-group">
        <label className="filter-label">交易總價 (萬)</label>
        <div className="filter-range-inputs">
          <input
            type="number"
            placeholder="最小值"
            value={filters.minPrice ? filters.minPrice / 10000 : ''}
            onChange={(e) => {
              const v = e.target.value ? parseFloat(e.target.value) * 10000 : undefined;
              onFiltersChange({ minPrice: v });
            }}
            className="filter-input-small"
          />
          <span>～</span>
          <input
            type="number"
            placeholder="最大值"
            value={filters.maxPrice ? filters.maxPrice / 10000 : ''}
            onChange={(e) => {
              const v = e.target.value ? parseFloat(e.target.value) * 10000 : undefined;
              onFiltersChange({ maxPrice: v });
            }}
            className="filter-input-small"
          />
        </div>
      </div>

      {/* Unit Price */}
      <div className="filter-group">
        <label className="filter-label">單價 (萬/坪)</label>
        <div className="filter-range-inputs">
          <input
            type="number"
            placeholder="最小值"
            value={filters.minUnitPrice ? filters.minUnitPrice / 10000 : ''}
            onChange={(e) => {
              const v = e.target.value ? parseFloat(e.target.value) * 10000 : undefined;
              onFiltersChange({ minUnitPrice: v });
            }}
            className="filter-input-small"
          />
          <span>～</span>
          <input
            type="number"
            placeholder="最大值"
            value={filters.maxUnitPrice ? filters.maxUnitPrice / 10000 : ''}
            onChange={(e) => {
              const v = e.target.value ? parseFloat(e.target.value) * 10000 : undefined;
              onFiltersChange({ maxUnitPrice: v });
            }}
            className="filter-input-small"
          />
        </div>
      </div>

      {/* Zoning */}
      <div className="filter-group">
        <label className="filter-label">使用分區</label>
        <input
          type="text"
          placeholder="如: 住五，商五"
          value={filters.urbanZoning ?? ''}
          onChange={(e) => onFiltersChange({ urbanZoning: e.target.value || undefined })}
          className="filter-input"
        />
      </div>

      {/* Land Use */}
      <div className="filter-group">
        <label className="filter-label">使用地類別</label>
        <input
          type="text"
          placeholder="如: 住宅地"
          value={filters.landUseCategory ?? ''}
          onChange={(e) => onFiltersChange({ landUseCategory: e.target.value || undefined })}
          className="filter-input"
        />
      </div>

      {/* Action buttons */}
      <div className="filter-actions">
        <button type="button" className="filter-apply" onClick={onApply}>
          套用篩選
        </button>
        <button type="button" className="filter-reset" onClick={onReset}>
          重置
        </button>
      </div>
    </div>
  );
};
