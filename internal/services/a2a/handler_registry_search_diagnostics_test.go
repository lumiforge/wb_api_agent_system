package a2a

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestHandleRegistrySearchReturnsDiagnosticsWhenRegistrySupportsIt(t *testing.T) {
	registry := &diagnosticRegistryForHandlerTest{
		result: wbregistry.SearchResult{
			Operations: []entities.WBRegistryOperation{
				registrySearchHandlerTestOperation("generated_get_api_v1_supplier_sales"),
			},
			Diagnostics: wbregistry.SearchDiagnostics{
				LexicalCandidates:        3,
				SemanticCandidates:       2,
				MergedCandidates:         4,
				ReturnedCandidates:       1,
				SemanticExpansionEnabled: true,
			},
		},
	}

	handler := NewHandler(Config{
		PublicBaseURL: "http://example.test",
		Logger:        log.New(io.Discard, "", 0),
	}, &plannerForRegistrySearchHandlerTest{}, registry)

	request := httptest.NewRequest(http.MethodGet, "/debug/registry/search?q=остатки&limit=5&readonly_only=true&exclude_jam=true", nil)
	recorder := httptest.NewRecorder()

	handler.HandleRegistrySearch(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response wbregistry.SearchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}

	if len(response.Operations) != 1 {
		t.Fatalf("expected one operation, got %#v", response.Operations)
	}

	if response.Operations[0].OperationID != "generated_get_api_v1_supplier_sales" {
		t.Fatalf("unexpected operation_id %q", response.Operations[0].OperationID)
	}

	if response.Diagnostics.LexicalCandidates != 3 {
		t.Fatalf("expected lexical_candidates=3, got %d", response.Diagnostics.LexicalCandidates)
	}

	if response.Diagnostics.SemanticCandidates != 2 {
		t.Fatalf("expected semantic_candidates=2, got %d", response.Diagnostics.SemanticCandidates)
	}

	if response.Diagnostics.MergedCandidates != 4 {
		t.Fatalf("expected merged_candidates=4, got %d", response.Diagnostics.MergedCandidates)
	}

	if response.Diagnostics.ReturnedCandidates != 1 {
		t.Fatalf("expected returned_candidates=1, got %d", response.Diagnostics.ReturnedCandidates)
	}

	if !response.Diagnostics.SemanticExpansionEnabled {
		t.Fatal("expected semantic_expansion_enabled=true")
	}

	if registry.lastQuery.Query != "остатки" {
		t.Fatalf("expected query остатки, got %q", registry.lastQuery.Query)
	}

	if registry.lastQuery.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", registry.lastQuery.Limit)
	}

	if !registry.lastQuery.ReadonlyOnly {
		t.Fatal("expected readonly_only=true")
	}

	if !registry.lastQuery.ExcludeJam {
		t.Fatal("expected exclude_jam=true")
	}
}

func TestHandleRegistrySearchFallsBackWithoutDiagnostics(t *testing.T) {
	registry := &plainRegistryForHandlerTest{
		operations: []entities.WBRegistryOperation{
			registrySearchHandlerTestOperation("generated_post_api_v3_stocks_warehouseid"),
		},
	}

	handler := NewHandler(Config{
		PublicBaseURL: "http://example.test",
		Logger:        log.New(io.Discard, "", 0),
	}, &plannerForRegistrySearchHandlerTest{}, registry)

	request := httptest.NewRequest(http.MethodGet, "/debug/registry/search?q=остатки&limit=5", nil)
	recorder := httptest.NewRecorder()

	handler.HandleRegistrySearch(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Operations []entities.WBRegistryOperation `json:"operations"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}

	if len(response.Operations) != 1 {
		t.Fatalf("expected one operation, got %#v", response.Operations)
	}

	if response.Operations[0].OperationID != "generated_post_api_v3_stocks_warehouseid" {
		t.Fatalf("unexpected operation_id %q", response.Operations[0].OperationID)
	}
}

type plannerForRegistrySearchHandlerTest struct{}

func (p *plannerForRegistrySearchHandlerTest) Plan(
	ctx context.Context,
	request entities.BusinessRequest,
) (*entities.ApiExecutionPlan, error) {
	return entities.NewBlockedPlan(request, "not_used", []entities.PlanWarning{}), nil
}

type diagnosticRegistryForHandlerTest struct {
	result    wbregistry.SearchResult
	lastQuery wbregistry.SearchQuery
}

func (r *diagnosticRegistryForHandlerTest) SearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	r.lastQuery = query
	return r.result.Operations, nil
}

func (r *diagnosticRegistryForHandlerTest) SearchOperationsWithDiagnostics(
	ctx context.Context,
	query wbregistry.SearchQuery,
) (wbregistry.SearchResult, error) {
	r.lastQuery = query
	return r.result, nil
}

func (r *diagnosticRegistryForHandlerTest) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	for _, operation := range r.result.Operations {
		if operation.OperationID == operationID {
			return &operation, nil
		}
	}

	return nil, nil
}

func (r *diagnosticRegistryForHandlerTest) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}

type plainRegistryForHandlerTest struct {
	operations []entities.WBRegistryOperation
}

func (r *plainRegistryForHandlerTest) SearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return r.operations, nil
}

func (r *plainRegistryForHandlerTest) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	for _, operation := range r.operations {
		if operation.OperationID == operationID {
			return &operation, nil
		}
	}

	return nil, nil
}

func (r *plainRegistryForHandlerTest) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}

func registrySearchHandlerTestOperation(operationID string) entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "test.yaml",
		OperationID:              operationID,
		Method:                   "GET",
		ServerURL:                "https://example.test",
		PathTemplate:             "/api/v1/test",
		Tags:                     []string{"test"},
		Category:                 "test",
		Summary:                  "Test operation",
		Description:              "Test operation.",
		XReadonlyMethod:          &readonly,
		XCategory:                "test",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        "{}",
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}
