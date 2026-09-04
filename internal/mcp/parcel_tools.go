package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// --- Parcel Tools ---

type getParcelInput struct {
	County    string `json:"county" jsonschema:"County (required)"`
	District  string `json:"district" jsonschema:"District (required)"`
	Section   string `json:"section" jsonschema:"Land section (required)"`
	LandNumber string `json:"land_number" jsonschema:"Land number (required)"`
}

type searchParcelsInput struct {
	County       string   `json:"county" jsonschema:"County (required)"`
	District     string   `json:"district" jsonschema:"District (required)"`
	Section      string   `json:"section,omitempty" jsonschema:"Land section"`
	AreaMinSqm   *float64 `json:"area_min_sqm,omitempty" jsonschema:"Minimum area in sqm"`
	AreaMaxSqm   *float64 `json:"area_max_sqm,omitempty" jsonschema:"Maximum area in sqm"`
	UrbanZoning  string   `json:"urban_zoning,omitempty" jsonschema:"Urban zoning code"`
	Limit        int      `json:"limit,omitempty" jsonschema:"Max results (default 100)"`
	Offset       int      `json:"offset,omitempty" jsonschema:"Offset (default 0)"`
}

func registerParcelTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_parcel",
			Description: "Get a single parcel by full four-key location (county, district, section, land_number).",
		},
		getParcelHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "search_parcels",
			Description: "Search parcels with filters for area, zoning, and location. Returns paginated results.",
		},
		searchParcelsHandler(s),
	)
}

type parcelOutput struct {
	Parcel domain.Parcel `json:"parcel"`
}

func getParcelHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelInput) (*mcpapi.CallToolResult, parcelOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getParcelInput) (*mcpapi.CallToolResult, parcelOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, parcelOutput{}, err
		}
		if input.County == "" || input.District == "" || input.Section == "" || input.LandNumber == "" {
			return nil, parcelOutput{}, NewError(ErrorCodeInvalidArgument,
				"county, district, section, and land_number are all required")
		}

		repo := s.getParcelRepository()
		if repo == nil {
			return nil, parcelOutput{}, NewError(ErrorCodeInternalError, "parcel repository not configured")
		}

		parcel, err := repo.GetByLandNumber(ctx, input.County, input.District, input.Section, input.LandNumber)
		if err != nil {
			if err == repository.ErrParcelNotFound {
				return nil, parcelOutput{}, NewError(ErrorCodeParcelNotFound,
					"parcel not found: "+input.County+"/"+input.District+"/"+input.Section+"/"+input.LandNumber)
			}
			return nil, parcelOutput{}, wrapServiceError(err)
		}

		return nil, parcelOutput{Parcel: *parcel}, nil
	}
}

type searchParcelsOutput struct {
	Parcels    []*domain.Parcel `json:"parcels"`
	TotalCount int              `json:"total_count"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

func searchParcelsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input searchParcelsInput) (*mcpapi.CallToolResult, searchParcelsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input searchParcelsInput) (*mcpapi.CallToolResult, searchParcelsOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, searchParcelsOutput{}, err
		}
		if input.County == "" {
			return nil, searchParcelsOutput{}, NewError(ErrorCodeInvalidArgument, "county is required")
		}
		if input.District == "" {
			return nil, searchParcelsOutput{}, NewError(ErrorCodeInvalidArgument, "district is required")
		}
		if input.Limit <= 0 {
			input.Limit = 100
		}

		repo := s.getParcelRepository()
		if repo == nil {
			return nil, searchParcelsOutput{}, NewError(ErrorCodeInternalError, "parcel repository not configured")
		}

		var section *string
		if input.Section != "" {
			section = &input.Section
		}
		filter := repository.ParcelFilter{
			County:    input.County,
			District:  input.District,
			Section:   section,
			MinArea:   input.AreaMinSqm,
			MaxArea:   input.AreaMaxSqm,
			Limit:     int32(input.Limit),
			Offset:    int32(input.Offset),
		}
		parcels, err := repo.Search(ctx, filter)
		if err != nil {
			return nil, searchParcelsOutput{}, wrapServiceError(err)
		}

		return nil, searchParcelsOutput{
			Parcels:    parcels,
			TotalCount: len(parcels),
			Limit:      input.Limit,
			Offset:     input.Offset,
		}, nil
	}
}

func (s *Server) getParcelRepository() repository.ParcelRepository {
	return nil
}
