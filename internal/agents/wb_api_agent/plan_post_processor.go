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
	normalizeExecutionMode(plan)
	validationPlan, err := p.validatePlan(ctx, request, plan)
	if err != nil {
		return nil, err
	}

	if validationPlan != nil {
		validationPlan.Metadata = request.Metadata
		return validationPlan, nil
	}

	plan.Metadata = request.Metadata
	return nil, nil
}

func normalizeExecutionMode(plan *entities.ApiExecutionPlan) {
	if plan == nil || plan.ExecutionMode != "" {
		return
	}

	// WHY: ADK fallback can return an otherwise valid plan with empty execution_mode.
	// Normalize before validation so a ready client-executable plan is not blocked
	// only because the LLM omitted this technical field.
	switch plan.Status {
	case "ready":
		plan.ExecutionMode = "automatic"
	case "needs_clarification":
		plan.ExecutionMode = "not_executable"
	case "blocked":
		plan.ExecutionMode = "not_executable"
	default:
		plan.ExecutionMode = "not_executable"
	}
}
