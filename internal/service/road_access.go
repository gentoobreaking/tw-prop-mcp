package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// RoadAccessEngine provides road access determination logic
type RoadAccessEngine struct {
	roadRepo            repository.RoadSegmentRepository
	parcelRoadAccessRepo repository.ParcelRoadAccessRepository
	parcelRepo          repository.ParcelRepository
}

// NewRoadAccessEngine creates a new RoadAccessEngine
func NewRoadAccessEngine(
	roadRepo repository.RoadSegmentRepository,
	parcelRoadAccessRepo repository.ParcelRoadAccessRepository,
	parcelRepo repository.ParcelRepository,
) *RoadAccessEngine {
	return &RoadAccessEngine{
		roadRepo:             roadRepo,
		parcelRoadAccessRepo: parcelRoadAccessRepo,
		parcelRepo:           parcelRepo,
	}
}

// RoadAccessConfig holds configuration for road access determination
type RoadAccessConfig struct {
	// AdjacentTolerance is the distance threshold (in meters) for considering a parcel adjacent to a road
	AdjacentTolerance float64
	// NearbyThreshold is the distance threshold (in meters) for considering a road "nearby"
	NearbyThreshold float64
	// SearchRadius is the maximum search radius for finding nearby roads
	SearchRadius float64
}

// DefaultRoadAccessConfig returns the default configuration
func DefaultRoadAccessConfig() RoadAccessConfig {
	return RoadAccessConfig{
		AdjacentTolerance: 2.0,  // 2 meters tolerance for adjacency
		NearbyThreshold:   50.0, // 50 meters for nearby
		SearchRadius:      100.0, // 100 meters search radius
	}
}

// DetermineRoadAccess determines the road access for a given parcel
func (e *RoadAccessEngine) DetermineRoadAccess(ctx context.Context, parcelID string, config RoadAccessConfig) (*domain.ParcelRoadAccess, error) {
	// Get parcel
	parcel, err := e.parcelRepo.GetByID(ctx, parcelID)
	if err != nil {
		return nil, fmt.Errorf("get parcel: %w", err)
	}

	// Find nearby roads
	nearbyRoads, err := e.findNearbyRoads(ctx, *parcel, config.SearchRadius)
	if err != nil {
		return nil, fmt.Errorf("find nearby roads: %w", err)
	}

	if len(nearbyRoads) == 0 {
		return e.createAccessRecord(ctx, parcelID, "", 0, "", nil, domain.AccessTypeNoRoadDetected, "GIS", "v1.0")
	}

	// Find the closest road and determine access type
	bestRoad, distance, nearestPoint, accessType := e.findBestRoad(ctx, *parcel, nearbyRoads, config)

	if bestRoad == nil {
		return e.createAccessRecord(ctx, parcelID, "", 0, "", nil, domain.AccessTypeUnknown, "GIS", "v1.0")
	}

	// Get road width
	roadWidth := bestRoad.WidthM

	return e.createAccessRecord(ctx, parcelID, bestRoad.ID, distance, nearestPoint, roadWidth, accessType, "GIS", "v1.0")
}

// findNearbyRoads finds roads within the search radius of a parcel
func (e *RoadAccessEngine) findNearbyRoads(ctx context.Context, parcel domain.Parcel, radius float64) ([]domain.RoadSegment, error) {
	// Use ST_DWithin to find roads within radius
	filter := domain.RoadSegmentFilter{
		BBox:  &parcel.BBox,
		Limit: 50,
	}
	return e.roadRepo.Search(ctx, filter)
}

// findBestRoad finds the best matching road for a parcel
func (e *RoadAccessEngine) findBestRoad(ctx context.Context, parcel domain.Parcel, roads []domain.RoadSegment, config RoadAccessConfig) (*domain.RoadSegment, float64, string, string) {
	var bestRoad *domain.RoadSegment
	var bestDistance float64 = math.MaxFloat64
	var bestNearestPoint string
	var accessType string

	for _, road := range roads {
		distance, nearestPoint, err := e.calculateDistance(ctx, parcel.Geometry, road.Geometry)
		if err != nil {
			continue
		}

		if distance < bestDistance {
			bestDistance = distance
			bestRoad = &road
			bestNearestPoint = nearestPoint

			// Determine access type based on distance
			if distance <= config.AdjacentTolerance {
				accessType = domain.AccessTypeRoadAdjacent
			} else if distance <= config.NearbyThreshold {
				accessType = domain.AccessTypeRoadNearby
			} else {
				accessType = domain.AccessTypeRoadNearby
			}
		}
	}

	return bestRoad, bestDistance, bestNearestPoint, accessType
}

