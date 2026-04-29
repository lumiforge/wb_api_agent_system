package sqlite

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWBRegistryEmbeddingStoreUpsertAndGet(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	embedding := wbregistry.OperationEmbedding{
		OperationID: "generated_get_api_v1_supplier_sales",
		SourceFile:  "reports.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-1",
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	if err := store.UpsertOperationEmbedding(context.Background(), embedding); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	found, err := store.GetOperationEmbedding(
		context.Background(),
		"generated_get_api_v1_supplier_sales",
		"text-embedding-3-small",
		3,
	)
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}

	if found == nil {
		t.Fatal("expected embedding, got nil")
	}

	assertOperationEmbedding(t, *found, embedding)
}

func TestWBRegistryEmbeddingStoreUpsertReplacesExistingEmbedding(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	initial := wbregistry.OperationEmbedding{
		OperationID: "generated_get_api_v1_supplier_sales",
		SourceFile:  "reports.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-1",
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	updated := wbregistry.OperationEmbedding{
		OperationID: "generated_get_api_v1_supplier_sales",
		SourceFile:  "reports.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-2",
		Vector:      []float64{0.4, 0.5, 0.6},
	}

	if err := store.UpsertOperationEmbedding(context.Background(), initial); err != nil {
		t.Fatalf("upsert initial embedding: %v", err)
	}

	if err := store.UpsertOperationEmbedding(context.Background(), updated); err != nil {
		t.Fatalf("upsert updated embedding: %v", err)
	}

	found, err := store.GetOperationEmbedding(
		context.Background(),
		"generated_get_api_v1_supplier_sales",
		"text-embedding-3-small",
		3,
	)
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}

	if found == nil {
		t.Fatal("expected embedding, got nil")
	}

	assertOperationEmbedding(t, *found, updated)
}

