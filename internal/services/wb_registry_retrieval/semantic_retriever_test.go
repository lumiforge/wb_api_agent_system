package wb_registry_retrieval

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestSemanticOperationRetrieverSearchReturnsRegistryOperationsByEmbeddingScore(t *testing.T) {
	readonly := true

	stocks := testSemanticRetrieverOperation(
		"generated_post_api_v3_stocks_warehouseid",
		"marketplace.yaml",
		"/api/v3/stocks/{warehouseId}",
		"Остатки",
		&readonly,
		false,
	)

	sales := testSemanticRetrieverOperation(
		"generated_get_api_v1_supplier_sales",
		"reports.yaml",
		"/api/v1/supplier/sales",
		"Продажи",
		&readonly,
		false,
	)

	sourceStore := &fakeSemanticSourceStore{
		operations: map[string]entities.WBRegistryOperation{
			stocks.OperationID: stocks,
			sales.OperationID:  sales,
		},
	}

	embeddingStore := newFakeEmbeddingStore()
	embeddingStore.items[embeddingKey(stocks.OperationID, "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: stocks.OperationID,
		SourceFile:  stocks.SourceFile,
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-stocks",
		Vector:      []float64{1, 0, 0},
	}
	embeddingStore.items[embeddingKey(sales.OperationID, "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
		OperationID: sales.OperationID,
		SourceFile:  sales.SourceFile,
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-sales",
		Vector:      []float64{0, 1, 0},
	}

	embeddingClient := &fakeEmbeddingClient{
		vector: []float64{1, 0, 0},
	}

	retriever := mustNewTestSemanticOperationRetriever(t, sourceStore, embeddingStore, embeddingClient)

	results, err := retriever.Search(context.Background(), wbregistry.SearchQuery{
		Query:        "остатки товаров",
		Limit:        2,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	assertSemanticOperationIDs(t, results, []string{
		"generated_post_api_v3_stocks_warehouseid",
		"generated_get_api_v1_supplier_sales",
	})

	if embeddingClient.calls != 1 {
		t.Fatalf("expected one embedding client call, got %d", embeddingClient.calls)
	}
}

func TestSemanticOperationRetrieverSearchAppliesPolicyFilters(t *testing.T) {
	readonly := true
	write := false

	readOperation := testSemanticRetrieverOperation(
		"read_operation",
		"read.yaml",
		"/api/v1/read",
		"Read",
		&readonly,
		false,
	)

	writeOperation := testSemanticRetrieverOperation(
		"write_operation",
		"write.yaml",
		"/api/v1/write",
		"Write",
		&write,
		false,
	)

	jamOperation := testSemanticRetrieverOperation(
		"jam_operation",
		"jam.yaml",
		"/api/v1/jam",
		"Jam",
		&readonly,
		true,
	)

	sourceStore := &fakeSemanticSourceStore{
		operations: map[string]entities.WBRegistryOperation{
			readOperation.OperationID:  readOperation,
			writeOperation.OperationID: writeOperation,
			jamOperation.OperationID:   jamOperation,
		},
	}

	embeddingStore := newFakeEmbeddingStore()
	for _, operation := range []entities.WBRegistryOperation{readOperation, writeOperation, jamOperation} {
		embeddingStore.items[embeddingKey(operation.OperationID, "text-embedding-3-small", 3)] = wbregistry.OperationEmbedding{
			OperationID: operation.OperationID,
			SourceFile:  operation.SourceFile,
			Model:       "text-embedding-3-small",
			Dimensions:  3,
			ContentHash: "hash-" + operation.OperationID,
			Vector:      []float64{1, 0, 0},
		}
	}

	embeddingClient := &fakeEmbeddingClient{
		vector: []float64{1, 0, 0},
	}

	retriever := mustNewTestSemanticOperationRetriever(t, sourceStore, embeddingStore, embeddingClient)

	results, err := retriever.Search(context.Background(), wbregistry.SearchQuery{
		Query:        "anything",
		Limit:        10,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	assertSemanticOperationIDs(t, results, []string{"read_operation"})
}

func TestSemanticOperationRetrieverSearchReturnsEmptyForBlankQuery(t *testing.T) {
	retriever := mustNewTestSemanticOperationRetriever(
		t,
		&fakeSemanticSourceStore{},
		newFakeEmbeddingStore(),
		&fakeEmbeddingClient{vector: []float64{1, 0, 0}},
	)

	results, err := retriever.Search(context.Background(), wbregistry.SearchQuery{
		Query: " ",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected empty results, got %#v", results)
	}
}

func TestNewSemanticOperationRetrieverRejectsInvalidConfig(t *testing.T) {
	sourceStore := &fakeSemanticSourceStore{}
	embeddingStore := newFakeEmbeddingStore()
	embeddingClient := &fakeEmbeddingClient{vector: []float64{1, 0, 0}}

	tests := []struct {
		name string
		cfg  SemanticOperationRetrieverConfig
	}{
		{
			name: "missing source store",
			cfg: SemanticOperationRetrieverConfig{
				EmbeddingStore:  embeddingStore,
				EmbeddingClient: embeddingClient,
				Model:           "text-embedding-3-small",
				Dimensions:      3,
			},
		},
		{
			name: "missing embedding store",
			cfg: SemanticOperationRetrieverConfig{
				SourceStore:     sourceStore,
				EmbeddingClient: embeddingClient,
				Model:           "text-embedding-3-small",
				Dimensions:      3,
			},
		},
		{
			name: "missing embedding client",
			cfg: SemanticOperationRetrieverConfig{
				SourceStore:    sourceStore,
				EmbeddingStore: embeddingStore,
				Model:          "text-embedding-3-small",
				Dimensions:     3,
			},
		},
		{
			name: "missing model",
			cfg: SemanticOperationRetrieverConfig{
				SourceStore:     sourceStore,
				EmbeddingStore:  embeddingStore,
				EmbeddingClient: embeddingClient,
				Dimensions:      3,
			},
		},
		{
			name: "invalid dimensions",
			cfg: SemanticOperationRetrieverConfig{
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
			_, err := NewSemanticOperationRetriever(test.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

type fakeSemanticSourceStore struct {
	operations map[string]entities.WBRegistryOperation
}

func (s *fakeSemanticSourceStore) ListOperations(ctx context.Context) ([]entities.WBRegistryOperation, error) {
	operations := make([]entities.WBRegistryOperation, 0, len(s.operations))
	for _, operation := range s.operations {
		operations = append(operations, operation)
	}

	return operations, nil
}

func (s *fakeSemanticSourceStore) RawSearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return s.ListOperations(ctx)
}

func (s *fakeSemanticSourceStore) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	operation, ok := s.operations[operationID]
	if !ok {
		return nil, nil
	}

	return &operation, nil
}

func (s *fakeSemanticSourceStore) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{
		Total: int64(len(s.operations)),
	}, nil
}

func mustNewTestSemanticOperationRetriever(
	t *testing.T,
	sourceStore wbregistry.RawOperationStore,
	embeddingStore wbregistry.OperationEmbeddingStore,
	embeddingClient wbregistry.EmbeddingClient,
) *SemanticOperationRetriever {
	t.Helper()

	retriever, err := NewSemanticOperationRetriever(SemanticOperationRetrieverConfig{
		SourceStore:     sourceStore,
		EmbeddingStore:  embeddingStore,
		EmbeddingClient: embeddingClient,
		Model:           "text-embedding-3-small",
		Dimensions:      3,
	})
	if err != nil {
		t.Fatalf("new semantic operation retriever: %v", err)
	}

	return retriever
}

func testSemanticRetrieverOperation(
	operationID string,
	sourceFile string,
	pathTemplate string,
	summary string,
	readonly *bool,
	requiresJam bool,
) entities.WBRegistryOperation {
	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               sourceFile,
		OperationID:              operationID,
		Method:                   "GET",
		ServerURL:                "https://example.test",
		PathTemplate:             pathTemplate,
		Tags:                     []string{summary},
		Category:                 "test",
		Summary:                  summary,
		Description:              summary,
		XReadonlyMethod:          readonly,
		XCategory:                "test",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        "{}",
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              requiresJam,
	}
}

func assertSemanticOperationIDs(
	t *testing.T,
	results []SemanticOperationResult,
	expected []string,
) {
	t.Helper()

	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %#v", len(expected), results)
	}

	for index, result := range results {
		if result.Operation.OperationID != expected[index] {
			t.Fatalf("expected result[%d]=%q, got %q", index, expected[index], result.Operation.OperationID)
		}
	}
}
