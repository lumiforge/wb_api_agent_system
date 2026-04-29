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
	"github.com/lumiforge/wb_api_agent_system/internal/services/wb_registry_retrieval"
)

func TestHandleRegistryEmbeddingsStatusReturnsSafeStatus(t *testing.T) {
	statusProvider := &embeddingStatusProviderForHandlerTest{
		status: wb_registry_retrieval.EmbeddingIndexStatus{
			RegistryOperations: 10,
			IndexedEmbeddings:  7,
			CoverageRatio:      0.7,
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
		},
	}

	handler := NewHandler(Config{
		PublicBaseURL:                "http://example.test",
		Logger:                       log.New(io.Discard, "", 0),
		EmbeddingIndexStatusProvider: statusProvider,
	}, &plannerForEmbeddingStatusHandlerTest{}, &registryForEmbeddingStatusHandlerTest{})

	request := httptest.NewRequest(http.MethodGet, "/debug/registry/embeddings/status", nil)
	recorder := httptest.NewRecorder()

	handler.HandleRegistryEmbeddingsStatus(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response wb_registry_retrieval.EmbeddingIndexStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}

	if response.RegistryOperations != 10 {
		t.Fatalf("expected registry_operations=10, got %d", response.RegistryOperations)
	}

	if response.IndexedEmbeddings != 7 {
		t.Fatalf("expected indexed_embeddings=7, got %d", response.IndexedEmbeddings)
	}

	if response.CoverageRatio != 0.7 {
		t.Fatalf("expected coverage_ratio=0.7, got %v", response.CoverageRatio)
	}

	if response.Model != "text-embedding-3-small" {
		t.Fatalf("expected model text-embedding-3-small, got %q", response.Model)
	}

	if response.Dimensions != 1536 {
		t.Fatalf("expected dimensions=1536, got %d", response.Dimensions)
	}

	if statusProvider.calls != 1 {
		t.Fatalf("expected one status call, got %d", statusProvider.calls)
	}
}

func TestHandleRegistryEmbeddingsStatusReturnsNotFoundWhenNotConfigured(t *testing.T) {
	handler := NewHandler(Config{
		PublicBaseURL: "http://example.test",
		Logger:        log.New(io.Discard, "", 0),
	}, &plannerForEmbeddingStatusHandlerTest{}, &registryForEmbeddingStatusHandlerTest{})

	request := httptest.NewRequest(http.MethodGet, "/debug/registry/embeddings/status", nil)
	recorder := httptest.NewRecorder()

	handler.HandleRegistryEmbeddingsStatus(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleRegistryEmbeddingsStatusRejectsNonGET(t *testing.T) {
	handler := NewHandler(Config{
		PublicBaseURL:                "http://example.test",
		Logger:                       log.New(io.Discard, "", 0),
		EmbeddingIndexStatusProvider: &embeddingStatusProviderForHandlerTest{},
	}, &plannerForEmbeddingStatusHandlerTest{}, &registryForEmbeddingStatusHandlerTest{})

	request := httptest.NewRequest(http.MethodPost, "/debug/registry/embeddings/status", nil)
	recorder := httptest.NewRecorder()

	handler.HandleRegistryEmbeddingsStatus(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

type embeddingStatusProviderForHandlerTest struct {
	status wb_registry_retrieval.EmbeddingIndexStatus
	calls  int
}

func (p *embeddingStatusProviderForHandlerTest) Status(
	ctx context.Context,
) (wb_registry_retrieval.EmbeddingIndexStatus, error) {
	p.calls++
	return p.status, nil
}

type plannerForEmbeddingStatusHandlerTest struct{}

func (p *plannerForEmbeddingStatusHandlerTest) Plan(
	ctx context.Context,
	request entities.BusinessRequest,
) (*entities.ApiExecutionPlan, error) {
	return entities.NewBlockedPlan(request, "not_used", []entities.PlanWarning{}), nil
}

type registryForEmbeddingStatusHandlerTest struct{}

func (r *registryForEmbeddingStatusHandlerTest) SearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return nil, nil
}

func (r *registryForEmbeddingStatusHandlerTest) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	return nil, nil
}

func (r *registryForEmbeddingStatusHandlerTest) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}
