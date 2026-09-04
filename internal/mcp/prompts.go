package mcp

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"

	"tw-prop-mcp/internal/domain"
)

// --- MCP Prompts ---

// registerPrompts registers AI agent prompt templates.
// Per T017 acceptance criteria, 3 prompt templates are required:
//   - explain_valuation: explain why a valuation was estimated at a given value
//   - analyze_comparables: summarize comparable transaction analysis
//   - debug_transaction: help debug an unexpected transaction result
func (s *Server) registerPrompts() {
	// Prompt 1: Explain Valuation
	s.server.AddPrompt(&mcpapi.Prompt{
		Name:        "prompt_explain_valuation",
		Title:       "Explain Land Valuation",
		Description: "Explain why the land valuation engine produced a specific estimated value. Call after estimate_land_value. Covers comparable selection, outlier exclusion, statistical methodology, and confidence scoring.",
		Arguments: []*mcpapi.PromptArgument{
			{Name: "transaction_id", Title: "Transaction ID", Description: "UUID of the transaction that was valued", Required: true},
			{Name: "snapshot_id", Title: "Snapshot ID", Description: "Dataset snapshot to use (default: latest)", Required: false},
			{Name: "verbose", Title: "Verbose", Description: "Include detailed comparable list and scoring breakdown (true/false)", Required: false},
		},
	}, promptExplainValuationHandler(s))

	// Prompt 2: Analyze Comparables
	s.server.AddPrompt(&mcpapi.Prompt{
		Name:        "prompt_analyze_comparables",
		Title:       "Analyze Comparable Transactions",
		Description: "Generate a structured analysis of comparable transactions identified for a given parcel. Call after find_comparable_transactions to review filtering criteria, scoring rationale, and outlier exclusion.",
		Arguments: []*mcpapi.PromptArgument{
			{Name: "parcel_id", Title: "Parcel ID", Description: "UUID of the parcel to analyze comparables for", Required: true},
			{Name: "transaction_count", Title: "Transaction Count", Description: "Number of comparables to analyze (default: 10)", Required: false},
			{Name: "include_outliers", Title: "Include Outliers", Description: "Include outlier transactions in analysis (true/false)", Required: false},
		},
	}, promptAnalyzeComparablesHandler(s))

	// Prompt 3: Debug Transaction
	s.server.AddPrompt(&mcpapi.Prompt{
		Name:        "prompt_debug_transaction",
		Title:       "Debug Transaction Query",
		Description: "Diagnose why a transaction query returned unexpected results. Reviews query hash, provenance, snapshot freshness, and validation status.",
		Arguments: []*mcpapi.PromptArgument{
			{Name: "transaction_id", Title: "Transaction ID", Description: "UUID of the transaction to debug", Required: true},
			{Name: "snapshot_id", Title: "Snapshot ID", Description: "Dataset snapshot to check provenance for (default: latest)", Required: false},
			{Name: "expected_field", Title: "Expected Field", Description: "Specific field that returned unexpected value (optional)", Required: false},
		},
	}, promptDebugTransactionHandler(s))
}

// explainValuationHandler generates a prompt that explains a valuation result.
func promptExplainValuationHandler(s *Server) mcpapi.PromptHandler {
	return func(ctx context.Context, req *mcpapi.GetPromptRequest) (*mcpapi.GetPromptResult, error) {
		args := req.Params.Arguments
		txnID, ok := args["transaction_id"]
		if !ok || txnID == "" {
			return nil, fmt.Errorf("transaction_id is required")
		}

		verbose := args["verbose"] == "true"

		// Fetch transaction context for provenance
		var txn domain.Transaction
		var txnFound bool
		if s.TxRepo != nil {
			result, err := s.TxRepo.GetByID(ctx, uuid.MustParse(txnID))
			if err == nil {
				txn = result
				txnFound = true
			}
		}
		var txnPtr *domain.Transaction
		if txnFound {
			txnPtr = &txn
		}

		systemMsg := `You are a real estate valuation assistant for Taiwan. Explain land valuation methodology clearly, citing comparable transactions, outlier exclusion, and statistical methods.`

		humanMsg := buildExplanationPrompt(txnID, txnPtr, verbose, s.configVersion)

		return &mcpapi.GetPromptResult{
			Description: "Explain land valuation methodology for a specific transaction",
			Messages: []*mcpapi.PromptMessage{
				{
					Role:    "system",
					Content: &mcpapi.TextContent{Text: systemMsg},
				},
				{
					Role:    "user",
					Content: &mcpapi.TextContent{Text: humanMsg},
				},
			},
		}, nil
	}
}

