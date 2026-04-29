package wbregistry

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Defines registry retrieval query constraints used by planner-facing retrieval services.
type SearchQuery struct {
	Query        string
	Limit        int
	ReadonlyOnly bool
	ExcludeJam   bool
}

// PURPOSE: Describes retrieval-stage candidate counts without exposing embeddings or model internals.
type SearchDiagnostics struct {
	LexicalCandidates        int  `json:"lexical_candidates"`
	SemanticCandidates       int  `json:"semantic_candidates"`
	MergedCandidates         int  `json:"merged_candidates"`
	ReturnedCandidates       int  `json:"returned_candidates"`
	SemanticExpansionEnabled bool `json:"semantic_expansion_enabled"`
}

type SearchResult struct {
	Operations  []entities.WBRegistryOperation `json:"operations"`
	Diagnostics SearchDiagnostics              `json:"diagnostics"`
}

// PURPOSE: Optional debug retrieval contract for exposing safe retrieval diagnostics.
type DiagnosticRetriever interface {
	SearchOperationsWithDiagnostics(ctx context.Context, query SearchQuery) (SearchResult, error)
}

type Stats struct {
	Total                int64 `json:"total"`
	Read                 int64 `json:"read"`
	Write                int64 `json:"write"`
	UnknownReadonly      int64 `json:"unknown_readonly"`
	JamOnly              int64 `json:"jam_only"`
	GeneratedOperationID int64 `json:"generated_operation_id"`
}

// PURPOSE: Provides raw source-of-record registry access without retrieval ranking ownership.
type RawOperationStore interface {
	RawSearchOperations(ctx context.Context, query SearchQuery) ([]entities.WBRegistryOperation, error)
	GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error)
	Stats(ctx context.Context) (Stats, error)
	ListOperations(ctx context.Context) ([]entities.WBRegistryOperation, error)
}

// PURPOSE: Defines ranked registry retrieval contracts used by planners without depending on storage adapters.
type Retriever interface {
	SearchOperations(ctx context.Context, query SearchQuery) ([]entities.WBRegistryOperation, error)
	GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error)
	Stats(ctx context.Context) (Stats, error)
}
