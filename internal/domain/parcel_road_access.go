package domain

import (
	"fmt"
	"time"
)

const (
	// AccessTypeRoadAdjacent means the parcel boundary directly touches the road
	AccessTypeRoadAdjacent = "ROAD_ADJACENT"
	// AccessTypeRoadNearby means the road is nearby but not directly adjacent
	AccessTypeRoadNearby = "ROAD_NEARBY"
	// AccessTypeNoRoadDetected means no road was detected within search radius
	AccessTypeNoRoadDetected = "NO_ROAD_DETECTED"
	// AccessTypeUnknown means access type could not be determined
	AccessTypeUnknown = "UNKNOWN"
)

// ParcelRoadAccess represents the road access relationship for a parcel
type ParcelRoadAccess struct {
	ID             string    `json:"id"`
	ParcelID       string    `json:"parcel_id"`
	RoadID         string    `json:"road_id,omitempty"`
	DistanceM      float64   `json:"distance_m"`
	NearestPoint   string    `json:"nearest_point,omitempty"` // WKT POINT in EPSG:3826
	RoadWidthM     *float64  `json:"road_width_m,omitempty"`
	AccessType     string    `json:"access_type"` // ROAD_ADJACENT, ROAD_NEARBY, NO_ROAD_DETECTED, UNKNOWN
	Source         string    `json:"source"`
	AlgorithmVersion string  `json:"algorithm_version"`
	ComputedAt     time.Time `json:"computed_at"`
}

// ParcelRoadAccessCreateParams for creating a new parcel road access record
type ParcelRoadAccessCreateParams struct {
	ParcelID        string
	RoadID          string
	DistanceM       float64
	NearestPoint    string
	RoadWidthM      *float64
	AccessType      string
	Source          string
	AlgorithmVersion string
}

// ParcelRoadAccessFilter for filtering parcel road access records
type ParcelRoadAccessFilter struct {
	ParcelID    *string
	RoadID      *string
	AccessType  *string
	Source      *string
	AlgorithmVersion *string
	Limit       int32
	Offset      int32
}

// ValidateAccessType validates if the access type is valid
func ValidateAccessType(accessType string) error {
	switch accessType {
	case AccessTypeRoadAdjacent, AccessTypeRoadNearby, AccessTypeNoRoadDetected, AccessTypeUnknown:
		return nil
	default:
		return fmt.Errorf("invalid access type: %s", accessType)
	}
}