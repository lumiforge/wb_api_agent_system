package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

func TestOpenAICompatibleEmbeddingClientEmbedTexts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/embeddings" {
			t.Fatalf("expected /embeddings, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}

		var request embeddingRequestBody
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if request.Model != "text-embedding-3-small" {
			t.Fatalf("expected model text-embedding-3-small, got %q", request.Model)
		}

		if request.Dimensions != 3 {
			t.Fatalf("expected dimensions 3, got %d", request.Dimensions)
		}

		if request.EncodingFormat != "float" {
			t.Fatalf("expected encoding_format float, got %q", request.EncodingFormat)
		}

		if len(request.Input) != 2 || request.Input[0] != "first document" || request.Input[1] != "second document" {
			t.Fatalf("unexpected input %#v", request.Input)
		}

		writeEmbeddingTestJSON(t, w, embeddingResponseBody{
			Model: "text-embedding-3-small",
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float64{0.4, 0.5, 0.6},
					Index:     1,
				},
				{
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     0,
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAICompatibleEmbeddingClient(server.URL, "test-key")

	response, err := client.EmbedTexts(context.Background(), wbregistry.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		Texts:      []string{"first document", "second document"},
	})
	if err != nil {
		t.Fatalf("embed texts: %v", err)
	}

	if response.Model != "text-embedding-3-small" {
		t.Fatalf("expected response model text-embedding-3-small, got %q", response.Model)
	}

	if response.Dimensions != 3 {
		t.Fatalf("expected dimensions 3, got %d", response.Dimensions)
	}

	assertEmbeddingVector(t, response.Vectors[0], []float64{0.1, 0.2, 0.3})
	assertEmbeddingVector(t, response.Vectors[1], []float64{0.4, 0.5, 0.6})
}

func TestOpenAICompatibleEmbeddingClientRejectsInvalidInput(t *testing.T) {
	client := NewOpenAICompatibleEmbeddingClient("http://127.0.0.1", "test-key")

	tests := []struct {
		name    string
		request wbregistry.EmbeddingRequest
	}{
		{
			name: "empty model",
			request: wbregistry.EmbeddingRequest{
				Dimensions: 3,
				Texts:      []string{"document"},
			},
		},
		{
			name: "non-positive dimensions",
			request: wbregistry.EmbeddingRequest{
				Model:      "text-embedding-3-small",
				Dimensions: 0,
				Texts:      []string{"document"},
			},
		},
		{
			name: "empty texts",
			request: wbregistry.EmbeddingRequest{
				Model:      "text-embedding-3-small",
				Dimensions: 3,
				Texts:      []string{},
			},
		},
		{
			name: "blank text",
			request: wbregistry.EmbeddingRequest{
				Model:      "text-embedding-3-small",
				Dimensions: 3,
				Texts:      []string{" "},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.EmbedTexts(context.Background(), test.request)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestOpenAICompatibleEmbeddingClientReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewOpenAICompatibleEmbeddingClient(server.URL, "test-key")

	_, err := client.EmbedTexts(context.Background(), wbregistry.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		Texts:      []string{"document"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenAICompatibleEmbeddingClientRejectsResponseCountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEmbeddingTestJSON(t, w, embeddingResponseBody{
			Model: "text-embedding-3-small",
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     0,
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAICompatibleEmbeddingClient(server.URL, "test-key")

	_, err := client.EmbedTexts(context.Background(), wbregistry.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		Texts:      []string{"first document", "second document"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenAICompatibleEmbeddingClientRejectsDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEmbeddingTestJSON(t, w, embeddingResponseBody{
			Model: "text-embedding-3-small",
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float64{0.1, 0.2},
					Index:     0,
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAICompatibleEmbeddingClient(server.URL, "test-key")

	_, err := client.EmbedTexts(context.Background(), wbregistry.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		Texts:      []string{"document"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenAICompatibleEmbeddingClientRejectsOutOfRangeIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEmbeddingTestJSON(t, w, embeddingResponseBody{
			Model: "text-embedding-3-small",
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     2,
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAICompatibleEmbeddingClient(server.URL, "test-key")

	_, err := client.EmbedTexts(context.Background(), wbregistry.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		Texts:      []string{"document"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func writeEmbeddingTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func assertEmbeddingVector(t *testing.T, actual []float64, expected []float64) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("expected vector len %d, got %d", len(expected), len(actual))
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("expected vector[%d]=%v, got %v", index, expected[index], actual[index])
		}
	}
}
