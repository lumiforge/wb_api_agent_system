package wb_api_agent

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

type singleOperationRegistry struct{ operation entities.WBRegistryOperation }

func (r *singleOperationRegistry) SearchOperations(ctx context.Context, query wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	return []entities.WBRegistryOperation{r.operation}, nil
}
func (r *singleOperationRegistry) GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error) {
	if operationID != r.operation.OperationID {
		return nil, nil
	}
	operation := r.operation
	return &operation, nil
}

func (r *singleOperationRegistry) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}
