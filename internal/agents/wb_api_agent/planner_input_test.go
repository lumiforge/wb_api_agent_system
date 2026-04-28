package wb_api_agent

import (
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func TestBuildPlannerInputIncludesMetadata(t *testing.T) {
	agent := &Agent{}
	request := entities.BusinessRequest{
		RequestID: "req_1",
		Metadata: &entities.RequestMetadata{
			CorrelationID: "corr_1",
			SessionID:     "sess_1",
		},
	}

	input := agent.buildPlannerInput(request, nil)
	if input.Metadata == nil || input.Metadata.CorrelationID != "corr_1" {
		t.Fatalf("expected metadata copied to planner input, got %#v", input.Metadata)
	}
}
