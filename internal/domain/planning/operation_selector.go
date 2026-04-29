package planning

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Defines the probabilistic operation-selection boundary used before deterministic composition.
type OperationSelector interface {
	SelectOperations(ctx context.Context, input entities.OperationSelectionInput) (*entities.OperationSelectionPlan, error)
}
