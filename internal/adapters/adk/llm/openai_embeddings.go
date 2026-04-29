package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

var _ wbregistry.EmbeddingClient = (*OpenAICompatibleEmbeddingClient)(nil)

// PURPOSE: Calls an OpenAI-compatible embeddings endpoint through the registry embedding boundary.
type OpenAICompatibleEmbeddingClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type embeddingRequestBody struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

type embeddingResponseBody struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
}

const (
	embeddingMaxAttempts      = 8
	embeddingDefaultRetryWait = 10 * time.Second
)

func NewOpenAICompatibleEmbeddingClient(baseURL string, apiKey string) *OpenAICompatibleEmbeddingClient {
	return &OpenAICompatibleEmbeddingClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  http.DefaultClient,
	}
}

func (c *OpenAICompatibleEmbeddingClient) EmbedTexts(
	ctx context.Context,
	input wbregistry.EmbeddingRequest,
) (wbregistry.EmbeddingResponse, error) {
	if strings.TrimSpace(input.Model) == "" {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("embedding model is required")
	}
	if input.Dimensions <= 0 {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("embedding dimensions must be positive")
	}
	if len(input.Texts) == 0 {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("embedding texts must not be empty")
	}
	for index, text := range input.Texts {
		if strings.TrimSpace(text) == "" {
			return wbregistry.EmbeddingResponse{}, fmt.Errorf("embedding text[%d] must not be empty", index)
		}
	}

	body := embeddingRequestBody{
		Model:          input.Model,
		Input:          input.Texts,
		Dimensions:     input.Dimensions,
		EncodingFormat: "float",
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("marshal embeddings request: %w", err)
	}

	responsePayload, err := c.sendEmbeddingRequestWithRetry(ctx, payload)
	if err != nil {
		return wbregistry.EmbeddingResponse{}, err
	}

	var responseBody embeddingResponseBody
	if err := json.Unmarshal(responsePayload, &responseBody); err != nil {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("parse embeddings response: %w", err)
	}

	if len(responseBody.Data) != len(input.Texts) {
		return wbregistry.EmbeddingResponse{}, fmt.Errorf("embeddings response count mismatch: expected=%d got=%d", len(input.Texts), len(responseBody.Data))
	}

	vectors := make([][]float64, len(responseBody.Data))
	for _, item := range responseBody.Data {
		if item.Index < 0 || item.Index >= len(responseBody.Data) {
			return wbregistry.EmbeddingResponse{}, fmt.Errorf("embeddings response index out of range: %d", item.Index)
		}

		if len(item.Embedding) != input.Dimensions {
			return wbregistry.EmbeddingResponse{}, fmt.Errorf("embedding dimensions mismatch at index %d: expected=%d got=%d", item.Index, input.Dimensions, len(item.Embedding))
		}

		vectors[item.Index] = item.Embedding
	}

	return wbregistry.EmbeddingResponse{
		Model:      responseBody.Model,
		Dimensions: input.Dimensions,
		Vectors:    vectors,
	}, nil
}

func (c *OpenAICompatibleEmbeddingClient) sendEmbeddingRequestWithRetry(
	ctx context.Context,
	payload []byte,
) ([]byte, error) {
	var lastErr error

	for attempt := 1; attempt <= embeddingMaxAttempts; attempt++ {
		responsePayload, retryAfter, err := c.sendEmbeddingRequest(ctx, payload)
		if err == nil {
			return responsePayload, nil
		}

		lastErr = err

		if retryAfter <= 0 {
			return nil, err
		}

		if attempt == embeddingMaxAttempts {
			break
		}

		timer := time.NewTimer(retryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("embedding retry context: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return nil, lastErr
}

func (c *OpenAICompatibleEmbeddingClient) sendEmbeddingRequest(
	ctx context.Context,
	payload []byte,
) ([]byte, time.Duration, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("create embeddings request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("send embeddings request: %w", err)
	}
	defer httpResp.Body.Close()

	responsePayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read embeddings response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, embeddingRetryAfter(httpResp), fmt.Errorf("embeddings request rate limited: status=%d body=%s", httpResp.StatusCode, string(responsePayload))
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("embeddings request failed: status=%d body=%s", httpResp.StatusCode, string(responsePayload))
	}

	return responsePayload, 0, nil
}

func embeddingRetryAfter(response *http.Response) time.Duration {
	retryAfter := strings.TrimSpace(response.Header.Get("Retry-After"))
	if retryAfter == "" {
		return embeddingDefaultRetryWait
	}

	seconds, err := strconv.Atoi(retryAfter)
	if err != nil || seconds <= 0 {
		return embeddingDefaultRetryWait
	}

	return time.Duration(seconds) * time.Second
}