func TestWBRegistryEmbeddingStoreSeparatesModelAndDimensions(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	small3 := wbregistry.OperationEmbedding{
		OperationID: "operation_cards",
		SourceFile:  "products.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-small-3",
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	small4 := wbregistry.OperationEmbedding{
		OperationID: "operation_cards",
		SourceFile:  "products.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  4,
		ContentHash: "hash-small-4",
		Vector:      []float64{0.1, 0.2, 0.3, 0.4},
	}

	large3 := wbregistry.OperationEmbedding{
		OperationID: "operation_cards",
		SourceFile:  "products.yaml",
		Model:       "text-embedding-3-large",
		Dimensions:  3,
		ContentHash: "hash-large-3",
		Vector:      []float64{0.7, 0.8, 0.9},
	}

	for _, embedding := range []wbregistry.OperationEmbedding{small3, small4, large3} {
		if err := store.UpsertOperationEmbedding(context.Background(), embedding); err != nil {
			t.Fatalf("upsert embedding: %v", err)
		}
	}

	foundSmall3, err := store.GetOperationEmbedding(context.Background(), "operation_cards", "text-embedding-3-small", 3)
	if err != nil {
		t.Fatalf("get small3: %v", err)
	}
	if foundSmall3 == nil {
		t.Fatal("expected small3 embedding, got nil")
	}
	assertOperationEmbedding(t, *foundSmall3, small3)

	foundSmall4, err := store.GetOperationEmbedding(context.Background(), "operation_cards", "text-embedding-3-small", 4)
	if err != nil {
		t.Fatalf("get small4: %v", err)
	}
	if foundSmall4 == nil {
		t.Fatal("expected small4 embedding, got nil")
	}
	assertOperationEmbedding(t, *foundSmall4, small4)

	foundLarge3, err := store.GetOperationEmbedding(context.Background(), "operation_cards", "text-embedding-3-large", 3)
	if err != nil {
		t.Fatalf("get large3: %v", err)
	}
	if foundLarge3 == nil {
		t.Fatal("expected large3 embedding, got nil")
	}
	assertOperationEmbedding(t, *foundLarge3, large3)
}

func TestWBRegistryEmbeddingStoreReturnsNilWhenMissing(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	found, err := store.GetOperationEmbedding(
		context.Background(),
		"missing_operation",
		"text-embedding-3-small",
		3,
	)
	if err != nil {
		t.Fatalf("get missing embedding: %v", err)
	}

	if found != nil {
		t.Fatalf("expected nil, got %#v", found)
	}
}

func TestWBRegistryEmbeddingStoreRejectsInvalidEmbedding(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	tests := []struct {
		name      string
		embedding wbregistry.OperationEmbedding
	}{
		{
			name: "empty operation id",
			embedding: wbregistry.OperationEmbedding{
				Model:       "text-embedding-3-small",
				Dimensions:  3,
				ContentHash: "hash",
				Vector:      []float64{0.1, 0.2, 0.3},
			},
		},
		{
			name: "empty model",
			embedding: wbregistry.OperationEmbedding{
				OperationID: "operation_cards",
				Dimensions:  3,
				ContentHash: "hash",
				Vector:      []float64{0.1, 0.2, 0.3},
			},
		},
		{
			name: "non-positive dimensions",
			embedding: wbregistry.OperationEmbedding{
				OperationID: "operation_cards",
				Model:       "text-embedding-3-small",
				Dimensions:  0,
				ContentHash: "hash",
				Vector:      []float64{},
			},
		},
		{
			name: "vector length mismatch",
			embedding: wbregistry.OperationEmbedding{
				OperationID: "operation_cards",
				Model:       "text-embedding-3-small",
				Dimensions:  3,
				ContentHash: "hash",
				Vector:      []float64{0.1, 0.2},
			},
		},
		{
			name: "empty content hash",
			embedding: wbregistry.OperationEmbedding{
				OperationID: "operation_cards",
				Model:       "text-embedding-3-small",
				Dimensions:  3,
				Vector:      []float64{0.1, 0.2, 0.3},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := store.UpsertOperationEmbedding(context.Background(), test.embedding)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func newTestWBRegistryEmbeddingStore(t *testing.T) *WBRegistryEmbeddingStore {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := AutoMigrateWBRegistryEmbeddingStore(db); err != nil {
		t.Fatalf("auto migrate embedding store: %v", err)
	}

	return NewWBRegistryEmbeddingStore(db)
}

func assertOperationEmbedding(
	t *testing.T,
	actual wbregistry.OperationEmbedding,
	expected wbregistry.OperationEmbedding,
) {
	t.Helper()

	if actual.OperationID != expected.OperationID {
		t.Fatalf("expected operation_id %q, got %q", expected.OperationID, actual.OperationID)
	}

	if actual.SourceFile != expected.SourceFile {
		t.Fatalf("expected source_file %q, got %q", expected.SourceFile, actual.SourceFile)
	}

	if actual.Model != expected.Model {
		t.Fatalf("expected model %q, got %q", expected.Model, actual.Model)
	}

	if actual.Dimensions != expected.Dimensions {
		t.Fatalf("expected dimensions %d, got %d", expected.Dimensions, actual.Dimensions)
	}

	if actual.ContentHash != expected.ContentHash {
		t.Fatalf("expected content_hash %q, got %q", expected.ContentHash, actual.ContentHash)
	}

	if len(actual.Vector) != len(expected.Vector) {
		t.Fatalf("expected vector len %d, got %d", len(expected.Vector), len(actual.Vector))
	}

	for index := range expected.Vector {
		if actual.Vector[index] != expected.Vector[index] {
			t.Fatalf("expected vector[%d]=%v, got %v", index, expected.Vector[index], actual.Vector[index])
		}
	}
}
func TestWBRegistryEmbeddingStoreListOperationEmbeddings(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	sales := wbregistry.OperationEmbedding{
		OperationID: "generated_get_api_v1_supplier_sales",
		SourceFile:  "reports.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-sales",
		Vector:      []float64{0.1, 0.2, 0.3},
	}

	stocks := wbregistry.OperationEmbedding{
		OperationID: "generated_post_api_v3_stocks_warehouseid",
		SourceFile:  "marketplace.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  3,
		ContentHash: "hash-stocks",
		Vector:      []float64{0.4, 0.5, 0.6},
	}

	differentDimensions := wbregistry.OperationEmbedding{
		OperationID: "operation_cards",
		SourceFile:  "products.yaml",
		Model:       "text-embedding-3-small",
		Dimensions:  4,
		ContentHash: "hash-cards",
		Vector:      []float64{0.7, 0.8, 0.9, 1.0},
	}

	for _, embedding := range []wbregistry.OperationEmbedding{sales, stocks, differentDimensions} {
		if err := store.UpsertOperationEmbedding(context.Background(), embedding); err != nil {
			t.Fatalf("upsert embedding: %v", err)
		}
	}

	found, err := store.ListOperationEmbeddings(context.Background(), "text-embedding-3-small", 3)
	if err != nil {
		t.Fatalf("list operation embeddings: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("expected 2 embeddings, got %#v", found)
	}

	assertOperationEmbedding(t, found[0], stocks)
	assertOperationEmbedding(t, found[1], sales)
}

func TestWBRegistryEmbeddingStoreStatsOperationEmbeddings(t *testing.T) {
	store := newTestWBRegistryEmbeddingStore(t)

	embeddings := []wbregistry.OperationEmbedding{
		{
			OperationID: "generated_get_api_v1_supplier_sales",
			SourceFile:  "reports.yaml",
			Model:       "text-embedding-3-small",
			Dimensions:  3,
			ContentHash: "hash-sales",
			Vector:      []float64{0.1, 0.2, 0.3},
		},
		{
			OperationID: "generated_post_api_v3_stocks_warehouseid",
			SourceFile:  "marketplace.yaml",
			Model:       "text-embedding-3-small",
			Dimensions:  3,
			ContentHash: "hash-stocks",
			Vector:      []float64{0.4, 0.5, 0.6},
		},
		{
			OperationID: "operation_cards",
			SourceFile:  "products.yaml",
			Model:       "text-embedding-3-small",
			Dimensions:  4,
			ContentHash: "hash-cards",
			Vector:      []float64{0.7, 0.8, 0.9, 1.0},
		},
	}

	for _, embedding := range embeddings {
		if err := store.UpsertOperationEmbedding(context.Background(), embedding); err != nil {
			t.Fatalf("upsert embedding: %v", err)
		}
	}

	stats, err := store.StatsOperationEmbeddings(context.Background(), "text-embedding-3-small", 3)
	if err != nil {
		t.Fatalf("stats operation embeddings: %v", err)
	}

	if stats.Total != 2 {
		t.Fatalf("expected total=2, got %d", stats.Total)
	}
}
