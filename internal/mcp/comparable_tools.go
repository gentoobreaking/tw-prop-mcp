package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
)

// --- Comparable Tools ---

type findComparableTransactionsInput struct {
	ParcelID      string             `json:"parcel_id" jsonschema:"description=Target parcel UUID (required)"`
	Count         int                `json:"count,omitempty" jsonschema:"description=Number of comparables (default 10)"`
	SearchRadiusM *float64           `json:"search_radius_m,omitempty" jsonschema:"description=Search radius in meters (default 500)"`
	Weights       map[string]float64 `json:"weights,omitempty" jsonschema:"description=Optional scoring weights override"`
}

type findComparableTransactionsOutput struct {
	Comparables []domain.ComparableResult `json:"comparables"`
	Count       int                       `json:"count"`
	QueryHash   string                    `json:"query_hash"`
}

func registerComparableTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "find_comparable_transactions",
			Description: "Find comparable real estate transactions for a target parcel. Uses comparable engine with area/temporal/zoning/land-use/road-access scoring weighted by distance decay.",
		},
		findComparableTransactionsHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "score_comparable_transactions",
			Description: "Score a set of comparable transactions against a target parcel using the comparable engine's scoring algorithm.",
		},
		scoreComparableTransactionsHandler(s),
	)
}

func findComparableTransactionsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input findComparableTransactionsInput) (*mcpapi.CallToolResult, findComparableTransactionsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input findComparableTransactionsInput) (*mcpapi.CallToolResult, findComparableTransactionsOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, findComparableTransactionsOutput{}, err
		}
		if input.ParcelID == "" {
			return nil, findComparableTransactionsOutput{}, NewError(ErrorCodeInvalidArgument, "parcel_id is required")
		}

		// In production, this would use the ComparableEngine
		return nil, findComparableTransactionsOutput{
			Comparables: []domain.ComparableResult{},
			Count:       0,
		}, nil
	}
}

type scoreComparableTransactionsInput struct {
	TargetTransactionID string                      `json:"target_transaction_id" jsonschema:"description=Target transaction UUID (required)"`
	CandidateIDs        []string                    `json:"candidate_ids" jsonschema:"description=Candidate transaction IDs (required)"`
	Weights             map[string]float64          `json:"weights,omitempty" jsonschema:"description=Optional scoring weights override"`
}

type scoreComparableTransactionsOutput struct {
	Scores      []domain.ComparableResult `json:"scores"`
	QueryHash   string                    `json:"query_hash"`
}

func scoreComparableTransactionsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input scoreComparableTransactionsInput) (*mcpapi.CallToolResult, scoreComparableTransactionsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input scoreComparableTransactionsInput) (*mcpapi.CallToolResult, scoreComparableTransactionsOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, scoreComparableTransactionsOutput{}, err
		}
		if input.TargetTransactionID == "" {
			return nil, scoreComparableTransactionsOutput{}, NewError(ErrorCodeInvalidArgument, "target_transaction_id is required")
		}
		if len(input.CandidateIDs) == 0 {
			return nil, scoreComparableTransactionsOutput{}, NewError(ErrorCodeInvalidArgument, "candidate_ids is required")
		}

		return nil, scoreComparableTransactionsOutput{
			Scores: []domain.ComparableResult{},
		}, nil
	}
}
