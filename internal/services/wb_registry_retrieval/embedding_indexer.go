package wb_registry_retrieval

import (
	"context"
	"fmt"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Builds and stores registry operation embeddings from source-of-record registry data.
type EmbeddingIndexer struct {
	sourceStore     wbregistry.RawOperationStore
	embeddingStore  wbregistry.OperationEmbeddingStore
	embeddingClient wbregistry.EmbeddingClient
	documentBuilder *EmbeddingDocumentBuilder
	model           string
	dimensions      int
	batchSize       int
}

type EmbeddingIndexerConfig struct {
	SourceStore     wbregistry.RawOperationStore
	EmbeddingStore  wbregistry.OperationEmbeddingStore
	EmbeddingClient wbregistry.EmbeddingClient
	Model           string
	Dimensions      int
	BatchSize       int
}

type EmbeddingIndexResult struct {
	OperationsScanned int
	EmbeddingsCreated int
	EmbeddingsSkipped int
}

func NewEmbeddingIndexer(cfg EmbeddingIndexerConfig) (*EmbeddingIndexer, error) {
	if cfg.SourceStore == nil {
		return nil, fmt.Errorf("source store is required")
	}
	if cfg.EmbeddingStore == nil {
		return nil, fmt.Errorf("embedding store is required")
	}
	if cfg.EmbeddingClient == nil {
		return nil, fmt.Errorf("embedding client is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	if cfg.Dimensions <= 0 {
		return nil, fmt.Errorf("embedding dimensions must be positive")
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}

	return &EmbeddingIndexer{
		sourceStore:     cfg.SourceStore,
		embeddingStore:  cfg.EmbeddingStore,
		embeddingClient: cfg.EmbeddingClient,
		documentBuilder: NewEmbeddingDocumentBuilder(),
		model:           cfg.Model,
		dimensions:      cfg.Dimensions,
		batchSize:       batchSize,
	}, nil
}

func (i *EmbeddingIndexer) Rebuild(ctx context.Context) (EmbeddingIndexResult, error) {
	operations, err := i.sourceStore.ListOperations(ctx)
	if err != nil {
		return EmbeddingIndexResult{}, fmt.Errorf("list registry operations: %w", err)
	}

	result := EmbeddingIndexResult{
		OperationsScanned: len(operations),
	}

	pending := make([]OperationEmbeddingDocument, 0, i.batchSize)

	for _, operation := range operations {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("embedding index context: %w", err)
		}

		document := i.documentBuilder.BuildOperationDocument(operation)

		existing, err := i.embeddingStore.GetOperationEmbedding(ctx, document.OperationID, i.model, i.dimensions)
		if err != nil {
			return result, fmt.Errorf("get existing operation embedding %s: %w", document.OperationID, err)
		}

		if existing != nil && existing.ContentHash == document.ContentHash {
			result.EmbeddingsSkipped++
			continue
		}

		pending = append(pending, document)

		if len(pending) >= i.batchSize {
			created, err := i.embedAndStoreDocuments(ctx, pending)
			if err != nil {
				return result, err
			}

			result.EmbeddingsCreated += created
			pending = pending[:0]
		}
	}

	if len(pending) > 0 {
		created, err := i.embedAndStoreDocuments(ctx, pending)
		if err != nil {
			return result, err
		}

		result.EmbeddingsCreated += created
	}

	return result, nil
}

func (i *EmbeddingIndexer) embedAndStoreDocuments(
	ctx context.Context,
	documents []OperationEmbeddingDocument,
) (int, error) {
	texts := make([]string, 0, len(documents))
	for _, document := range documents {
		texts = append(texts, document.Content)
	}

	response, err := i.embeddingClient.EmbedTexts(ctx, wbregistry.EmbeddingRequest{
		Model:      i.model,
		Dimensions: i.dimensions,
		Texts:      texts,
	})
	if err != nil {
		return 0, fmt.Errorf("embed registry operation documents: %w", err)
	}

	if len(response.Vectors) != len(documents) {
		return 0, fmt.Errorf("embedding response count mismatch: documents=%d vectors=%d", len(documents), len(response.Vectors))
	}

	for index, document := range documents {
		embedding := wbregistry.OperationEmbedding{
			OperationID: document.OperationID,
			SourceFile:  document.SourceFile,
			Model:       i.model,
			Dimensions:  i.dimensions,
			ContentHash: document.ContentHash,
			Vector:      response.Vectors[index],
		}

		// WHY: Embeddings are keyed by operation/model/dimensions and can be safely replaced when source content changes.
		if err := i.embeddingStore.UpsertOperationEmbedding(ctx, embedding); err != nil {
			return index, fmt.Errorf("upsert operation embedding %s: %w", document.OperationID, err)
		}
	}

	return len(documents), nil
}
