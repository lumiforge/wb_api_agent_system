package llm

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Allows application services to depend on planning behavior without importing concrete agents.
type Planner interface {
	Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error)
}
