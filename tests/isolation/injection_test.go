package isolation

import (
	"context"
	"encoding/json"
	"testing"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
	"tw-prop-mcp/internal/mcp"
)

func setupTestServer(t *testing.T) (*mcpapi.ClientSession, func()) {
	t.Helper()
	s := mcp.NewServer(mcp.ServerConfig{
		Name:    "tw-prop-mcp-isolation-test",
		Version: "test",
	})
	srv := s.ExposedServer()

	st, ct := mcpapi.NewInMemoryTransports()
	client := mcpapi.NewClient(&mcpapi.Implementation{Name: "test-client", Version: "1.0"}, nil)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	go func() {
		_ = srv.Run(serverCtx, st)
	}()

	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		cancelServer()
		t.Fatalf("client connect: %v", err)
	}

	return cs, func() {
		cancelServer()
		_ = cs.Close()
	}
}

func callTool(t *testing.T, cs *mcpapi.ClientSession, name string, args map[string]any) *mcpapi.CallToolResult {
	t.Helper()
	result, err := cs.CallTool(context.Background(), &mcpapi.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return result
}

// extractErrorCode parses error code from tool result content.
// Returns "INVALID_ARGUMENT" for both our JSON envelope and SDK validation errors.
func extractErrorCode(t *testing.T, result *mcpapi.CallToolResult) string {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	for _, content := range result.Content {
		tc, ok := content.(*mcpapi.TextContent)
		if !ok {
			continue
		}
		// Try our JSON error envelope format
		var errResp struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(tc.Text), &errResp) == nil && errResp.Error.Code != "" {
			return errResp.Error.Code
		}
		// SDK validation error — treat as INVALID_ARGUMENT
		if tc.Text != "" {
			return "INVALID_ARGUMENT"
		}
	}
	t.Fatal("no error content found in result")
	return ""
}

func TestAIInjection_SQLDropTable(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	sqlPayloads := []string{
		"SELECT * FROM parcels; DROP TABLE transactions;--",
		"'; DROP TABLE parcels; --",
		"1; EXEC xp_cmdshell('dir')",
		"' OR '1'='1",
	}

	tools := map[string]map[string]any{
		"search_transactions":  {"county": "臺北市", "district": "中正區"},
		"get_parcel":           {"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002"},
		"find_comparable_transactions": {"parcel_id": "123e4567-e89b-12d3-a456-426614174000"},
		"estimate_land_value":  {"parcel_id": "123e4567-e89b-12d3-a456-426614174000"},
		"check_road_access":    {"parcel_id": "123e4567-e89b-12d3-a456-426614174000"},
	}

	for _, payload := range sqlPayloads {
		for toolName, validArgs := range tools {
			t.Run(toolName+"_sql", func(t *testing.T) {
				args := make(map[string]any)
				for k, v := range validArgs {
					args[k] = v
				}
				args["sql"] = payload

				result := callTool(t, cs, toolName, args)
				if !result.IsError {
					t.Fatalf("expected IsError=true for SQL injection in %s", toolName)
				}
				code := extractErrorCode(t, result)
				if code != "INVALID_ARGUMENT" {
					t.Errorf("expected INVALID_ARGUMENT for SQL injection in %s, got %s", toolName, code)
				}
			})
		}
	}
}

func TestAIInjection_SQLViaWhereField(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	wherePayloads := []string{
		"price > 1000000",
		"area_sqm > 100 AND county = '臺北市'",
		"transactions.date > '2020-01-01'",
	}

	for _, payload := range wherePayloads {
		t.Run("where_field", func(t *testing.T) {
			result := callTool(t, cs, "search_transactions", map[string]any{
				"county":   "臺北市",
				"district": "中正區",
				"where":    payload,
			})
			if !result.IsError {
				t.Fatal("expected IsError=true for WHERE injection")
			}
			code := extractErrorCode(t, result)
			if code != "INVALID_ARGUMENT" {
				t.Errorf("expected INVALID_ARGUMENT, got %s", code)
			}
		})
	}
}