// analyzeComparablesHandler generates a prompt for comparative market analysis.
func promptAnalyzeComparablesHandler(s *Server) mcpapi.PromptHandler {
	return func(ctx context.Context, req *mcpapi.GetPromptRequest) (*mcpapi.GetPromptResult, error) {
		args := req.Params.Arguments
		parcelID, ok := args["parcel_id"]
		if !ok || parcelID == "" {
			return nil, fmt.Errorf("parcel_id is required")
		}

		txnCount := 10
		if countStr := args["transaction_count"]; countStr != "" {
			if c, err := strconv.Atoi(countStr); err == nil && c > 0 {
				txnCount = c
			}
		}
		includeOutliers := args["include_outliers"] == "true"

		// Fetch parcel for context
		var parcel *domain.Parcel
		if s.ParcelRepo != nil {
			parcel, _ = s.ParcelRepo.GetByID(ctx, parcelID)
		}

		parcelCtx := "Parcel ID: " + parcelID
		if parcel != nil {
			parcelCtx = fmt.Sprintf("Parcel %s in %s %s (section: %s, land_number: %s)",
				parcelID, parcel.County, parcel.District, parcel.Section, parcel.LandNumber)
		}

		systemMsg := `You are a real estate comparable analysis expert. Review comparable transactions, explain filtering criteria, scoring methodology, and outlier exclusion logic.`

		humanMsg := buildComparableAnalysisPrompt(parcelCtx, txnCount, includeOutliers, s.configVersion)

		return &mcpapi.GetPromptResult{
			Description: "Analyze comparable transactions for a parcel",
			Messages: []*mcpapi.PromptMessage{
				{
					Role:    "system",
					Content: &mcpapi.TextContent{Text: systemMsg},
				},
				{
					Role:    "user",
					Content: &mcpapi.TextContent{Text: humanMsg},
				},
			},
		}, nil
	}
}

// debugTransactionHandler generates a prompt for diagnosing transaction query issues.
func promptDebugTransactionHandler(s *Server) mcpapi.PromptHandler {
	return func(ctx context.Context, req *mcpapi.GetPromptRequest) (*mcpapi.GetPromptResult, error) {
		args := req.Params.Arguments
		txnID, ok := args["transaction_id"]
		if !ok || txnID == "" {
			return nil, fmt.Errorf("transaction_id is required")
		}

		field := args["expected_field"]

		// Fetch transaction metadata for provenance context
		var txn domain.Transaction
		var txnErr error
		if s.TxRepo != nil {
			txn, txnErr = s.TxRepo.GetByID(ctx, uuid.MustParse(txnID))
		}

		provenanceSummary := "No transaction found for this ID."
		if txnErr == nil {
			provenanceSummary = fmt.Sprintf(
				"Transaction ID: %s\nImport Batch ID: %s\nCounty: %s\nDistrict: %s\nSection: %s\nTransaction Date: %s\nTotal Price: %d TWD\nUnit Price: %d TWD/sqm\nSource Record Hash: %s",
				txn.TransactionID, txn.ImportBatchID, txn.County, txn.District,
				txn.Section, txn.TransactionDate.Format("2006-01-02"),
				txn.TotalPrice, txn.UnitPrice, txn.SourceRecordHash,
			)
		}

		systemMsg := `You are a real estate transaction debugging assistant. Diagnose why a transaction query returned unexpected results.`

		humanMsg := buildDebugPrompt(txnID, provenanceSummary, field, s.configVersion)

		return &mcpapi.GetPromptResult{
			Description: "Debug a transaction query issue",
			Messages: []*mcpapi.PromptMessage{
				{
					Role:    "system",
					Content: &mcpapi.TextContent{Text: systemMsg},
				},
				{
					Role:    "user",
					Content: &mcpapi.TextContent{Text: humanMsg},
				},
			},
		}, nil
	}
}

