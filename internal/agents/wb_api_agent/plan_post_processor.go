package wb_api_agent

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Applies registry validation and safe deterministic normalization before executor handoff.
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

	// WHY: Composed plans should set execution_mode, but boundary validation keeps the output contract explicit.
	// Normalize before validation so a ready client-executable plan is not blocked
	// only because this technical field is absent.
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
