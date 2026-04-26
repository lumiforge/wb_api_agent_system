package wbregistry

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Defines registry retrieval contracts used by planners without depending on storage adapters.
type SearchQuery struct {
	Query        string
	Limit        int
	ReadonlyOnly bool
	ExcludeJam   bool
}

type Stats struct {
	Total                int64 `json:"total"`
	Read                 int64 `json:"read"`
	Write                int64 `json:"write"`
	UnknownReadonly      int64 `json:"unknown_readonly"`
	JamOnly              int64 `json:"jam_only"`
	GeneratedOperationID int64 `json:"generated_operation_id"`
}

type Retriever interface {
	SearchOperations(ctx context.Context, query SearchQuery) ([]entities.WBRegistryOperation, error)
	GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error)
	Stats(ctx context.Context) (Stats, error)
}