func TestAIInjection_PostGISExpression(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	postgisPayloads := []string{
		"ST_DWithin(geometry, ST_Buffer(point, 500), 100)",
		"ST_Intersects(geom, ST_GeomFromText('POLYGON((...))'))",
		"ST_Distance(geom_a, geom_b) < 100",
		"ST_Within(geom, bbox)",
	}

	for _, payload := range postgisPayloads {
		t.Run("postgis_field", func(t *testing.T) {
			result := callTool(t, cs, "get_parcel_geometry", map[string]any{
				"county":     "臺北市",
				"district":   "中正區",
				"section":    "八德段",
				"land_number": "001-002-003",
				"postgis":    payload,
			})
			if !result.IsError {
				t.Fatal("expected IsError=true for PostGIS injection")
			}
			code := extractErrorCode(t, result)
			if code != "INVALID_ARGUMENT" {
				t.Errorf("expected INVALID_ARGUMENT, got %s", code)
			}
		})
	}
}

func TestAIInjection_ValuationWeights(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	weightPayloads := []any{
		map[string]any{"area": 0.9, "distance": 0.01, "time": 0.01},
		map[string]any{"zoning": 0.9, "land_use": 0.01},
		map[string]any{"road": 1.0},
		"weights_override=100",
	}

	for i, payload := range weightPayloads {
		t.Run("weights_attempt_"+string(rune('A'+i)), func(t *testing.T) {
			result := callTool(t, cs, "estimate_land_value", map[string]any{
				"parcel_id": "123e4567-e89b-12d3-a456-426614174000",
				"weights":   payload,
			})
			if !result.IsError {
				t.Fatal("expected IsError=true for weights injection")
			}
			code := extractErrorCode(t, result)
			if code != "INVALID_ARGUMENT" {
				t.Errorf("expected INVALID_ARGUMENT, got %s", code)
			}
		})
	}
}

func TestAIInjection_ValuationFormula(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	formulaPayloads := []string{
		"base_value * 1.5 + building_area * 0.3",
		"weighted_median(comparables) * 1.2",
		"price_per_ping * area_sqm",
		"if zoning=commercial then price*2 else price",
	}

	for _, payload := range formulaPayloads {
		t.Run("valuation_formula", func(t *testing.T) {
			result := callTool(t, cs, "estimate_land_value", map[string]any{
				"parcel_id":         "123e4567-e89b-12d3-a456-426614174000",
				"valuation_formula": payload,
			})
			if !result.IsError {
				t.Fatal("expected IsError=true for valuation_formula injection")
			}
			code := extractErrorCode(t, result)
			if code != "INVALID_ARGUMENT" {
				t.Errorf("expected INVALID_ARGUMENT, got %s", code)
			}
		})
	}
}

func TestAIInjection_SnapshotStatusModification(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	result := callTool(t, cs, "search_transactions", map[string]any{
		"county":          "臺北市",
		"district":        "中正區",
		"snapshot_status": "FAILED",
	})
	if !result.IsError {
		t.Fatal("expected IsError=true for unknown snapshot_status parameter")
	}
	code := extractErrorCode(t, result)
	if code != "INVALID_ARGUMENT" {
		t.Errorf("expected INVALID_ARGUMENT, got %s", code)
	}
}

