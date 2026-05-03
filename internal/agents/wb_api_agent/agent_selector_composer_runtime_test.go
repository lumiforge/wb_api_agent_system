package wb_api_agent

import (
	"context"
	orch "github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/orchestration"
	"io"
	"log"
	"testing"
)

func TestAgentPlanUsesSelectorComposerPipelineAfterRegistryRetrieval(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")

	agent := &Agent{
		registry: &singleOperationRegistry{
			operation: registryOperation,
		},

		operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(),
		operationSelector: &fakeOperationSelector{
			plan: validPipelineSelectionPlan("operation_stocks"),
		},
		apiPlanComposer: &fakeApiPlanComposer{
			plan: validPipelineExecutionPlan(registryOperation),
		},
		postProcessor: orch.NewPlanPostProcessor(&singleOperationRegistry{
			operation: registryOperation,
		}),
		logger: testLogger(),
	}

	plan, err := agent.Plan(context.Background(), validPipelineBusinessRequest())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "ready" {
		t.Fatalf("expected ready plan, got %q", plan.Status)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}

	if plan.Steps[0].OperationID != "operation_stocks" {
		t.Fatalf("expected operation_stocks, got %q", plan.Steps[0].OperationID)
	}
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
