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
		instrument(s, "get_data_snapshot", "", getDataSnapshotHandler(s)),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_data_provenance",
			Description: "Get full provenance chain for a transaction or valuation result. Tracer bullets from source file → snapshot → record hash.",
		},
		instrument(s, "get_data_provenance", "", getDataProvenanceHandler(s)),
	)
}

type getDataSnapshotInput struct {
	SnapshotID string `json:"snapshot_id" jsonschema:"Snapshot ID (required)"`
}

type getDataSnapshotOutput struct {
	domain.DatasetSnapshot
}

func getDataSnapshotHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataSnapshotInput) (*mcpapi.CallToolResult, getDataSnapshotOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataSnapshotInput) (*mcpapi.CallToolResult, getDataSnapshotOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), getDataSnapshotOutput{}, nil
		}
		if input.SnapshotID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "snapshot_id is required")), getDataSnapshotOutput{}, nil
		}

		return nil, getDataSnapshotOutput{}, nil
	}
}

type getDataProvenanceInput struct {
	TransactionID *string `json:"transaction_id,omitempty" jsonschema:"Transaction ID"`
	ValuationID   *string `json:"valuation_id,omitempty" jsonschema:"Valuation result ID"`
	ParcelID      *string `json:"parcel_id,omitempty" jsonschema:"Parcel ID"`
}

type getDataProvenanceOutput struct {
	Chain domain.ProvenanceChain `json:"provenance_chain"`
}

func getDataProvenanceHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataProvenanceInput) (*mcpapi.CallToolResult, getDataProvenanceOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getDataProvenanceInput) (*mcpapi.CallToolResult, getDataProvenanceOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), getDataProvenanceOutput{}, nil
		}
		if input.TransactionID == nil && input.ValuationID == nil && input.ParcelID == nil {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument,
				"at least one of transaction_id, valuation_id, or parcel_id is required")), getDataProvenanceOutput{}, nil
		}

		return nil, getDataProvenanceOutput{}, nil
	}
}
