package deterministic_planner

import (
	"context"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Routes known business requests to deterministic plan builders before LLM planning is used.
type Planner struct {
	scenarios []Scenario
}

type Scenario interface {
	TryPlan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, bool, error)
}

func New(registry wbregistry.Retriever) *Planner {
	return &Planner{
		scenarios: []Scenario{
			NewSellerWarehouseStocksScenario(registry),
		},
	}
}

func (p *Planner) TryPlan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, bool, error) {
	for _, scenario := range p.scenarios {
		plan, handled, err := scenario.TryPlan(ctx, request)
		if err != nil || handled {
			return plan, handled, err
		}
	}

	return nil, false, nil
}