// --- Prompt builder helpers ---

// buildExplanationPrompt constructs the user message for valuation explanation.
func buildExplanationPrompt(transactionID string, txn *domain.Transaction, verbose bool, configVersion string) string {
	prompt := fmt.Sprintf("Explain the land valuation for transaction %s.\n\n", transactionID)

	if txn != nil {
		prompt += fmt.Sprintf("**Transaction Context:**\n- County: %s, District: %s, Section: %s\n- Total Price: %d TWD\n- Unit Price: %d TWD/sqm\n- Transaction Date: %s\n\n",
			txn.County, txn.District, txn.Section,
			txn.TotalPrice, txn.UnitPrice,
			txn.TransactionDate.Format("2006-01-02"))
	}

	prompt += fmt.Sprintf("**Configuration version:** %s\n\n", configVersion)

	prompt += `**Methodology to cover:**
1. **Comparable Selection**: How the engine filters candidates by county, district, section, and building type
2. **Outlier Exclusion**: IQR method (k=1.5) applied to unit prices of comparables
3. **Valuation Model**: Bear/Base/Bull values using P25/P50/P75 percentiles
4. **Confidence Scoring**: Based on comparable count and data quality
5. **Price Conversion**: TWD/sqm → TWD/ping using factor 3.305785

**Follow-up actions**: After explaining, call "search_transactions" with the same county/district/section to show broader market context.`

	if verbose {
		prompt += "\n\nInclude a detailed breakdown of each comparable transaction used, with its score and price per square meter."
	}

	return prompt
}

// buildComparableAnalysisPrompt constructs the user message for comparable analysis.
func buildComparableAnalysisPrompt(parcelCtx string, txnCount int, includeOutliers bool, configVersion string) string {
	prompt := fmt.Sprintf("Analyze the comparable transactions for: %s\n\n", parcelCtx)
	prompt += "**Parameters:**\n"
	prompt += fmt.Sprintf("- Target comparables: %d\n", txnCount)
	prompt += fmt.Sprintf("- Configuration version: %s\n", configVersion)
	if includeOutliers {
		prompt += "- Include outlier transactions in analysis\n"
	} else {
		prompt += "- Exclude outliers (IQR k=1.5)\n"
	}

	prompt += `
**Analysis to provide:**
1. **Filtering rationale**: Explain how county/district/section constraints narrow the candidate pool
2. **Scoring breakdown**: Weighted scoring based on age, area, floor, and transaction date similarity
3. **Outlier handling**: IQR-based detection and exclusion of anomalous unit prices
4. **Confidence assessment**: Number of valid comparables determines confidence (HIGH ≥5, MEDIUM ≥2, LOW <2)
5. **Market summary**: Synthesize comparable prices into a coherent market summary

**Follow-up actions**: After analysis, suggest whether more comparables should be loaded or if confidence is sufficient for valuation.`

	return prompt
}

// buildDebugPrompt constructs the user message for transaction debugging.
func buildDebugPrompt(transactionID string, provenanceSummary string, field string, configVersion string) string {
	prompt := fmt.Sprintf("Debug transaction %s — investigate why query results are unexpected.\n\n", transactionID)
	prompt += fmt.Sprintf("**Transaction Provenance:**\n%s\n\n", provenanceSummary)
	prompt += fmt.Sprintf("**Configuration version:** %s\n\n", configVersion)

	if field != "" {
		prompt += fmt.Sprintf("**Field of concern:** %s\n\n", field)
	}

	prompt += `**Debugging checklist:**
1. **Snapshot freshness**: Call "get_data_snapshot" to check if the active snapshot is current
2. **Provenance chain**: Call "get_data_provenance" to verify data lineage from source to transaction
3. **Query hash**: Check "metadata.query_hash" in the original response for reproducibility
4. **Data validation**: Verify the transaction passed all normalizer + validator checks
5. **Coordinate system**: Confirm GIS geometry is in correct EPSG (3826 source → 4326 target)

**Expected output**: Identify root cause (stale snapshot, validation gap, coordinate conversion issue, or data quality problem) and recommend next steps.`

	return prompt
}
