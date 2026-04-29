package entities

import (
	"errors"
	"strings"
	"testing"
)

// PURPOSE: Protects OperationSelectionPlan as a selector-to-composer contract, not a business classifier.
func TestOperationSelectionPlanValidateShapeAcceptsReadyForComposition(t *testing.T) {
	plan := OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        OperationSelectionStatusReadyForComposition,
		SelectedOperations: []SelectedOperation{
			{
				OperationID:   "operation-1",
				Purpose:       "Fetch product cards.",
				DependsOn:     []string{},
				InputStrategy: OperationInputStrategyStaticDefaults,
			},
		},
		MissingInputs:      []MissingBusinessInput{},
		RejectedCandidates: []RejectedOperationCandidate{},
		Warnings:           []PlanWarning{},
	}

	if err := plan.ValidateShape(); err != nil {
		t.Fatalf("expected valid plan, got %v", err)
	}
}

func TestOperationSelectionPlanValidateShapeRejectsReadyWithoutSelectedOperations(t *testing.T) {
	plan := validReadyOperationSelectionPlan()
	plan.SelectedOperations = []SelectedOperation{}

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "ready_for_composition requires at least one selected operation")
}

func TestOperationSelectionPlanValidateShapeRejectsReadyWithMissingInputs(t *testing.T) {
	plan := validReadyOperationSelectionPlan()
	plan.MissingInputs = []MissingBusinessInput{
		{
			Code:           "warehouse",
			UserQuestion:   "Provide the seller warehouse.",
			Accepts:        []string{"warehouse ID", "warehouse name"},
			InternalFields: []string{"warehouse_id"},
		},
	}

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "ready_for_composition must not contain missing inputs")
}

func TestOperationSelectionPlanValidateShapeAcceptsNeedsClarification(t *testing.T) {
	plan := OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        OperationSelectionStatusNeedsClarification,
		SelectedOperations: []SelectedOperation{
			{
				OperationID:   "operation-1",
				Purpose:       "Fetch product stocks.",
				DependsOn:     []string{},
				InputStrategy: OperationInputStrategyBusinessEntities,
			},
		},
		MissingInputs: []MissingBusinessInput{
			{
				Code:           "warehouse",
				UserQuestion:   "Provide the seller warehouse.",
				Accepts:        []string{"warehouse ID", "warehouse name"},
				InternalFields: []string{"warehouse_id"},
			},
		},
		RejectedCandidates: []RejectedOperationCandidate{},
		Warnings:           []PlanWarning{},
	}

	if err := plan.ValidateShape(); err != nil {
		t.Fatalf("expected valid plan, got %v", err)
	}
}

func TestOperationSelectionPlanValidateShapeRejectsNeedsClarificationWithoutMissingInputs(t *testing.T) {
	plan := OperationSelectionPlan{
		SchemaVersion:      "1.0",
		RequestID:          "request-1",
		Marketplace:        "wildberries",
		Status:             OperationSelectionStatusNeedsClarification,
		SelectedOperations: []SelectedOperation{},
		MissingInputs:      []MissingBusinessInput{},
		RejectedCandidates: []RejectedOperationCandidate{},
		Warnings:           []PlanWarning{},
	}

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "needs_clarification requires at least one missing input")
}

func TestOperationSelectionPlanValidateShapeRejectsNilCollections(t *testing.T) {
	plan := validReadyOperationSelectionPlan()
	plan.SelectedOperations = nil
	plan.MissingInputs = nil
	plan.RejectedCandidates = nil
	plan.Warnings = nil

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "selected_operations must be an array")
	assertOperationSelectionShapeErrorContains(t, err, "missing_inputs must be an array")
	assertOperationSelectionShapeErrorContains(t, err, "rejected_candidates must be an array")
	assertOperationSelectionShapeErrorContains(t, err, "warnings must be an array")
}

func TestOperationSelectionPlanValidateShapeRejectsUnsupportedInputStrategy(t *testing.T) {
	plan := validReadyOperationSelectionPlan()
	plan.SelectedOperations[0].InputStrategy = OperationInputStrategy("semantic_guess")

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, `selected_operations[0].input_strategy is unsupported: "semantic_guess"`)
}

func TestOperationSelectionPlanValidateShapeRejectsBlockedWithSelectedOperations(t *testing.T) {
	plan := validReadyOperationSelectionPlan()
	plan.Status = OperationSelectionStatusBlocked
	plan.UserFacingSummary = "Request is blocked by policy."

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "blocked must not contain selected operations")
}

func TestOperationSelectionPlanValidateShapeRejectsUnsupportedWithoutSummary(t *testing.T) {
	plan := OperationSelectionPlan{
		SchemaVersion:      "1.0",
		RequestID:          "request-1",
		Marketplace:        "wildberries",
		Status:             OperationSelectionStatusUnsupported,
		SelectedOperations: []SelectedOperation{},
		MissingInputs:      []MissingBusinessInput{},
		RejectedCandidates: []RejectedOperationCandidate{},
		Warnings:           []PlanWarning{},
	}

	err := plan.ValidateShape()

	assertOperationSelectionShapeErrorContains(t, err, "unsupported requires user_facing_summary")
}

func validReadyOperationSelectionPlan() OperationSelectionPlan {
	return OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        OperationSelectionStatusReadyForComposition,
		SelectedOperations: []SelectedOperation{
			{
				OperationID:   "operation-1",
				Purpose:       "Fetch product cards.",
				DependsOn:     []string{},
				InputStrategy: OperationInputStrategyStaticDefaults,
			},
		},
		MissingInputs:      []MissingBusinessInput{},
		RejectedCandidates: []RejectedOperationCandidate{},
		Warnings:           []PlanWarning{},
	}
}

func assertOperationSelectionShapeErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	var shapeError OperationSelectionShapeValidationError
	if !errors.As(err, &shapeError) {
		t.Fatalf("expected OperationSelectionShapeValidationError, got %T: %v", err, err)
	}

	for _, message := range shapeError.Errors {
		if strings.Contains(message, expected) {
			return
		}
	}

	t.Fatalf("expected shape error containing %q, got %v", expected, shapeError.Errors)
}
