package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

type testPlanner struct {
	plan *entities.ApiExecutionPlan
	err  error
}

func (p *testPlanner) Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error) {
	if p.err != nil {
		return nil, p.err
	}

	if p.plan != nil {
		return p.plan, nil
	}

	return &entities.ApiExecutionPlan{
		SchemaVersion:    "1.0",
		RequestID:        request.RequestID,
		Marketplace:      "wildberries",
		Status:           "ready",
		Intent:           request.Intent,
		RiskLevel:        "read",
		RequiresApproval: false,
		ExecutionMode:    request.Constraints.ExecutionMode,
		Inputs:           map[string]entities.InputValue{},
		Steps:            []entities.ApiPlanStep{},
		Transforms:       []entities.TransformStep{},
		FinalOutput: entities.FinalOutput{
			Type:        "object",
			Description: "Test output.",
			Fields:      map[string]any{},
		},
		Warnings: []entities.PlanWarning{},
		Validation: entities.PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: true,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      true,
			Errors:                []string{},
		},
	}, nil
}

type testRegistry struct{}

func (r *testRegistry) SearchOperations(ctx context.Context, query wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	return []entities.WBRegistryOperation{}, nil
}

func (r *testRegistry) GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error) {
	return nil, nil
}

func (r *testRegistry) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}

func TestHandleRPCInvalidJSON(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{}, &testRegistry{})

	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader("{bad json"))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer a.b.c")

	response := httptest.NewRecorder()

	handler.HandleRPC(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}

	var body entities.JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.Error == nil {
		t.Fatal("expected JSON-RPC error")
	}

	if body.Error.Code != -32700 {
		t.Fatalf("expected code -32700, got %d", body.Error.Code)
	}

	if body.Error.Message != "parse error" {
		t.Fatalf("expected parse error, got %q", body.Error.Message)
	}
}

func TestHandleRPCUnknownMethod(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{}, &testRegistry{})

	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(`{
		"jsonrpc": "2.0",
		"id": "req_unknown",
		"method": "unknown",
		"params": {}
	}`))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler.HandleRPC(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var body entities.JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.ID != "req_unknown" {
		t.Fatalf("expected id req_unknown, got %#v", body.ID)
	}

	if body.Error == nil {
		t.Fatal("expected JSON-RPC error")
	}

	if body.Error.Code != -32601 {
		t.Fatalf("expected code -32601, got %d", body.Error.Code)
	}

	if body.Error.Message != "method not found" {
		t.Fatalf("expected method not found, got %q", body.Error.Message)
	}
}

func TestHandleRPCMethodMustBePost(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{}, &testRegistry{})

	request := httptest.NewRequest(http.MethodGet, "/a2a", nil)
	response := httptest.NewRecorder()

	handler.HandleRPC(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", response.Code)
	}

	var body entities.JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.Error == nil {
		t.Fatal("expected JSON-RPC error")
	}

	if body.Error.Code != -32600 {
		t.Fatalf("expected code -32600, got %d", body.Error.Code)
	}
}