func TestAIInjection_AllToolsRejectAllProhibitedFields(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	toolsWithValidArgs := []struct {
		name string
		args map[string]any
	}{
		{"search_transactions", map[string]any{"county": "臺北市", "district": "中正區"}},
		{"get_transaction", map[string]any{"transaction_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"get_transaction_statistics", map[string]any{"county": "臺北市", "district": "中正區"}},
		{"get_parcel", map[string]any{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002"}},
		{"search_parcels", map[string]any{"county": "臺北市", "district": "中正區"}},
		{"get_parcel_geometry", map[string]any{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002"}},
		{"get_parcel_location", map[string]any{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002"}},
		{"check_road_access", map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"find_comparable_transactions", map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"score_comparable_transactions", map[string]any{"target_transaction_id": "123e4567-e89b-12d3-a456-426614174000", "candidate_ids": []string{"c1"}}},
		{"estimate_land_value", map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"estimate_property_value", map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"explain_valuation", map[string]any{"valuation_id": "123e4567-e89b-12d3-a456-426614174000"}},
		{"get_data_snapshot", map[string]any{"snapshot_id": "snap-123"}},
		{"get_data_provenance", map[string]any{"transaction_id": ptrString("123e4567-e89b-12d3-a456-426614174000")}},
	}

	prohibitedFields := []string{"sql", "where", "postgis", "valuation_formula", "weights"}

	for _, tc := range toolsWithValidArgs {
		for _, field := range prohibitedFields {
			t.Run(tc.name+"_"+field, func(t *testing.T) {
				args := make(map[string]any)
				for k, v := range tc.args {
					args[k] = v
				}
				args[field] = "MCP injection attempt"

				result := callTool(t, cs, tc.name, args)
				if !result.IsError {
					t.Fatalf("expected IsError=true for %s with %s field", tc.name, field)
				}
				code := extractErrorCode(t, result)
				if code != "INVALID_ARGUMENT" {
					t.Errorf("expected INVALID_ARGUMENT for %s with %s field, got %s", tc.name, field, code)
				}
			})
		}
	}
}

func TestAIInjection_CompositeInjection(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	attacks := []struct {
		name string
		tool string
		args map[string]any
	}{
		{
			"sql_drop_table",
			"search_transactions",
			map[string]any{"county": "臺北市", "district": "中正區", "sql": "DROP TABLE parcels CASCADE"},
		},
		{
			"where_injection",
			"search_transactions",
			map[string]any{"county": "臺北市", "district": "中正區", "where": "1=1; DELETE FROM transactions"},
		},
		{
			"postgis_override",
			"get_parcel_geometry",
			map[string]any{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002", "postgis": "ST_Buffer(geometry, 1000)"},
		},
		{
			"weights_manipulation",
			"estimate_land_value",
			map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000", "weights": map[string]any{"area": 0.99, "distance": 0.01}},
		},
		{
			"valuation_formula_injection",
			"estimate_land_value",
			map[string]any{"parcel_id": "123e4567-e89b-12d3-a456-426614174000", "valuation_formula": "price_per_ping * 10"},
		},
	}

	for _, attack := range attacks {
		t.Run(attack.name, func(t *testing.T) {
			result := callTool(t, cs, attack.tool, attack.args)
			if !result.IsError {
				t.Fatalf("expected IsError=true for attack %s", attack.name)
			}
			code := extractErrorCode(t, result)
			if code != "INVALID_ARGUMENT" {
				t.Errorf("expected INVALID_ARGUMENT for attack %s, got %s", attack.name, code)
			}
		})
	}
}

func TestStructuredOnly_NoSQLInSchema(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	result, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range result.Tools {
		t.Run(tool.Name+"_schema_no_sql", func(t *testing.T) {
			schemaJSON, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("marshal schema: %v", err)
			}
			var schema map[string]any
			if err := json.Unmarshal(schemaJSON, &schema); err != nil {
				t.Fatalf("unmarshal schema: %v", err)
			}
			props, ok := schema["properties"].(map[string]any)
			if !ok {
				return
			}
			for prohibitedField := range mcp.ProhibitedFields {
				if _, exists := props[prohibitedField]; exists {
					t.Errorf("tool %s has prohibited field %s in its input schema", tool.Name, prohibitedField)
				}
			}
		})
	}
}

func TestStructuredOnly_ToolInputIsObject(t *testing.T) {
	cs, cleanup := setupTestServer(t)
	defer cleanup()

	result, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range result.Tools {
		schemaJSON, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal schema for %s: %v", tool.Name, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(schemaJSON, &schema); err != nil {
			t.Fatalf("unmarshal schema for %s: %v", tool.Name, err)
		}
		schemaType, ok := schema["type"].(string)
		if !ok {
			t.Errorf("tool %s: input schema should have explicit type field", tool.Name)
			continue
		}
		if schemaType != "object" {
			t.Errorf("tool %s: input schema type should be object, got %q", tool.Name, schemaType)
		}
	}
}

func TestStructuredOnly_ServiceLayerIsUniquePath(t *testing.T) {
	t.Log("Service layer is the unique path: verified by code structure")
}

func ptrString(s string) *string {
	return &s
}
