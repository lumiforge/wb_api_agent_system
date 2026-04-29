package wb_registry_retrieval

import (
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestEmbeddingSearcherRanksByCosineSimilarity(t *testing.T) {
	searcher := NewEmbeddingSearcher()

	results, err := searcher.Search(
		[]float64{1, 0, 0},
		[]wbregistry.OperationEmbedding{
			{
				OperationID: "sales",
				Vector:      []float64{0, 1, 0},
			},
			{
				OperationID: "stocks",
				Vector:      []float64{1, 0, 0},
			},
			{
				OperationID: "orders",
				Vector:      []float64{0.5, 0.5, 0},
			},
		},
		2,
	)
	if err != nil {
		t.Fatalf("search embeddings: %v", err)
	}

	assertEmbeddingSearchResultIDs(t, results, []string{"stocks", "orders"})
}

func TestEmbeddingSearcherUsesOperationIDTieBreaker(t *testing.T) {
	searcher := NewEmbeddingSearcher()

	results, err := searcher.Search(
		[]float64{1, 0},
		[]wbregistry.OperationEmbedding{
			{
				OperationID: "z_operation",
				Vector:      []float64{1, 0},
			},
			{
				OperationID: "a_operation",
				Vector:      []float64{1, 0},
			},
		},
		2,
	)
	if err != nil {
		t.Fatalf("search embeddings: %v", err)
	}

	assertEmbeddingSearchResultIDs(t, results, []string{"a_operation", "z_operation"})
}

func TestEmbeddingSearcherRejectsDimensionMismatch(t *testing.T) {
	searcher := NewEmbeddingSearcher()

	_, err := searcher.Search(
		[]float64{1, 0},
		[]wbregistry.OperationEmbedding{
			{
				OperationID: "bad_operation",
				Vector:      []float64{1, 0, 0},
			},
		},
		1,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEmbeddingSearcherRejectsZeroVector(t *testing.T) {
	searcher := NewEmbeddingSearcher()

	_, err := searcher.Search(
		[]float64{1, 0},
		[]wbregistry.OperationEmbedding{
			{
				OperationID: "zero_operation",
				Vector:      []float64{0, 0},
			},
		},
		1,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func assertEmbeddingSearchResultIDs(
	t *testing.T,
	results []EmbeddingSearchResult,
	expected []string,
) {
	t.Helper()

	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %#v", len(expected), results)
	}

	for index, result := range results {
		if result.OperationID != expected[index] {
			t.Fatalf("expected result[%d]=%q, got %q", index, expected[index], result.OperationID)
		}
	}
}
