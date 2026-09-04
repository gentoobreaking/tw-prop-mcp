package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/service"
)

// --- Tool registration ---

func registerTransactionTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "search_transactions",
			Description: "Search real estate transactions with structured filters. Returns transactions with price statistics and provenance.",
		},
		instrument(s, "search_transactions", "transaction", searchTransactionsHandler(s)),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_transaction",
			Description: "Get a single transaction by ID with full provenance chain.",
		},
		instrument(s, "get_transaction", "transaction", getTransactionHandler(s)),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_transaction_statistics",
			Description: "Get price/area statistics for transactions matching filters (1 ping = 3.305785 sqm).",
		},
		instrument(s, "get_transaction_statistics", "transaction", getTransactionStatisticsHandler(s)),
	)
}

// --- Tool handlers ---

// search_transactions input
type searchTransactionsInput struct {
	County          string     `json:"county" jsonschema:"County (required)"`
	District        string     `json:"district" jsonschema:"District (required)"`
	Section         string     `json:"section,omitempty" jsonschema:"Land section"`
	LandNumber      string     `json:"land_number,omitempty" jsonschema:"Land number"`
	TransactionType string     `json:"transaction_type,omitempty" jsonschema:"Transaction type"`
	DateFrom        *time.Time `json:"date_from,omitempty" jsonschema:"Start date"`
	DateTo          *time.Time `json:"date_to,omitempty" jsonschema:"End date"`
	Limit           int        `json:"limit,omitempty" jsonschema:"Max results (default 100)"`
	Offset          int        `json:"offset,omitempty" jsonschema:"Offset (default 0)"`
}

// Output struct for search_transactions
type transactionSearchOutput struct {
	Transactions []service.TransactionData `json:"transactions"`
	Statistics   service.PriceStats        `json:"statistics"`
	Count        int                       `json:"count"`
	TotalCount   int                       `json:"total_count"`
	Provenance   []service.ProvenanceInfo  `json:"data_provenance"`
	Metadata     service.SearchMetadata    `json:"metadata"`
}

func searchTransactionsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input searchTransactionsInput) (*mcpapi.CallToolResult, transactionSearchOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input searchTransactionsInput) (*mcpapi.CallToolResult, transactionSearchOutput, error) {
		// AI Isolation check on raw arguments
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), transactionSearchOutput{}, nil
		}

		if input.County == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "county is required")), transactionSearchOutput{}, nil
		}
		if input.District == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "district is required")), transactionSearchOutput{}, nil
		}

		params := service.SearchParams{
			County:          input.County,
			District:        input.District,
			Section:         input.Section,
			LandNumber:      input.LandNumber,
			TransactionType: input.TransactionType,
			DateFrom:        input.DateFrom,
			DateTo:          input.DateTo,
			Limit:           input.Limit,
			Offset:          input.Offset,
		}

		svc := s.getTransactionService()
		result, err := svc.SearchTransactions(ctx, params)
		if err != nil {
			return mcpErrorResult(wrapServiceError(err)), transactionSearchOutput{}, nil
		}

		output := transactionSearchOutput{
			Transactions: result.Data,
			Count:        len(result.Data),
			TotalCount:   result.TotalCount,
			Provenance:   result.DataProvenance,
			Metadata:     result.Metadata,
		}

		return nil, output, nil
	}
}

func getTransactionHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input struct {
	TransactionID string `json:"transaction_id" jsonschema:"Transaction ID (required)"`
}) (*mcpapi.CallToolResult, *service.TransactionData, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input struct {
		TransactionID string `json:"transaction_id" jsonschema:"Transaction ID (required)"`
	}) (*mcpapi.CallToolResult, *service.TransactionData, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), nil, nil
		}
		if input.TransactionID == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "transaction_id is required")), nil, nil
		}

		svc := s.getTransactionService()
		data, err := svc.GetTransaction(ctx, input.TransactionID)
		if err != nil {
			if errors.Is(err, repository.ErrTransactionNotFound) {
				return mcpErrorResult(NewError(ErrorCodeTransactionNotFound, "transaction not found: "+input.TransactionID)), nil, nil
			}
			return mcpErrorResult(wrapServiceError(err)), nil, nil
		}
		return nil, data, nil
	}
}

type getTransactionStatisticsInput struct {
	County   string `json:"county" jsonschema:"County (required)"`
	District string `json:"district" jsonschema:"District (required)"`
	Section  string `json:"section,omitempty" jsonschema:"Land section"`
}

func getTransactionStatisticsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getTransactionStatisticsInput) (*mcpapi.CallToolResult, *service.StatisticsResult, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getTransactionStatisticsInput) (*mcpapi.CallToolResult, *service.StatisticsResult, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), nil, nil
		}
		if input.County == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "county is required")), nil, nil
		}
		if input.District == "" {
			return mcpErrorResult(NewError(ErrorCodeInvalidArgument, "district is required")), nil, nil
		}

		params := service.StatisticsParams{
			County:   input.County,
			District: input.District,
			Section:  input.Section,
		}

		svc := s.getTransactionService()
		result, err := svc.GetTransactionStatistics(ctx, params)
		if err != nil {
			return mcpErrorResult(wrapServiceError(err)), nil, nil
		}

		return nil, result, nil
	}
}

// --- Engine wiring helpers ---

func (s *Server) getTransactionService() *service.TransactionService {
	repo := s.getTransactionRepository()
	return service.NewTransactionService(repo)
}

func (s *Server) getTransactionRepository() repository.TransactionRepository {
	if s.config.DatabaseDSN != "" {
		// Would create real DB-backed repo here
	}
	return nil
}

// checkAIIsolation validates that the raw tool arguments don't contain
// prohibited fields (sql, where, postgis, etc.).
// Returns nil if valid, or a *McpError describing the violation.
func checkAIIsolation(req *mcpapi.CallToolRequest) *McpError {
	rawArgs := req.Params.Arguments
	if len(rawArgs) == 0 {
		return nil
	}
	var input map[string]any
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return NewError(ErrorCodeInvalidArgument, "invalid JSON arguments")
	}
	for key := range input {
		if ProhibitedFields[key] {
			return NewError(ErrorCodeInvalidArgument,
				"prohibited field '"+key+"': AI isolation requires structured parameters only")
		}
	}
	return nil
}

// mcpErrorResult converts an McpError into a CallToolResult with IsError=true
// and the error envelope serialized as JSON text content.
func mcpErrorResult(mce *McpError) *mcpapi.CallToolResult {
	content := fmt.Sprintf(`{"error":{"code":"%s","message":"%s","retryable":%t}}`,
		mce.Code, mce.Message, mce.Retryable)
	return &mcpapi.CallToolResult{
		Content: []mcpapi.Content{
			&mcpapi.TextContent{Text: content},
		},
		IsError: true,
	}
}

// wrapServiceError converts service errors to MCP errors.
func wrapServiceError(err error) *McpError {
	if mce := IsMcpError(err); mce != nil {
		return mce
	}
	return NewError(ErrorCodeInternalError, err.Error())
}
