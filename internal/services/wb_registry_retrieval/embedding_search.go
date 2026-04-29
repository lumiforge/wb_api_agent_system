package wb_registry_retrieval

import (
	"fmt"
	"math"
	"sort"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Scores persisted operation embeddings against a query vector without owning embedding storage.
type EmbeddingSearcher struct{}

type EmbeddingSearchResult struct {
	OperationID string
	Score       float64
}

func NewEmbeddingSearcher() *EmbeddingSearcher {
	return &EmbeddingSearcher{}
}

func (s *EmbeddingSearcher) Search(
	queryVector []float64,
	embeddings []wbregistry.OperationEmbedding,
	limit int,
) ([]EmbeddingSearchResult, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector must not be empty")
	}
	if limit <= 0 {
		return []EmbeddingSearchResult{}, nil
	}

	results := make([]EmbeddingSearchResult, 0, len(embeddings))

	for _, embedding := range embeddings {
		if len(embedding.Vector) != len(queryVector) {
			return nil, fmt.Errorf("embedding dimension mismatch for operation %s: query=%d embedding=%d", embedding.OperationID, len(queryVector), len(embedding.Vector))
		}

		score, err := cosineSimilarity(queryVector, embedding.Vector)
		if err != nil {
			return nil, fmt.Errorf("score operation embedding %s: %w", embedding.OperationID, err)
		}

		results = append(results, EmbeddingSearchResult{
			OperationID: embedding.OperationID,
			Score:       score,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].OperationID < results[j].OperationID
		}

		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		return results[:limit], nil
	}

	return results, nil
}

func cosineSimilarity(left []float64, right []float64) (float64, error) {
	if len(left) != len(right) {
		return 0, fmt.Errorf("vector length mismatch: left=%d right=%d", len(left), len(right))
	}

	var dotProduct float64
	var leftNorm float64
	var rightNorm float64

	for index := range left {
		dotProduct += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}

	if leftNorm == 0 || rightNorm == 0 {
		return 0, fmt.Errorf("zero vector cannot be scored")
	}

	return dotProduct / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm)), nil
}
