package wb_api_agent

import (
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Protects validator responsibility boundaries for ready-plan shape and recoverable missing inputs.
func TestValidateReadyPlanShapeAllowsEmptyIntentAndEmptyInputs(t *testing.T) {
	plan := entities.ApiExecutionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        "ready",
		Intent:        "",
		RiskLevel:     "read",
		ExecutionMode: "automatic",
		Inputs:        map[string]entities.InputValue{},
		Steps: []entities.ApiPlanStep{
			{
				StepID:      "step-1",
				OperationID: "operation-1",
			},
		},
		Transforms: []entities.TransformStep{},
		Warnings:   []entities.PlanWarning{},
		Validation: entities.PlanValidation{
			Errors: []string{},
		},
	}

	errors := validateReadyPlanShape(entities.BusinessRequest{}, plan)

	assertNoStringContaining(t, errors, "intent is empty")
	assertNoStringContaining(t, errors, "inputs is empty")
}

func TestValidateReadyPlanShapeRejectsNilInputs(t *testing.T) {
	plan := entities.ApiExecutionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        "ready",
		RiskLevel:     "read",
		ExecutionMode: "automatic",
		Inputs:        nil,
		Steps: []entities.ApiPlanStep{
			{
				StepID:      "step-1",
				OperationID: "operation-1",
			},
		},
		Transforms: []entities.TransformStep{},
		Warnings:   []entities.PlanWarning{},
		Validation: entities.PlanValidation{
			Errors: []string{},
		},
	}

	errors := validateReadyPlanShape(entities.BusinessRequest{}, plan)

	assertStringContaining(t, errors, "inputs must be an object")
}

func TestValidateRequiredInputBindingReturnsClarificationNotBlock(t *testing.T) {
	result := validateValueBinding(
		"step-1",
		"body.chrtIds",
		entities.ValueBinding{
			Source:    "input",
			InputName: "chrt_ids",
			Required:  true,
		},
		map[string]entities.InputValue{},
	)

	if len(result.BlockErrors) != 0 {
		t.Fatalf("expected no block errors, got %v", result.BlockErrors)
	}

	if len(result.ClarifyingQuestions) != 1 {
		t.Fatalf("expected one clarifying question, got %v", result.ClarifyingQuestions)
	}

	question := result.ClarifyingQuestions[0]

	assertStringDoesNotContain(t, question, "entities.")
	assertStringDoesNotContain(t, question, "chrt_ids")
	assertStringContains(t, question, "product identifiers")
}

func TestValidateEmptyRequiredInputBindingReturnsClarificationNotBlock(t *testing.T) {
	result := validateValueBinding(
		"step-1",
		"path_params.warehouseId",
		entities.ValueBinding{
			Source:    "input",
			InputName: "warehouse_id",
			Required:  true,
		},
		map[string]entities.InputValue{
			"warehouse_id": {
				Type:     "integer",
				Required: true,
				Value:    nil,
			},
		},
	)

	if len(result.BlockErrors) != 0 {
		t.Fatalf("expected no block errors, got %v", result.BlockErrors)
	}

	if len(result.ClarifyingQuestions) != 1 {
		t.Fatalf("expected one clarifying question, got %v", result.ClarifyingQuestions)
	}

	question := result.ClarifyingQuestions[0]

	assertStringDoesNotContain(t, question, "entities.")
	assertStringDoesNotContain(t, question, "warehouse_id")
	assertStringContains(t, question, "warehouse")
}

func TestValidateOptionalMissingInputBindingReturnsBlock(t *testing.T) {
	result := validateValueBinding(
		"step-1",
		"query_params.locale",
		entities.ValueBinding{
			Source:    "input",
			InputName: "locale",
			Required:  false,
		},
		map[string]entities.InputValue{},
	)

	if len(result.ClarifyingQuestions) != 0 {
		t.Fatalf("expected no clarifying questions, got %v", result.ClarifyingQuestions)
	}

	assertStringContaining(t, result.BlockErrors, "references missing optional input locale")
}

func TestValidateRequiredRequestBodyUsesUserFacingClarification(t *testing.T) {
	schemaJSON := `{
		"content": {
			"application/json": {
				"schema": {
					"type": "object",
					"required": ["chrtIds"],
					"properties": {
						"chrtIds": {
							"type": "array"
						}
					}
				}
			}
		}
	}`

	questions := validateRequiredRequestBody(
		"step-1",
		schemaJSON,
		map[string]any{},
		map[string]entities.InputValue{},
	)

	if len(questions) != 1 {
		t.Fatalf("expected one question, got %v", questions)
	}

	question := questions[0]

	assertStringDoesNotContain(t, question, "entities.")
	assertStringDoesNotContain(t, question, "chrt_ids")
	assertStringContains(t, question, "product identifiers")
}

func TestValidateRequiredRequestBodyWithNonObjectBodyUsesUserFacingClarification(t *testing.T) {
	schemaJSON := `{
		"content": {
			"application/json": {
				"schema": {
					"type": "object",
					"required": ["dateFrom"],
					"properties": {
						"dateFrom": {
							"type": "string"
						}
					}
				}
			}
		}
	}`

	questions := validateRequiredRequestBody(
		"step-1",
		schemaJSON,
		nil,
		map[string]entities.InputValue{},
	)

	if len(questions) != 1 {
		t.Fatalf("expected one question, got %v", questions)
	}

	question := questions[0]

	assertStringDoesNotContain(t, question, "entities.")
	assertStringDoesNotContain(t, question, "date_from")
	assertStringContains(t, question, "period start date")
}

func assertStringContaining(t *testing.T, values []string, expected string) {
	t.Helper()

	for _, value := range values {
		if strings.Contains(value, expected) {
			return
		}
	}

	t.Fatalf("expected one of %v to contain %q", values, expected)
}

func assertNoStringContaining(t *testing.T, values []string, unexpected string) {
	t.Helper()

	for _, value := range values {
		if strings.Contains(value, unexpected) {
			t.Fatalf("did not expect %q in %v", unexpected, values)
		}
	}
}

func assertStringContains(t *testing.T, value string, expected string) {
	t.Helper()

	if !strings.Contains(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}

func assertStringDoesNotContain(t *testing.T, value string, unexpected string) {
	t.Helper()

	if strings.Contains(value, unexpected) {
		t.Fatalf("did not expect %q to contain %q", value, unexpected)
	}
}
