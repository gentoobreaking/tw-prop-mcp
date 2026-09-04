package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
)

// --- Valuation Tools ---

type estimateLandValueInput struct {
	ParcelID             string `json:"parcel_id" jsonschema:"Target parcel UUID (required)"`
	SnapshotID           string `json:"snapshot_id,omitempty" jsonschema:"Dataset snapshot ID"`
	AlgorithmVersion     string `json:"algorithm_version,omitempty" jsonschema:"Algorithm version"`
	ConfigurationVersion string `json:"configuration_version,omitempty" jsonschema:"Configuration version"`
	OutlierMethod        string `json:"outlier_method,omitempty" jsonschema:"Outlier method (IQR, P10_P90, MAD)"`
}

type estimateLandValueOutput struct {
	Result   domain.ValuationResult `json:"result"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

func registerValuationTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "estimate_land_value",
			Description: "Estimate land value for a target parcel using comparable transactions (per ping, yuan/坪). Returns bear/base/bull values with confidence and full provenance chain.",
		},
		estimateLandValueHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "estimate_property_value",
			Description: "Estimate total property value including building valuation on top of land value.",
		},
		estimatePropertyValueHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "explain_valuation",
			Description: "Get a detailed explanation of a valuation result including scoring methodology, comparable analysis, and confidence reasoning.",
		},
		explainValuationHandler(s),
	)
}

func estimateLandValueHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input estimateLandValueInput) (*mcpapi.CallToolResult, estimateLandValueOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input estimateLandValueInput) (*mcpapi.CallToolResult, estimateLandValueOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), estimateLandValueOutput{}, nil
		}
		if input.ParcelID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "parcel_id is required")), estimateLandValueOutput{}, nil
		}

		return nil, estimateLandValueOutput{}, nil
	}
}

type estimatePropertyValueInput struct {
	ParcelID          string   `json:"parcel_id" jsonschema:"Target parcel UUID (required)"`
	BuildingAreaSqm   *float64 `json:"building_area_sqm,omitempty" jsonschema:"Building area in sqm"`
	BuildingType      string   `json:"building_type,omitempty" jsonschema:"Building type (e.g., residential, commercial)"`
	BuildingAge       *int     `json:"building_age,omitempty" jsonschema:"Building age in years"`
	ValuationResultID string   `json:"valuation_result_id,omitempty" jsonschema:"Existing land valuation result ID"`
}

type estimatePropertyValueOutput struct {
	LandValue     int64                 `json:"land_value"`
	BuildingValue int64                 `json:"building_value"`
	TotalValue    int64                 `json:"total_value"`
	Confidence    domain.ConfidenceLevel `json:"confidence"`
}

func estimatePropertyValueHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input estimatePropertyValueInput) (*mcpapi.CallToolResult, estimatePropertyValueOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input estimatePropertyValueInput) (*mcpapi.CallToolResult, estimatePropertyValueOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), estimatePropertyValueOutput{}, nil
		}
		if input.ParcelID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "parcel_id is required")), estimatePropertyValueOutput{}, nil
		}

		return nil, estimatePropertyValueOutput{}, nil
	}
}

type explainValuationInput struct {
	ValuationID string `json:"valuation_id" jsonschema:"Valuation result ID (required)"`
	DetailLevel string `json:"detail_level,omitempty" jsonschema:"Detail level (basic, detailed, full)"`
}

type explainValuationOutput struct {
	Explanation string                    `json:"explanation"`
	Methodology string                    `json:"methodology"`
	Comparables []domain.ComparableResult `json:"comparables,omitempty"`
	Provenance  domain.ProvenanceChain    `json:"provenance"`
}

func explainValuationHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input explainValuationInput) (*mcpapi.CallToolResult, explainValuationOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input explainValuationInput) (*mcpapi.CallToolResult, explainValuationOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), explainValuationOutput{}, nil
		}
		if input.ValuationID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "valuation_id is required")), explainValuationOutput{}, nil
		}

		return nil, explainValuationOutput{}, nil
	}
}
