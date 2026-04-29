package wb_registry_retrieval

import (
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func TestEmbeddingDocumentBuilderBuildsStableDocument(t *testing.T) {
	builder := NewEmbeddingDocumentBuilder()

	operation := testEmbeddingDocumentOperation()

	first := builder.BuildOperationDocument(operation)
	second := builder.BuildOperationDocument(operation)

	if first.OperationID != operation.OperationID {
		t.Fatalf("expected operation_id %q, got %q", operation.OperationID, first.OperationID)
	}

	if first.SourceFile != operation.SourceFile {
		t.Fatalf("expected source_file %q, got %q", operation.SourceFile, first.SourceFile)
	}

	if first.Content == "" {
		t.Fatal("expected non-empty content")
	}

	if first.Content != second.Content {
		t.Fatal("expected stable content")
	}

	if first.ContentHash != second.ContentHash {
		t.Fatal("expected stable content hash")
	}

	if len(first.ContentHash) != 64 {
		t.Fatalf("expected sha256 hex hash length 64, got %d", len(first.ContentHash))
	}
}

func TestEmbeddingDocumentBuilderIncludesSearchRelevantRegistryFields(t *testing.T) {
	builder := NewEmbeddingDocumentBuilder()

	document := builder.BuildOperationDocument(testEmbeddingDocumentOperation())

	expectedParts := []string{
		"marketplace: wildberries",
		"source_file: reports.yaml",
		"operation_id: generated_get_api_v1_supplier_sales",
		"method: GET",
		"path_template: /api/v1/supplier/sales",
		"category: Основные отчёты",
		"x_category: analytics",
		"tags: Продажи, Отчёты",
		"summary: Продажи",
		"description: Метод возвращает информацию о продажах и возвратах.",
		"requires_jam: false",
		"readonly: true",
		"token_types: seller",
		"query_params_schema:",
		"response_schema:",
	}

	for _, expected := range expectedParts {
		if !strings.Contains(document.Content, expected) {
			t.Fatalf("expected document content to contain %q\ncontent:\n%s", expected, document.Content)
		}
	}
}

func TestEmbeddingDocumentBuilderHashChangesWhenContentChanges(t *testing.T) {
	builder := NewEmbeddingDocumentBuilder()

	operation := testEmbeddingDocumentOperation()
	first := builder.BuildOperationDocument(operation)

	operation.Summary = "Продажи и возвраты"
	second := builder.BuildOperationDocument(operation)

	if first.ContentHash == second.ContentHash {
		t.Fatal("expected content hash to change when embedding content changes")
	}
}

func TestEmbeddingDocumentBuilderReadonlyUnknownWhenRegistryFlagMissing(t *testing.T) {
	builder := NewEmbeddingDocumentBuilder()

	operation := testEmbeddingDocumentOperation()
	operation.XReadonlyMethod = nil

	document := builder.BuildOperationDocument(operation)

	if !strings.Contains(document.Content, "readonly: unknown") {
		t.Fatalf("expected readonly unknown, got content:\n%s", document.Content)
	}
}

func testEmbeddingDocumentOperation() entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "reports.yaml",
		OperationID:              "generated_get_api_v1_supplier_sales",
		Method:                   "GET",
		ServerURL:                "https://statistics-api.wildberries.ru",
		PathTemplate:             "/api/v1/supplier/sales",
		Tags:                     []string{"Продажи", "Отчёты"},
		Category:                 "Основные отчёты",
		Summary:                  "Продажи",
		Description:              "Метод возвращает информацию о продажах и возвратах.",
		XReadonlyMethod:          &readonly,
		XCategory:                "analytics",
		XTokenTypes:              []string{"seller"},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    `{"dateFrom":{"required":true,"schema":{"type":"string"}}}`,
		HeadersSchemaJSON:        `{"Authorization":{"required":true}}`,
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       `{"200":{"content":{"application/json":{"schema":{"type":"array"}}}}}`,
		RateLimitNotes:           "1 request per minute",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}
