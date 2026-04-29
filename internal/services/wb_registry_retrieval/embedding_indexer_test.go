package wb_registry_retrieval

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestEmbeddingIndexerRebuildEmbedsAndStoresMissingOperations(t *testing.T) {
	sourceStore := &fakeEmbeddingSourceStore{
		operations: []entities.WBRegistryOperation{
			testEmbeddingDocumentOperation(),
			testEmbeddingIndexerOperation("generated_get_api_v1_supplier_orders", "/api/v1/supplier/orders", "Заказы"),
		},
	}

	embeddingStore := newFakeEmbeddingStore()
	embeddingClient := &fakeEmbeddingClient{
		vector: []float64{0.1, 0.2, 0.3},
	}

	indexer := mustNewTestEmbeddingIndexer(t, sourceStore, embeddingStore, embeddingClient)

	result, err := indexer.Rebuild(context.Background())
	if err != nil {
		t.Fatalf("rebuild embeddings: %v", err)
	}

	if result.OperationsScanned != 2 {
		t.Fatalf("expected 2 scanned operations, got %d", result.OperationsScanned)
	}

	if result.EmbeddingsCreated != 2 {
		t.Fatalf("expected 2 created embeddings, got %d", result.EmbeddingsCreated)
	}

	if result.EmbeddingsSkipped != 0 {
		t.Fatalf("expected 0 skipped embeddings, got %d", result.EmbeddingsSkipped)
	}

	if embeddingClient.calls != 1 {
		t.Fatalf("expected one embedding client call, got %d", embeddingClient.calls)
	}

	if len(embeddingStore.items) != 2 {
		t.Fatalf("expected 2 stored embeddings, got %#v", embeddingStore.items)
	}
}

