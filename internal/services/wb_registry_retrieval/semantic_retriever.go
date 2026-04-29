package wb_registry_retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Expands registry candidates through persisted operation embeddings without replacing deterministic lexical ranking.
type SemanticOperationRetriever struct {
	sourceStore     wbregistry.RawOperationStore
	embeddingStore  wbregistry.OperationEmbeddingStore
	embeddingClient wbregistry.EmbeddingClient
	searcher        *EmbeddingSearcher
	model           string
	dimensions      int
}

type SemanticOperationRetrieverConfig struct {
	SourceStore     wbregistry.RawOperationStore
	EmbeddingStore  wbregistry.OperationEmbeddingStore
	EmbeddingClient wbregistry.EmbeddingClient
	Model           string
	Dimensions      int
}

type SemanticOperationResult struct {
	Operation entities.WBRegistryOperation
	Score     float64
}

func NewSemanticOperationRetriever(cfg SemanticOperationRetrieverConfig) (*SemanticOperationRetriever, error) {
	if cfg.SourceStore == nil {
		return nil, fmt.Errorf("source store is required")
	}
	if cfg.EmbeddingStore == nil {
		return nil, fmt.Errorf("embedding store is required")
	}
	if cfg.EmbeddingClient == nil {
		return nil, fmt.Errorf("embedding client is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	if cfg.Dimensions <= 0 {
		return nil, fmt.Errorf("embedding dimensions must be positive")
	}

	return &SemanticOperationRetriever{
		sourceStore:     cfg.SourceStore,
		embeddingStore:  cfg.EmbeddingStore,
		embeddingClient: cfg.EmbeddingClient,
		searcher:        NewEmbeddingSearcher(),
		model:           cfg.Model,
		dimensions:      cfg.Dimensions,
	}, nil
}

func (r *SemanticOperationRetriever) Search(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]SemanticOperationResult, error) {
	limit := normalizedSearchLimit(query.Limit)
	queryText := strings.TrimSpace(query.Query)
	if queryText == "" {
		return []SemanticOperationResult{}, nil
	}

	queryEmbedding, err := r.embeddingClient.EmbedTexts(ctx, wbregistry.EmbeddingRequest{
		Model:      r.model,
		Dimensions: r.dimensions,
		Texts:      []string{queryText},
	})
	if err != nil {
		return nil, fmt.Errorf("embed semantic registry query: %w", err)
	}

	if len(queryEmbedding.Vectors) != 1 {
		return nil, fmt.Errorf("semantic query embedding response count mismatch: expected=1 got=%d", len(queryEmbedding.Vectors))
	}

	operationEmbeddings, err := r.embeddingStore.ListOperationEmbeddings(ctx, r.model, r.dimensions)
	if err != nil {
		return nil, fmt.Errorf("list operation embeddings: %w", err)
	}

	vectorResults, err := r.searcher.Search(queryEmbedding.Vectors[0], operationEmbeddings, limit)
	if err != nil {
		return nil, fmt.Errorf("search operation embeddings: %w", err)
	}

	results := make([]SemanticOperationResult, 0, len(vectorResults))
	for _, vectorResult := range vectorResults {
		operation, err := r.sourceStore.GetOperation(ctx, vectorResult.OperationID)
		if err != nil {
			return nil, fmt.Errorf("get semantic operation candidate %s: %w", vectorResult.OperationID, err)
		}

		if operation == nil {
			continue
		}

		if query.ReadonlyOnly && (operation.XReadonlyMethod == nil || !*operation.XReadonlyMethod) {
			continue
		}

		if query.ExcludeJam && operation.RequiresJam {
			continue
		}

		results = append(results, SemanticOperationResult{
			Operation: *operation,
			Score:     vectorResult.Score,
		})
	}

	return results, nil
}
