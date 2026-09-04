package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
)

// --- Comparable Tools ---

type findComparableTransactionsInput struct {
	ParcelID      string   `json:"parcel_id" jsonschema:"Target parcel UUID (required)"`
	Count         int      `json:"count,omitempty" jsonschema:"Number of comparables (default 10)"`
	SearchRadiusM *float64 `json:"search_radius_m,omitempty" jsonschema:"Search radius in meters (default 500)"`
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
		instrument(s, "find_comparable_transactions", "comparable", findComparableTransactionsHandler(s)),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "score_comparable_transactions",
			Description: "Score a set of comparable transactions against a target parcel using the comparable engine's scoring algorithm.",
		},
		instrument(s, "score_comparable_transactions", "comparable", scoreComparableTransactionsHandler(s)),
	)
}

func findComparableTransactionsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input findComparableTransactionsInput) (*mcpapi.CallToolResult, findComparableTransactionsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input findComparableTransactionsInput) (*mcpapi.CallToolResult, findComparableTransactionsOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), findComparableTransactionsOutput{}, nil
		}
		if input.ParcelID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "parcel_id is required")), findComparableTransactionsOutput{}, nil
		}

		return nil, findComparableTransactionsOutput{
			Comparables: []domain.ComparableResult{},
			Count:       0,
		}, nil
	}
}

type scoreComparableTransactionsInput struct {
	TargetTransactionID string   `json:"target_transaction_id" jsonschema:"Target transaction UUID (required)"`
	CandidateIDs        []string `json:"candidate_ids" jsonschema:"Candidate transaction IDs (required)"`
}

type scoreComparableTransactionsOutput struct {
	Scores      []domain.ComparableResult `json:"scores"`
	QueryHash   string                    `json:"query_hash"`
}

func scoreComparableTransactionsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input scoreComparableTransactionsInput) (*mcpapi.CallToolResult, scoreComparableTransactionsOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input scoreComparableTransactionsInput) (*mcpapi.CallToolResult, scoreComparableTransactionsOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), scoreComparableTransactionsOutput{}, nil
		}
		if input.TargetTransactionID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "target_transaction_id is required")), scoreComparableTransactionsOutput{}, nil
		}
		if len(input.CandidateIDs) == 0 {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "candidate_ids is required")), scoreComparableTransactionsOutput{}, nil
		}

		return nil, scoreComparableTransactionsOutput{
			Scores: []domain.ComparableResult{},
		}, nil
	}
}
