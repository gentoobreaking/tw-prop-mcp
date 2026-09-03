package domain

import (
	"time"
)

const (
	// WidthSourceOfficial means width comes from official cadastral data
	WidthSourceOfficial = "OFFICIAL"
	// WidthSourceGISDerived means width was derived from GIS data
	WidthSourceGISDerived = "GIS_DERIVED"
	// WidthSourceUnknown means width source is unknown
	WidthSourceUnknown = "UNKNOWN"
)

// RoadSegment represents a road segment in the domain
type RoadSegment struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	RoadClass       string    `json:"road_class"`
	WidthM          *float64  `json:"width_m,omitempty"`
	WidthSource     string    `json:"width_source"` // OFFICIAL, GIS_DERIVED, UNKNOWN
	Geometry        string    `json:"geometry"`     // WKT/EWKT in EPSG:3826 (MultiLineString)
	Source          string    `json:"source"`
	SourceVersion   string    `json:"source_version"`
	ImportBatchID   string    `json:"import_batch_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoadSegmentCreateParams for creating a new road segment
type RoadSegmentCreateParams struct {
	Name            string
	RoadClass       string
	WidthM          *float64
	WidthSource     string
	Geometry        string
	Source          string
	SourceVersion   string
	ImportBatchID   string
}

// RoadSegmentFilter for searching road segments
type RoadSegmentFilter struct {
	Name        *string
	RoadClass   *string
	WidthSource *string
	BBox        *string // WKT polygon for spatial filter
	Limit       int32
	Offset      int32
}