package wbregistry

import "context"

// PURPOSE: Represents a persisted vector for one registry operation and one embedding model configuration.
type OperationEmbedding struct {
	OperationID string
	SourceFile  string
	Model       string
	Dimensions  int
	ContentHash string
	Vector      []float64
}

// PURPOSE: Reports safe aggregate embedding coverage without exposing stored vectors.
type OperationEmbeddingStats struct {
	Total int64 `json:"total"`
}

// PURPOSE: Defines source-of-record storage for registry operation embeddings, separate from ADK sessions.
type OperationEmbeddingStore interface {
	UpsertOperationEmbedding(ctx context.Context, embedding OperationEmbedding) error
	GetOperationEmbedding(ctx context.Context, operationID string, model string, dimensions int) (*OperationEmbedding, error)
	ListOperationEmbeddings(ctx context.Context, model string, dimensions int) ([]OperationEmbedding, error)
	StatsOperationEmbeddings(ctx context.Context, model string, dimensions int) (OperationEmbeddingStats, error)
}
