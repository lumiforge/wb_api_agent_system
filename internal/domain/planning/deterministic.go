package planning

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Defines deterministic planning contract without binding agents to concrete scenario implementations.
type DeterministicPlanner interface {
	TryPlan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, bool, error)
}
