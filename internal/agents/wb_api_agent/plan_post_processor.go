package wb_api_agent

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Applies registry validation and safe normalization to ADK-produced plans before executor handoff.
type PlanPostProcessor struct {
	registry wbregistry.Retriever
}

func NewPlanPostProcessor(registry wbregistry.Retriever) *PlanPostProcessor {
	return &PlanPostProcessor{
		registry: registry,
	}
}

func (p *PlanPostProcessor) Process(
	ctx context.Context,
	request entities.BusinessRequest,
	plan *entities.ApiExecutionPlan,
) (*entities.ApiExecutionPlan, error) {
	return p.validatePlan(ctx, request, plan)
}