func TestEmbeddingIndexerRebuildSkipsUnchangedEmbeddings(t *testing.T) {
	operation := testEmbeddingDocumentOperation()
	document := NewEmbeddingDocumentBuilder().BuildOperationDocument(operation)

	sourceStore := &fakeEmbeddingSourceStore{
		operations: []entities.WBRegistryOperation{operation},
	}

	embeddingStore := newFakeEmbeddingStore()
	embeddingStore.items[embeddingKey(document.OperationID, "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: document.OperationID,
		SourceFile:  document.SourceFile,
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: document.ContentHash,
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	embeddingClient := &fakeEmbeddingClient{
		vector: []float64{0.4, 0.5, 0.6},
	}

	indexer := mustNewTestEmbeddingIndexer(t, sourceStore, embeddingStore, embeddingClient)

	result, err := indexer.Rebuild(context.Background())
	if err != nil {
		t.Fatalf("rebuild embeddings: %v", err)
	}

	if result.OperationsScanned != 1 {
		t.Fatalf("expected 1 scanned operation, got %d", result.OperationsScanned)
	}

	if result.EmbeddingsCreated != 0 {
		t.Fatalf("expected 0 created embeddings, got %d", result.EmbeddingsCreated)
	}

	if result.EmbeddingsSkipped != 1 {
		t.Fatalf("expected 1 skipped embedding, got %d", result.EmbeddingsSkipped)
	}

	if embeddingClient.calls != 0 {
		t.Fatalf("expected no embedding client calls, got %d", embeddingClient.calls)
	}
}

func TestEmbeddingIndexerRebuildReembedsChangedContent(t *testing.T) {
	operation := testEmbeddingDocumentOperation()
	oldDocument := NewEmbeddingDocumentBuilder().BuildOperationDocument(operation)

	operation.Summary = "Продажи и возвраты"

	sourceStore := &fakeEmbeddingSourceStore{
		operations: []entities.WBRegistryOperation{operation},
	}

	embeddingStore := newFakeEmbeddingStore()
	embeddingStore.items[embeddingKey(operation.OperationID, "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: oldDocument.OperationID,
		SourceFile:  oldDocument.SourceFile,
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: oldDocument.ContentHash,
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	embeddingClient := &fakeEmbeddingClient{
		vector: []float64{0.7, 0.8, 0.9},
	}

	indexer := mustNewTestEmbeddingIndexer(t, sourceStore, embeddingStore, embeddingClient)

	result, err := indexer.Rebuild(context.Background())
	if err != nil {
		t.Fatalf("rebuild embeddings: %v", err)
	}

	if result.EmbeddingsCreated != 1 {
		t.Fatalf("expected 1 created embedding, got %d", result.EmbeddingsCreated)
	}

	if result.EmbeddingsSkipped != 0 {
		t.Fatalf("expected 0 skipped embeddings, got %d", result.EmbeddingsSkipped)
	}

	stored := embeddingStore.items[embeddingKey(operation.OperationID, "text-embedding-3-small", 3)]
	if stored.ContentHash == oldDocument.ContentHash {
		t.Fatal("expected stored content hash to be updated")
	}

	assertFloatVector(t, stored.Vector, []float64{0.7, 0.8, 0.9})
}

func TestNewEmbeddingIndexerRejectsInvalidConfig(t *testing.T) {
	embeddingStore := newFakeEmbeddingStore()
	sourceStore := &fakeEmbeddingSourceStore{}
	embeddingClient := &fakeEmbeddingClient{}

	tests := []struct {
		name string
		cfg  EmbeddingIndexerConfig
	}{
		{
			name: "missing source store",
			cfg: EmbeddingIndexerConfig{
				EmbeddingStore:  embeddingStore,
				EmbeddingClient: embeddingClient,
				Model:           "text-embedding-3-small",
				Dimensions:      3,
			},
		},
		{
			name: "missing embedding store",
			cfg: EmbeddingIndexerConfig{
				SourceStore:     sourceStore,
				EmbeddingClient: embeddingClient,
				Model:           "text-embedding-3-small",
				Dimensions:      3,
			},
		},
		{
			name: "missing embedding client",
			cfg: EmbeddingIndexerConfig{
				SourceStore:    sourceStore,
				EmbeddingStore: embeddingStore,
				Model:          "text-embedding-3-small",
				Dimensions:     3,
			},
		},
		{
			name: "missing model",
			cfg: EmbeddingIndexerConfig{
				SourceStore:     sourceStore,
				EmbeddingStore:  embeddingStore,
				EmbeddingClient: embeddingClient,
				Dimensions:      3,
			},
		},
		{
			name: "invalid dimensions",
			cfg: EmbeddingIndexerConfig{
				SourceStore:     sourceStore,
				EmbeddingStore:  embeddingStore,
				EmbeddingClient: embeddingClient,
				Model:           "text-embedding-3-small",
				Dimensions:      0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewEmbeddingIndexer(test.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

type fakeEmbeddingSourceStore struct {
	operations []entities.WBRegistryOperation
}

func (s *fakeEmbeddingSourceStore) ListOperations(ctx context.Context) ([]entities.WBRegistryOperation, error) {
	return s.operations, nil
}

func (s *fakeEmbeddingSourceStore) RawSearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return s.operations, nil
}

func (s *fakeEmbeddingSourceStore) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	for _, operation := range s.operations {
		if operation.OperationID == operationID {
			return &operation, nil
		}
	}

	return nil, nil
}

func (s *fakeEmbeddingSourceStore) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{
		Total: int64(len(s.operations)),
	}, nil
}

type fakeEmbeddingStore struct {
	items map[string]wbregistry.OperationEmbedding
}

func newFakeEmbeddingStore() *fakeEmbeddingStore {
	return &fakeEmbeddingStore{
		items: map[string]wbregistry.OperationEmbedding{},
	}
}

func (s *fakeEmbeddingStore) UpsertOperationEmbedding(
	ctx context.Context,
	embedding wbregistry.OperationEmbedding,
) error {
	s.items[embeddingKey(embedding.OperationID, embedding.Model, embedding.Dimensions)] = embedding
	return nil
}

func (s *fakeEmbeddingStore) GetOperationEmbedding(
	ctx context.Context,
	operationID string,
	model string,
	dimensions int,
) (*wbregistry.OperationEmbedding, error) {
	embedding, ok := s.items[embeddingKey(operationID, model, dimensions)]
	if !ok {
		return nil, nil
	}

	return &embedding, nil
}

type fakeEmbeddingClient struct {
	vector []float64
	calls  int
}

func (c *fakeEmbeddingClient) EmbedTexts(
	ctx context.Context,
	input wbregistry.EmbeddingRequest,
) (wbregistry.EmbeddingResponse, error) {
	c.calls++

	vectors := make([][]float64, 0, len(input.Texts))
	for range input.Texts {
		vector := append([]float64{}, c.vector...)
		vectors = append(vectors, vector)
	}

	return wbregistry.EmbeddingResponse{
		Model:      input.Model,
		Dimensions: input.Dimensions,
		Vectors:    vectors,
	}, nil
}

func mustNewTestEmbeddingIndexer(
	t *testing.T,
	sourceStore wbregistry.RawOperationStore,
	embeddingStore wbregistry.OperationEmbeddingStore,
	embeddingClient wbregistry.EmbeddingClient,
) *EmbeddingIndexer {
	t.Helper()

	indexer, err := NewEmbeddingIndexer(EmbeddingIndexerConfig{
		SourceStore:     sourceStore,
		EmbeddingStore:  embeddingStore,
		EmbeddingClient: embeddingClient,
		Model:           "text-embedding-3-small",
		Dimensions:      3,
		BatchSize:       2,
	})
	if err != nil {
		t.Fatalf("new embedding indexer: %v", err)
	}

	return indexer
}

func testEmbeddingIndexerOperation(
	operationID string,
	pathTemplate string,
	summary string,
) entities.WBRegistryOperation {
	operation := testEmbeddingDocumentOperation()
	operation.OperationID = operationID
	operation.PathTemplate = pathTemplate
	operation.Summary = summary

	return operation
}

func embeddingKey(operationID string, model string, dimensions int) string {
	return operationID + "|" + model + "|" + string(rune(dimensions))
}

func assertFloatVector(t *testing.T, actual []float64, expected []float64) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("expected vector len %d, got %d", len(expected), len(actual))
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("expected vector[%d]=%v, got %v", index, expected[index], actual[index])
		}
	}
}

func (s *fakeEmbeddingStore) StatsOperationEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
) (wbregistry.OperationEmbeddingStats, error) {
	var total int64

	for _, embedding := range s.items {
		if embedding.Model == model && embedding.Dimensions == dimensions {
			total++
		}
	}

	return wbregistry.OperationEmbeddingStats{
		Total: total,
	}, nil
}

func (s *fakeEmbeddingStore) ListOperationEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
) ([]wbregistry.OperationEmbedding, error) {
	result := make([]wbregistry.OperationEmbedding, 0)

	for _, embedding := range s.items {
		if embedding.Model == model && embedding.Dimensions == dimensions {
			result = append(result, embedding)
		}
	}

	return result, nil
}
