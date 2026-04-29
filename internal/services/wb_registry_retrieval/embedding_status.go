package wb_registry_retrieval

import (
	"context"
	"fmt"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Provides safe read-only visibility into registry embedding index coverage.
type EmbeddingIndexStatusService struct {
	sourceStore    wbregistry.RawOperationStore
	embeddingStore wbregistry.OperationEmbeddingStore
	model          string
	dimensions     int
}

type EmbeddingIndexStatus struct {
	RegistryOperations int64   `json:"registry_operations"`
	IndexedEmbeddings  int64   `json:"indexed_embeddings"`
	CoverageRatio      float64 `json:"coverage_ratio"`
	Model              string  `json:"model"`
	Dimensions         int     `json:"dimensions"`
}

type EmbeddingIndexStatusServiceConfig struct {
	SourceStore    wbregistry.RawOperationStore
	EmbeddingStore wbregistry.OperationEmbeddingStore
	Model          string
	Dimensions     int
}

func NewEmbeddingIndexStatusService(
	cfg EmbeddingIndexStatusServiceConfig,
) (*EmbeddingIndexStatusService, error) {
	if cfg.SourceStore == nil {
		return nil, fmt.Errorf("source store is required")
	}
	if cfg.EmbeddingStore == nil {
		return nil, fmt.Errorf("embedding store is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	if cfg.Dimensions <= 0 {
		return nil, fmt.Errorf("embedding dimensions must be positive")
	}

	return &EmbeddingIndexStatusService{
		sourceStore:    cfg.SourceStore,
		embeddingStore: cfg.EmbeddingStore,
		model:          cfg.Model,
		dimensions:     cfg.Dimensions,
	}, nil
}

func (s *EmbeddingIndexStatusService) Status(
	ctx context.Context,
) (EmbeddingIndexStatus, error) {
	registryStats, err := s.sourceStore.Stats(ctx)
	if err != nil {
		return EmbeddingIndexStatus{}, fmt.Errorf("get registry stats: %w", err)
	}

	embeddingStats, err := s.embeddingStore.StatsOperationEmbeddings(ctx, s.model, s.dimensions)
	if err != nil {
		return EmbeddingIndexStatus{}, fmt.Errorf("get embedding stats: %w", err)
	}

	coverageRatio := 0.0
	if registryStats.Total > 0 {
		coverageRatio = float64(embeddingStats.Total) / float64(registryStats.Total)
	}

	return EmbeddingIndexStatus{
		RegistryOperations: registryStats.Total,
		IndexedEmbeddings:  embeddingStats.Total,
		CoverageRatio:      coverageRatio,
		Model:              s.model,
		Dimensions:         s.dimensions,
	}, nil
}
