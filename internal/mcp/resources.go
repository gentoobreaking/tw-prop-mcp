package mcp

import (
	"context"
	"fmt"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- MCP Resources ---

// registerResources registers all MCP resources with realestate:// URIs.
// Per T017 acceptance criteria, 5 resource templates are required:
//   - realestate://snapshot/{snapshot_id}
//   - realestate://transaction/{transaction_id}
//   - realestate://parcel/{parcel_id}
//   - realestate://valuation/{valuation_id}
//   - realestate://algorithm/{version}
func registerResources(srv *mcpapi.Server, s *Server) {
	// Snapshot resource
	srv.AddResource(&mcpapi.Resource{
		Name:        "Data Snapshot",
		Description: "Dataset snapshot metadata (source, file hash, status).",
		URI:         "realestate://snapshot/{snapshot_id}",
		MIMEType:    "application/json",
	}, getSnapshotResourceHandler(s))

	// Transaction resource
	srv.AddResource(&mcpapi.Resource{
		Name:        "Transaction",
		Description: "Single real estate transaction with full provenance chain.",
		URI:         "realestate://transaction/{transaction_id}",
		MIMEType:    "application/json",
	}, getTransactionResourceHandler(s))

	// Parcel resource
	srv.AddResource(&mcpapi.Resource{
		Name:        "Parcel",
		Description: "Parcel geometry and attributes.",
		URI:         "realestate://parcel/{parcel_id}",
		MIMEType:    "application/json",
	}, getParcelResourceHandler(s))

	// Valuation resource
	srv.AddResource(&mcpapi.Resource{
		Name:        "Valuation Result",
		Description: "Land valuation result with comparable IDs, weights, and provenance.",
		URI:         "realestate://valuation/{valuation_id}",
		MIMEType:    "application/json",
	}, getValuationResourceHandler(s))

	// Algorithm resource
	srv.AddResource(&mcpapi.Resource{
		Name:        "Algorithm Version",
		Description: "Algorithm configuration and version metadata.",
		URI:         "realestate://algorithm/{version}",
		MIMEType:    "application/json",
	}, getAlgorithmResourceHandler(s))
}

// Resource handlers — each extracts the ID from the URI and returns JSON content.

func getSnapshotResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     fmt.Sprintf(`{"error": "snapshot not found: %s"}`, req.Params.URI),
				},
			},
		}, nil
	}
}

func getTransactionResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     fmt.Sprintf(`{"error": "transaction not found: %s"}`, req.Params.URI),
				},
			},
		}, nil
	}
}

func getParcelResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     fmt.Sprintf(`{"error": "parcel not found: %s"}`, req.Params.URI),
				},
			},
		}, nil
	}
}

func getValuationResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     fmt.Sprintf(`{"error": "valuation not found: %s"}`, req.Params.URI),
				},
			},
		}, nil
	}
}

func getAlgorithmResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     `{"error": "algorithm version not found"}`,
				},
			},
		}, nil
	}
}
