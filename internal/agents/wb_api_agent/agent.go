package wb_api_agent

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Owns WB API planning behavior implemented through ADK agents.
type Agent struct{}

func New() *Agent {
	return &Agent{}
}

func (a *Agent) Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error) {
	// WHY: The scaffold must return the required ApiExecutionPlan shape before registry retrieval and LLM planning exist.
	return entities.NewNeedsClarificationPlan(request, []string{
		"WB API planner is not initialized yet.",
	}), nil
}
