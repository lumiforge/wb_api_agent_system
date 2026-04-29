package wbregistry

import "context"

// PURPOSE: Defines the model boundary for converting text documents into embedding vectors.
type EmbeddingClient interface {
	EmbedTexts(ctx context.Context, input EmbeddingRequest) (EmbeddingResponse, error)
}

type EmbeddingRequest struct {
	Model      string
	Dimensions int
	Texts      []string
}

type EmbeddingResponse struct {
	Model      string
	Dimensions int
	Vectors    [][]float64
}
