package planning

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Defines deterministic ApiExecutionPlan composition from registry-backed selected operations.
type ApiPlanComposer interface {
	Compose(ctx context.Context, input entities.ApiPlanCompositionInput) (*entities.ApiExecutionPlan, error)
}
