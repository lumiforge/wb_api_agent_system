package selector

import (
	"errors"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
)

// PURPOSE: Protects ADK selector output parsing before selector results can reach deterministic composition.
func TestParseOperationSelectionPlanAcceptsValidJSON(t *testing.T) {
	response := `{
		"schema_version": "1.0",
		"request_id": "request-1",
		"marketplace": "wildberries",
		"status": "ready_for_composition",
		"selected_operations": [
			{
				"operation_id": "operation_cards",
				"purpose": "Fetch product cards.",
				"depends_on": [],
				"input_strategy": "static_defaults"
			}
		],
		"missing_inputs": [],
		"rejected_candidates": [],
		"warnings": []
	}`

	plan, err := parseOperationSelectionPlan(response)
	if err != nil {
		t.Fatalf("expected valid plan, got %v", err)
	}

	if plan.Status != entities.OperationSelectionStatusReadyForComposition {
		t.Fatalf("expected ready_for_composition, got %q", plan.Status)
	}
}

func TestParseOperationSelectionPlanAcceptsMarkdownJSONFence(t *testing.T) {
	response := "```json\n" + `{
		"schema_version": "1.0",
		"request_id": "request-1",
		"marketplace": "wildberries",
		"status": "needs_clarification",
		"selected_operations": [
			{
				"operation_id": "operation_stocks",
				"purpose": "Fetch stocks.",
				"depends_on": [],
				"input_strategy": "business_entities"
			}
		],
		"missing_inputs": [
			{
				"code": "warehouse",
				"user_question": "Provide the seller warehouse.",
				"accepts": ["warehouse ID", "warehouse name"],
				"internal_fields": ["warehouse_id"]
			}
		],
		"rejected_candidates": [],
		"warnings": []
	}` + "\n```"

	plan, err := parseOperationSelectionPlan(response)
	if err != nil {
		t.Fatalf("expected valid fenced plan, got %v", err)
	}

	if plan.Status != entities.OperationSelectionStatusNeedsClarification {
		t.Fatalf("expected needs_clarification, got %q", plan.Status)
	}
}

func TestParseOperationSelectionPlanRejectsInvalidJSON(t *testing.T) {
	_, err := parseOperationSelectionPlan(`{"schema_version":`)

	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseOperationSelectionPlanRejectsInvalidShape(t *testing.T) {
	response := `{
		"schema_version": "1.0",
		"request_id": "request-1",
		"marketplace": "wildberries",
		"status": "ready_for_composition",
		"selected_operations": [],
		"missing_inputs": [],
		"rejected_candidates": [],
		"warnings": []
	}`

	_, err := parseOperationSelectionPlan(response)

	var shapeError entities.OperationSelectionShapeValidationError
	if !errors.As(err, &shapeError) {
		t.Fatalf("expected OperationSelectionShapeValidationError, got %T: %v", err, err)
	}
}

func TestBuildOperationSelectorInstructionDeclaresLayerBoundaries(t *testing.T) {
	instruction := buildOperationSelectorInstruction()

	assertContains(t, instruction, "Your responsibility is only operation selection")
	assertContains(t, instruction, "Executable ApiExecutionPlan composition is forbidden")
	assertContains(t, instruction, "Use only operation_id values present in registry_candidates")
	assertContains(t, instruction, "Never put internal field names into missing_inputs[].user_question")
}

func TestADKOperationSelectorImplementsDomainInterface(t *testing.T) {
	var _ planning.OperationSelector = (*ADKOperationSelector)(nil)
}

func assertContains(t *testing.T, value string, expected string) {
	t.Helper()

	if !containsString(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}

func containsString(value string, expected string) bool {
	for index := 0; index+len(expected) <= len(value); index++ {
		if value[index:index+len(expected)] == expected {
			return true
		}
	}

	return false
}
