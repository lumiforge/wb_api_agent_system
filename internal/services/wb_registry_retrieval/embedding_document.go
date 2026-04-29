package wb_registry_retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Builds stable embedding input documents from registry source-of-record operations.
type EmbeddingDocumentBuilder struct{}

type OperationEmbeddingDocument struct {
	OperationID string
	SourceFile  string
	Content     string
	ContentHash string
}

func NewEmbeddingDocumentBuilder() *EmbeddingDocumentBuilder {
	return &EmbeddingDocumentBuilder{}
}

func (b *EmbeddingDocumentBuilder) BuildOperationDocument(
	operation entities.WBRegistryOperation,
) OperationEmbeddingDocument {
	content := stableOperationEmbeddingContent(operation)

	hashBytes := sha256.Sum256([]byte(content))
	contentHash := hex.EncodeToString(hashBytes[:])

	return OperationEmbeddingDocument{
		OperationID: operation.OperationID,
		SourceFile:  operation.SourceFile,
		Content:     content,
		ContentHash: contentHash,
	}
}

func stableOperationEmbeddingContent(operation entities.WBRegistryOperation) string {
	lines := []string{
		"marketplace: " + operation.Marketplace,
		"source_file: " + operation.SourceFile,
		"operation_id: " + operation.OperationID,
		"method: " + operation.Method,
		"path_template: " + operation.PathTemplate,
		"category: " + operation.Category,
		"x_category: " + operation.XCategory,
		"tags: " + strings.Join(operation.Tags, ", "),
		"summary: " + strings.Join(strings.Fields(operation.Summary), " "),
		"description: " + strings.Join(strings.Fields(operation.Description), " "),
		"rate_limit_notes: " + strings.Join(strings.Fields(operation.RateLimitNotes), " "),
		"subscription_requirements: " + strings.Join(strings.Fields(operation.SubscriptionRequirements), " "),
		"requires_jam: " + boolText(operation.RequiresJam),
		"readonly: " + readonlyText(operation.XReadonlyMethod),
		"token_types: " + strings.Join(operation.XTokenTypes, ", "),
		"path_params_schema: " + compactWhitespace(operation.PathParamsSchemaJSON),
		"query_params_schema: " + compactWhitespace(operation.QueryParamsSchemaJSON),
		"request_body_schema: " + compactWhitespace(operation.RequestBodySchemaJSON),
		"response_schema: " + compactWhitespace(operation.ResponseSchemaJSON),
	}

	return strings.Join(lines, "\n")
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func boolText(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

func readonlyText(value *bool) string {
	if value == nil {
		return "unknown"
	}

	return boolText(*value)
}