func TestHandleRPCValidMessageSend(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{
		plan: &entities.ApiExecutionPlan{
			SchemaVersion:    "1.0",
			RequestID:        "req_test_a2a",
			Marketplace:      "wildberries",
			Status:           "ready",
			Intent:           "get_inventory_and_sales",
			RiskLevel:        "read",
			RequiresApproval: false,
			ExecutionMode:    "automatic",
			Inputs:           map[string]entities.InputValue{},
			Steps:            []entities.ApiPlanStep{},
			Transforms:       []entities.TransformStep{},
			FinalOutput: entities.FinalOutput{
				Type:        "object",
				Description: "Test output.",
				Fields:      map[string]any{},
			},
			Warnings: []entities.PlanWarning{},
			Validation: entities.PlanValidation{
				RegistryChecked:       true,
				OutputSchemaChecked:   true,
				ReadonlyPolicyChecked: true,
				SecretsPolicyChecked:  true,
				JamPolicyChecked:      true,
				Errors:                []string{},
			},
		},
	}, &testRegistry{})

	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(`{
		"jsonrpc": "2.0",
		"id": "req_3",
		"method": "message/send",
		"params": {
			"request_id": "req_test_a2a",
			"marketplace": "wildberries",
			"intent": "get_inventory_and_sales",
			"natural_language_request": "Получить остатки и продажи",
			"entities": {
				"warehouse_id": 507,
				"chrt_ids": [12345678, 87654321]
			},
			"period": {
				"from": "2026-03-26",
				"to": "2026-04-26"
			},
			"constraints": {
				"readonly_only": true,
				"no_jam_subscription": true,
				"max_steps": 10,
				"execution_mode": "automatic"
			}
		}
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer a.b.c")

	response := httptest.NewRecorder()

	handler.HandleRPC(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var body entities.JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.ID != "req_3" {
		t.Fatalf("expected id req_3, got %#v", body.ID)
	}

	if body.Error != nil {
		t.Fatalf("expected no JSON-RPC error, got %#v", body.Error)
	}

	resultPayload, err := json.Marshal(body.Result)
	if err != nil {
		t.Fatal(err)
	}

	var plan entities.ApiExecutionPlan
	if err := json.Unmarshal(resultPayload, &plan); err != nil {
		t.Fatal(err)
	}

	if plan.Status != "ready" {
		t.Fatalf("expected ready plan, got %s", plan.Status)
	}

	if plan.RequestID != "req_test_a2a" {
		t.Fatalf("expected request_id req_test_a2a, got %s", plan.RequestID)
	}
	if plan.Metadata == nil || plan.Metadata.CorrelationID != "req_test_a2a" {
		t.Fatalf("expected metadata.correlation_id=req_test_a2a, got %#v", plan.Metadata)
	}
}

func TestParseBusinessRequestAcceptsOptionalMetadataObject(t *testing.T) {
	request, err := parseBusinessRequest(json.RawMessage(`{
		"request_id": "req_meta",
		"marketplace": "wildberries",
		"natural_language_request": "Get stocks",
		"metadata": {"session_id": "sess_1"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if request.Metadata == nil || request.Metadata.SessionID != "sess_1" {
		t.Fatalf("expected metadata session_id sess_1, got %#v", request.Metadata)
	}
}

func TestParseBusinessRequestCorrelationFallbackFromMetadata(t *testing.T) {
	request, err := parseBusinessRequest(json.RawMessage(`{
		"marketplace": "wildberries",
		"natural_language_request": "Get stocks",
		"metadata": {"correlation_id": "corr_only"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if request.RequestID != "corr_only" {
		t.Fatalf("expected request_id corr_only, got %q", request.RequestID)
	}
}

func TestHandleRPCValidMessageSendDoesNotExposeModelContentOrSelectionPlan(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{}, &testRegistry{})
	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(`{"jsonrpc":"2.0","id":"req_safe","method":"message/send","params":{"request_id":"req_safe","marketplace":"wildberries","natural_language_request":"Покажи остатки"}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer a.b.c")
	response := httptest.NewRecorder()
	handler.HandleRPC(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	body := response.Body.String()
	for _, forbidden := range []string{"\"parts\"", "\"role\":\"model\"", "ready_for_composition", "selected_operations", "registry_candidates"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked forbidden field %s: %s", forbidden, body)
		}
	}
	var rpc entities.JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error != nil {
		t.Fatalf("expected no JSON-RPC error, got %#v", rpc.Error)
	}
	resultBytes, _ := json.Marshal(rpc.Result)
	var plan entities.ApiExecutionPlan
	if err := json.Unmarshal(resultBytes, &plan); err != nil {
		t.Fatalf("expected ApiExecutionPlan result, got %s", string(resultBytes))
	}
	assertNoSelectorOrModelLeakage(t, response.Body.Bytes())
}

func TestHandleRPCBlockedMessageSendDoesNotExposeModelContentOrSelectionPlan(t *testing.T) {
	handler := NewHandler(Config{}, &testPlanner{plan: &entities.ApiExecutionPlan{SchemaVersion: "1.0", RequestID: "req_block", Marketplace: "wildberries", Status: "blocked", BlockReason: "api_plan_composition_failed", RiskLevel: "read", RequiresApproval: false, ExecutionMode: "not_executable", Inputs: map[string]entities.InputValue{}, Steps: []entities.ApiPlanStep{}, Transforms: []entities.TransformStep{}, FinalOutput: entities.FinalOutput{Type: "object", Description: "Blocked output.", Fields: map[string]any{}}, Warnings: []entities.PlanWarning{{Code: "api_plan_composition_error", Message: "unsupported"}}, Validation: entities.PlanValidation{RegistryChecked: true, OutputSchemaChecked: true, ReadonlyPolicyChecked: true, SecretsPolicyChecked: true, JamPolicyChecked: true, Errors: []string{}}}}, &testRegistry{})
	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(`{"jsonrpc":"2.0","id":"req_block","method":"message/send","params":{"request_id":"req_block","marketplace":"wildberries","natural_language_request":"Покажи остатки"}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer a.b.c")
	response := httptest.NewRecorder()
	handler.HandleRPC(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	body := response.Body.String()
	for _, forbidden := range []string{"\"parts\"", "\"role\":\"model\"", "ready_for_composition", "selected_operations", "registry_candidates"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked forbidden field %s: %s", forbidden, body)
		}
	}
	var rpc map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %#v", rpc["jsonrpc"])
	}
	result, ok := rpc["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %#v", rpc["result"])
	}
	if result["status"] != "blocked" {
		t.Fatalf("expected blocked status, got %#v", result["status"])
	}
	assertNoSelectorOrModelLeakage(t, response.Body.Bytes())
}

func assertNoSelectorOrModelLeakage(t *testing.T, payload []byte) {
	t.Helper()

	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		t.Fatalf("expected valid JSON response, got %v", err)
	}

	forbiddenContains := []string{
		"ready_for_composition",
		"selected_operations",
		"registry_candidates",
	}

	var walk func(node any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, value := range typed {
				if key == "parts" {
					t.Fatalf("detected forbidden key parts in payload: %s", string(payload))
				}

				if key == "role" {
					if role, ok := value.(string); ok && role == "model" {
						t.Fatalf("detected forbidden role=model in payload: %s", string(payload))
					}
				}

				for _, forbidden := range forbiddenContains {
					if key == forbidden || strings.Contains(key, forbidden) {
						t.Fatalf("detected forbidden key %q in payload: %s", key, string(payload))
					}
				}

				walk(value)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case string:
			for _, forbidden := range forbiddenContains {
				if typed == forbidden || strings.Contains(typed, forbidden) {
					t.Fatalf("detected forbidden string value %q in payload: %s", typed, string(payload))
				}
			}
		}
	}

	walk(root)
}
