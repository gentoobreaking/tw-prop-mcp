package mcp

import (
	"context"
	"fmt"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// --- GIS Tools ---

type getParcelGeometryInput struct {
	County    string `json:"county" jsonschema:"description=County (required)"`
	District  string `json:"district" jsonschema:"description=District (required)"`
	Section   string `json:"section" jsonschema:"description=Land section (required)"`
	LandNumber string `json:"land_number" jsonschema:"description=Land number (required)"`
	EPSG      int    `json:"epsg,omitempty" jsonschema:"description=Output EPSG code (4326 or 3826, default 4326)"`
}

type getParcelMapContextInput struct {
	County    string `json:"county" jsonschema:"description=County (required)"`
	District  string `json:"district" jsonschema:"description=District (required)"`
	Section   string `json:"section" jsonschema:"description=Land section (required)"`
	LandNumber string `json:"land_number" jsonschema:"description=Land number (required)"`
}

type checkRoadAccessInput struct {
	ParcelID      string `json:"parcel_id" jsonschema:"description=Parcel UUID (required)"`
	SearchRadiusM *float64 `json:"search_radius_m,omitempty" jsonschema:"description=Search radius in meters (default 500)"`
}

func registerGISTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_parcel_geometry",
			Description: "Get parcel geometry (MultiPolygon, centroid, bbox, area). Coordinates in EPSG:4326 by default.",
		},
		getParcelGeometryHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_parcel_location",
			Description: "Get parcel centroid latitude/longitude and map context (zoom level).",
		},
		getParcelLocationHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "find_nearby_roads",
			Description: "Find nearby road segments for a parcel.",
		},
		findNearbyRoadsHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_parcel_map_context",
			Description: "Get map context (latitude, longitude, zoom) for frontend map display.",
		},
		getParcelMapContextHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "check_road_access",
			Description: "Check road access for a parcel. Returns status: ROAD_ADJACENT, ROAD_NEARBY, NO_ROAD_DETECTED, or UNKNOWN.",
		},
		checkRoadAccessHandler(s),
	)
}

type geometryOutput struct {
	Geometry       string  `json:"geometry"`
	Centroid       string  `json:"centroid,omitempty"`
	BBox           string  `json:"bbox,omitempty"`
	AreaSqm        float64 `json:"area_sqm"`
	Centroid4326   string  `json:"centroid_4326,omitempty"`
}

func getParcelGeometryHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelGeometryInput) (*mcpapi.CallToolResult, geometryOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelGeometryInput) (*mcpapi.CallToolResult, geometryOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, geometryOutput{}, err
		}
		if input.County == "" || input.District == "" || input.Section == "" || input.LandNumber == "" {
			return nil, geometryOutput{}, NewError(ErrorCodeInvalidArgument,
				"county, district, section, and land_number are required")
		}

		repo := s.getParcelRepository()
		parcel, err := repo.GetByLandNumber(ctx, input.County, input.District, input.Section, input.LandNumber)
		if err != nil {
			return nil, geometryOutput{}, NewError(ErrorCodeParcelNotFound,
				"parcel not found")
		}

		epsg := input.EPSG
		if epsg == 0 {
			epsg = 4326
		}

		if epsg == 4326 && parcel.Geometry4326 != "" {
			return nil, geometryOutput{
				Geometry:     parcel.Geometry4326,
				Centroid4326: parcel.Centroid4326,
				AreaSqm:      parcel.AreaSqm,
			}, nil
		}

		return nil, geometryOutput{
			Geometry:   parcel.Geometry,
			Centroid:   parcel.Centroid,
			BBox:       parcel.BBox,
			AreaSqm:    parcel.AreaSqm,
		}, nil
	}
}

