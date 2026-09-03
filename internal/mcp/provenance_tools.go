package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
)

// --- Provenance Tools ---

func registerProvenanceTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_data_snapshot",
			Description: "Get information about a dataset snapshot (import batch metadata, source file, status).",
		},
		getDataSnapshotHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_data_provenance",
			Description: "Get full provenance chain for a transaction or valuation result. Tracer bullets from source file → snapshot → record hash.",
		},
		getDataProvenanceHandler(s),
	)
}

type getDataSnapshotInput struct {
	SnapshotID string `json:"snapshot_id" jsonschema:"description=Snapshot ID (required)"`
}

type getDataSnapshotOutput struct {
	domain.DatasetSnapshot
}

func getDataSnapshotHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataSnapshotInput) (*mcpapi.CallToolResult, getDataSnapshotOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataSnapshotInput) (*mcpapi.CallToolResult, getDataSnapshotOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, getDataSnapshotOutput{}, err
		}
		if input.SnapshotID == "" {
			return nil, getDataSnapshotOutput{}, NewError(ErrorCodeInvalidArgument, "snapshot_id is required")
		}

		// In production, this would query the SnapshotRepository
		return nil, getDataSnapshotOutput{}, nil
	}
}

type getDataProvenanceInput struct {
	TransactionID  *string `json:"transaction_id,omitempty" jsonschema:"description=Transaction ID"`
	ValuationID   *string `json:"valuation_id,omitempty" jsonschema:"description=Valuation result ID"`
	ParcelID      *string `json:"parcel_id,omitempty" jsonschema:"description=Parcel ID"`
}

type getDataProvenanceOutput struct {
	Chain domain.ProvenanceChain `json:"provenance_chain"`
}

func getDataProvenanceHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataProvenanceInput) (*mcpapi.CallToolResult, getDataProvenanceOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataProvenanceInput) (*mcpapi.CallToolResult, getDataProvenanceOutput, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, getDataProvenanceOutput{}, err
		}
		if input.TransactionID == nil && input.ValuationID == nil && input.ParcelID == nil {
			return nil, getDataProvenanceOutput{}, NewError(ErrorCodeInvalidArgument,
				"at least one of transaction_id, valuation_id, or parcel_id is required")
		}

		return nil, getDataProvenanceOutput{}, nil
	}
}