// calculateDistance calculates the distance between parcel and road geometries
func (e *RoadAccessEngine) calculateDistance(ctx context.Context, parcelGeom, roadGeom string) (float64, string, error) {
	// This would use PostGIS ST_Distance and ST_ClosestPoint
	// For now, return a placeholder
	return 0, "", nil
}

// createAccessRecord creates a parcel road access record
func (e *RoadAccessEngine) createAccessRecord(ctx context.Context, parcelID, roadID string, distance float64, nearestPoint string, roadWidth *float64, accessType, source, algoVersion string) (*domain.ParcelRoadAccess, error) {
	record, err := e.parcelRoadAccessRepo.Create(ctx, domain.ParcelRoadAccessCreateParams{
		ParcelID:        parcelID,
		RoadID:          roadID,
		DistanceM:       distance,
		NearestPoint:    nearestPoint,
		RoadWidthM:      roadWidth,
		AccessType:      accessType,
		Source:          source,
		AlgorithmVersion: algoVersion,
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// DetermineBatchRoadAccess determines road access for multiple parcels
func (e *RoadAccessEngine) DetermineBatchRoadAccess(ctx context.Context, parcelIDs []string, config RoadAccessConfig) ([]domain.ParcelRoadAccess, error) {
	results := make([]domain.ParcelRoadAccess, 0, len(parcelIDs))
	for _, parcelID := range parcelIDs {
		access, err := e.DetermineRoadAccess(ctx, parcelID, config)
		if err != nil {
			// Log error but continue
			continue
		}
		results = append(results, *access)
	}
	return results, nil
}

// ValidateRoadWidthSource validates that road width source is valid
func ValidateRoadWidthSource(source string) error {
	switch source {
	case domain.WidthSourceOfficial, domain.WidthSourceGISDerived, domain.WidthSourceUnknown:
		return nil
	default:
		return fmt.Errorf("invalid width source: %s", source)
	}
}

// RoadAccessResult represents the result of a road access determination
type RoadAccessResult struct {
	ParcelID         string
	RoadID           string
	DistanceM        float64
	NearestPoint     string
	RoadWidthM       *float64
	AccessType       string
	AlgorithmVersion string
	ComputedAt       time.Time
	Error            string
}

// DetermineRoadAccessForParcel determines road access for a single parcel (wrapper for external use)
func (e *RoadAccessEngine) DetermineRoadAccessForParcel(ctx context.Context, parcelID string) (*RoadAccessResult, error) {
	access, err := e.DetermineRoadAccess(ctx, parcelID, DefaultRoadAccessConfig())
	if err != nil {
		return &RoadAccessResult{
			ParcelID: parcelID,
			Error:    err.Error(),
		}, nil
	}

	return &RoadAccessResult{
		ParcelID:         access.ParcelID,
		RoadID:           access.RoadID,
		DistanceM:        access.DistanceM,
		NearestPoint:     access.NearestPoint,
		RoadWidthM:       access.RoadWidthM,
		AccessType:       access.AccessType,
		AlgorithmVersion: access.AlgorithmVersion,
		ComputedAt:       access.ComputedAt,
	}, nil
}

// BatchDetermineRoadAccess processes multiple parcels
func (e *RoadAccessEngine) BatchDetermineRoadAccess(ctx context.Context, parcelIDs []string) ([]RoadAccessResult, error) {
	results := make([]RoadAccessResult, 0, len(parcelIDs))
	for _, parcelID := range parcelIDs {
		result, err := e.DetermineRoadAccessForParcel(ctx, parcelID)
		if err != nil {
			results = append(results, RoadAccessResult{
				ParcelID: parcelID,
				Error:    err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}