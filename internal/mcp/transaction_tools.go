package mcp

import (
	"context"
	"encoding/json"
	"errors"
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
		searchTransactionsHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_transaction",
			Description: "Get a single transaction by ID with full provenance chain.",
		},
		getTransactionHandler(s),
	)

	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_transaction_statistics",
			Description: "Get price/area statistics for transactions matching filters (1 ping = 3.305785 sqm).",
		},
		getTransactionStatisticsHandler(s),
	)
}

// --- Tool handlers ---

// search_transactions input
type searchTransactionsInput struct {
	County          string     `json:"county" jsonschema:"description=County (required)"`
	District        string     `json:"district" jsonschema:"description=District (required)"`
	Section         string     `json:"section,omitempty" jsonschema:"description=Land section"`
	LandNumber      string     `json:"land_number,omitempty" jsonschema:"description=Land number"`
	TransactionType string     `json:"transaction_type,omitempty" jsonschema:"description=Transaction type"`
	DateFrom        *time.Time `json:"date_from,omitempty" jsonschema:"description=Start date"`
	DateTo          *time.Time `json:"date_to,omitempty" jsonschema:"description=End date"`
	Limit           int        `json:"limit,omitempty" jsonschema:"description=Max results (default 100)"`
	Offset          int        `json:"offset,omitempty" jsonschema:"description=Offset (default 0)"`
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
		if err := checkAIIsolation(req); err != nil {
			return nil, transactionSearchOutput{}, err
		}

		if input.County == "" {
			return nil, transactionSearchOutput{}, NewError(ErrorCodeInvalidArgument, "county is required")
		}
		if input.District == "" {
			return nil, transactionSearchOutput{}, NewError(ErrorCodeInvalidArgument, "district is required")
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
			return nil, transactionSearchOutput{}, wrapServiceError(err)
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
	TransactionID string `json:"transaction_id" jsonschema:"description=Transaction ID (required)"`
}) (*mcpapi.CallToolResult, *service.TransactionData, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input struct {
		TransactionID string `json:"transaction_id" jsonschema:"description=Transaction ID (required)"`
	}) (*mcpapi.CallToolResult, *service.TransactionData, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, nil, err
		}
		if input.TransactionID == "" {
			return nil, nil, NewError(ErrorCodeInvalidArgument, "transaction_id is required")
		}

		svc := s.getTransactionService()
		data, err := svc.GetTransaction(ctx, input.TransactionID)
		if err != nil {
			if errors.Is(err, repository.ErrTransactionNotFound) {
				return nil, nil, NewError(ErrorCodeTransactionNotFound, "transaction not found: "+input.TransactionID)
			}
			return nil, nil, wrapServiceError(err)
		}
		return nil, data, nil
	}
}

type getTransactionStatisticsInput struct {
	County   string `json:"county" jsonschema:"description=County (required)"`
	District string `json:"district" jsonschema:"description=District (required)"`
	Section  string `json:"section,omitempty" jsonschema:"description=Land section"`
}

func getTransactionStatisticsHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input getTransactionStatisticsInput) (*mcpapi.CallToolResult, *service.StatisticsResult, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input getTransactionStatisticsInput) (*mcpapi.CallToolResult, *service.StatisticsResult, error) {
		if err := checkAIIsolation(req); err != nil {
			return nil, nil, err
		}
		if input.County == "" {
			return nil, nil, NewError(ErrorCodeInvalidArgument, "county is required")
		}
		if input.District == "" {
			return nil, nil, NewError(ErrorCodeInvalidArgument, "district is required")
		}

		params := service.StatisticsParams{
			County:   input.County,
			District: input.District,
			Section:  input.Section,
		}

		svc := s.getTransactionService()
		result, err := svc.GetTransactionStatistics(ctx, params)
		if err != nil {
			return nil, nil, wrapServiceError(err)
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
func checkAIIsolation(req *mcpapi.CallToolRequest) error {
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

// wrapServiceError converts service errors to MCP errors.
func wrapServiceError(err error) *McpError {
	if mce := IsMcpError(err); mce != nil {
		return mce
	}
	return NewError(ErrorCodeInternalError, err.Error())
}