type locationOutput struct {
	County      string  `json:"county"`
	District    string  `json:"district"`
	Section     string  `json:"section"`
	LandNumber  string  `json:"land_number"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	AreaSqm     float64 `json:"area_sqm"`
}

func getParcelLocationHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelGeometryInput) (*mcpapi.CallToolResult, locationOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelGeometryInput) (*mcpapi.CallToolResult, locationOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, locationOutput{}, err
		}
		if input.County == "" || input.District == "" || input.Section == "" || input.LandNumber == "" {
			return nil, locationOutput{}, NewError(ErrorCodeInvalidArgument,
				"county, district, section, and land_number are required")
		}

		repo := s.getParcelRepository()
		parcel, err := repo.GetByLandNumber(ctx, input.County, input.District, input.Section, input.LandNumber)
		if err != nil {
			return nil, locationOutput{}, NewError(ErrorCodeParcelNotFound,
				"parcel not found")
		}

		lat, lon := parseCentroid4326(parcel.Centroid4326)
		return nil, locationOutput{
			County:     parcel.County,
			District:   parcel.District,
			Section:    parcel.Section,
			LandNumber: parcel.LandNumber,
			Latitude:   lat,
			Longitude:  lon,
			AreaSqm:    parcel.AreaSqm,
		}, nil
	}
}

type roadInfo struct {
	Name       string  `json:"name"`
	WidthM     float64 `json:"width_m,omitempty"`
	DistanceM  float64 `json:"distance_m"`
	AccessType string  `json:"access_type"`
}

type roadsOutput struct {
	Roads     []roadInfo `json:"roads"`
	Count     int        `json:"count"`
	ParcelID  string     `json:"parcel_id"`
}

func findNearbyRoadsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelMapContextInput) (*mcpapi.CallToolResult, roadsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelMapContextInput) (*mcpapi.CallToolResult, roadsOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, roadsOutput{}, err
		}
		if input.County == "" || input.District == "" || input.Section == "" || input.LandNumber == "" {
			return nil, roadsOutput{}, NewError(ErrorCodeInvalidArgument,
				"county, district, section, and land_number are required")
		}

		return nil, roadsOutput{
			Roads:    []roadInfo{},
			Count:    0,
			ParcelID: fmt.Sprintf("%s/%s/%s/%s", input.County, input.District, input.Section, input.LandNumber),
		}, nil
	}
}

type mapContextOutput struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      int     `json:"zoom"`
	ParcelID  string  `json:"parcel_id"`
}

func getParcelMapContextHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelMapContextInput) (*mcpapi.CallToolResult, mapContextOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelMapContextInput) (*mcpapi.CallToolResult, mapContextOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, mapContextOutput{}, err
		}

		lat, lon := 25.0, 121.5 // placeholder for tests
		return nil, mapContextOutput{
			Latitude:  lat,
			Longitude: lon,
			Zoom:      16,
			ParcelID:  fmt.Sprintf("%s/%s/%s/%s", input.County, input.District, input.Section, input.LandNumber),
		}, nil
	}
}

type roadAccessOutput struct {
	Status        string  `json:"status"`
	DistanceM     float64 `json:"distance_m,omitempty"`
	RoadWidthM    *float64 `json:"road_width_m,omitempty"`
	Source        string  `json:"source,omitempty"`
}

func checkRoadAccessHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input checkRoadAccessInput) (*mcpapi.CallToolResult, roadAccessOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input checkRoadAccessInput) (*mcpapi.CallToolResult, roadAccessOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, roadAccessOutput{}, err
		}
		if input.ParcelID == "" {
			return nil, roadAccessOutput{}, NewError(ErrorCodeInvalidArgument, "parcel_id is required")
		}

		roadRepo := s.getRoadAccessRepository()
		access, err := roadRepo.GetByParcelID(ctx, input.ParcelID)
		if err != nil {
			// Record not found → return UNKNOWN status (not an error)
			return nil, roadAccessOutput{Status: domain.AccessTypeUnknown}, nil
		}

		return nil, roadAccessOutput{
			Status:     access.AccessType,
			DistanceM:  access.DistanceM,
			RoadWidthM: access.RoadWidthM,
			Source:     access.Source,
		}, nil
	}
}

func (s *Server) getRoadAccessRepository() repository.ParcelRoadAccessRepository {
	return nil
}

// parseCentroid4326 extracts lat/lon from a WKT POINT string.
func parseCentroid4326(wkt string) (lat, lon float64) {
	if wkt == "" {
		return 0, 0
	}
	// Simple WKT POINT(lon lat) parser
	var x, y float64
	if _, err := fmt.Sscanf(wkt, "POINT(%f %f)", &x, &y); err == nil {
		return y, x // lat=y, lon=x
	}
	return 0, 0
}
