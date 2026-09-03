package mcp

import (
	"context"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- MCP Resources ---

func registerResources(srv *mcpapi.Server, s *Server) {
	// Static resource: realestate://snapshot/{snapshot_id}
	srv.AddResource(&mcpapi.Resource{
		Name:        "Data Snapshot",
		Description: "Dataset snapshot metadata (source, file hash, status).",
		URI:         "realestate://snapshot/{snapshot_id}",
		MIMEType:    "application/json",
	}, getSnapshotResourceHandler(s))

	// Static resource: realestate://parcel/{parcel_id}
	srv.AddResource(&mcpapi.Resource{
		Name:        "Parcel",
		Description: "Parcel geometry and attributes.",
		URI:         "realestate://parcel/{parcel_id}",
		MIMEType:    "application/json",
	}, getParcelResourceHandler(s))
}

// Resource handlers

func getSnapshotResourceHandler(s *Server) mcpapi.ResourceHandler {
	return func(ctx context.Context, req *mcpapi.ReadResourceRequest) (*mcpapi.ReadResourceResult, error) {
		// In production, this would extract snapshot_id from URI and query the repo
		return &mcpapi.ReadResourceResult{
			Contents: []*mcpapi.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     `{"error": "not implemented"}`,
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
					Text:     `{"error": "not implemented"}`,
				},
			},
		}, nil
	}
}
