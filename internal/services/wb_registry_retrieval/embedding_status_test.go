package wb_registry_retrieval

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestEmbeddingIndexStatusServiceReportsCoverage(t *testing.T) {
	sourceStore := &fakeEmbeddingStatusSourceStore{
		stats: wbregistry.Stats{
			Total: 4,
		},
	}

	embeddingStore := newFakeEmbeddingStore()
	embeddingStore.items[embeddingKey("operation-1", "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: "operation-1",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-1",
		Vector:      []float64{0.1, 0.2, 0.3},
	}
	embeddingStore.items[embeddingKey("operation-2", "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: "operation-2",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-2",
		Vector:      []float64{0.4, 0.5, 0.6},
	}

	service, err := NewEmbeddingIndexStatusService(EmbeddingIndexStatusServiceConfig{
		SourceStore:    sourceStore,
		EmbeddingStore: embeddingStore,
		Model:          "text-embedding-3-small",
		Dimensions:     3,
	})
	if err != nil {
		t.Fatalf("new embedding index status service: %v", err)
	}

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("embedding index status: %v", err)
	}

	if status.RegistryOperations != 4 {
		t.Fatalf("expected registry_operations=4, got %d", status.RegistryOperations)
	}

	if status.IndexedEmbeddings != 2 {
		t.Fatalf("expected indexed_embeddings=2, got %d", status.IndexedEmbeddings)
	}

	if status.CoverageRatio != 0.5 {
		t.Fatalf("expected coverage_ratio=0.5, got %v", status.CoverageRatio)
	}

	if status.Model != "text-embedding-3-small" {
		t.Fatalf("expected model text-embedding-3-small, got %q", status.Model)
	}

	if status.Dimensions != 3 {
		t.Fatalf("expected dimensions=3, got %d", status.Dimensions)
	}
}

func TestEmbeddingIndexStatusServiceHandlesEmptyRegistry(t *testing.T) {
	service, err := NewEmbeddingIndexStatusService(EmbeddingIndexStatusServiceConfig{
		SourceStore: &fakeEmbeddingStatusSourceStore{
			stats: wbregistry.Stats{},
		},
		EmbeddingStore: newFakeEmbeddingStore(),
		Model:          "text-embedding-3-small",
		Dimensions:     3,
	})
	if err != nil {
		t.Fatalf("new embedding index status service: %v", err)
	}

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("embedding index status: %v", err)
	}

	if status.CoverageRatio != 0 {
		t.Fatalf("expected coverage_ratio=0, got %v", status.CoverageRatio)
	}
}

type fakeEmbeddingStatusSourceStore struct {
	stats wbregistry.Stats
}

func (s *fakeEmbeddingStatusSourceStore) ListOperations(ctx context.Context) ([]entities.WBRegistryOperation, error) {
	return nil, nil
}

func (s *fakeEmbeddingStatusSourceStore) RawSearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return nil, nil
}

func (s *fakeEmbeddingStatusSourceStore) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	return nil, nil
}

func (s *fakeEmbeddingStatusSourceStore) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return s.stats, nil
}
