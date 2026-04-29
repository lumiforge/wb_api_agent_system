package planning

import (
	"errors"
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Protects the selector-to-composer boundary from invented operations and invalid dependencies.
func TestOperationSelectionValidatorAcceptsValidSelection(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()

	err := NewOperationSelectionValidator().Validate(input, plan)

	if err != nil {
		t.Fatalf("expected valid selection, got %v", err)
	}
}

func TestOperationSelectionValidatorRejectsSelectedOperationOutsideCandidates(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations[0].OperationID = "invented_operation"

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "selected_operations[0].operation_id invented_operation is not present in registry_candidates")
}

func TestOperationSelectionValidatorRejectsDuplicateSelectedOperation(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations = append(plan.SelectedOperations, entities.SelectedOperation{
		OperationID:   "operation_cards",
		Purpose:       "Fetch cards again.",
		DependsOn:     []string{},
		InputStrategy: entities.OperationInputStrategyStaticDefaults,
	})

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "selected_operations[1].operation_id operation_cards is duplicated")
}

func TestOperationSelectionValidatorRejectsRejectedCandidateOutsideCandidates(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.RejectedCandidates = []entities.RejectedOperationCandidate{
		{
			OperationID: "invented_rejected_operation",
			Reason:      "Not relevant.",
		},
	}

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "rejected_candidates[0].operation_id invented_rejected_operation is not present in registry_candidates")
}

func TestOperationSelectionValidatorAcceptsDependencyOnSelectedOperation(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations = append(plan.SelectedOperations, entities.SelectedOperation{
		OperationID:   "operation_stocks",
		Purpose:       "Fetch stocks.",
		DependsOn:     []string{"operation_cards"},
		InputStrategy: entities.OperationInputStrategyStepOutput,
	})

	err := NewOperationSelectionValidator().Validate(input, plan)

	if err != nil {
		t.Fatalf("expected valid dependency, got %v", err)
	}
}

func TestOperationSelectionValidatorRejectsDependencyOnUnselectedOperation(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations[0].DependsOn = []string{"operation_stocks"}

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "selected_operations[0].depends_on[0] references unselected operation operation_stocks")
}

func TestOperationSelectionValidatorRejectsEmptyDependency(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations[0].DependsOn = []string{""}

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "selected_operations[0].depends_on[0] is empty")
}

func TestOperationSelectionValidatorRejectsSelfDependency(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations[0].DependsOn = []string{"operation_cards"}

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "selected_operations[0].depends_on[0] self-references operation operation_cards")
}

func TestOperationSelectionValidatorRejectsRequestIDMismatch(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.RequestID = "other-request"

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "request_id mismatch: input=request-1 plan=other-request")
}

func TestOperationSelectionValidatorRejectsMarketplaceMismatch(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.Marketplace = "ozon"

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "marketplace mismatch: input=wildberries plan=ozon")
}

func TestOperationSelectionValidatorRejectsTechnicalLeakageInMissingInputQuestion(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.Status = entities.OperationSelectionStatusNeedsClarification
	plan.MissingInputs = []entities.MissingBusinessInput{
		{
			Code:           "warehouse",
			UserQuestion:   "Provide warehouse_id.",
			Accepts:        []string{"warehouse ID", "warehouse name"},
			InternalFields: []string{"warehouse_id"},
		},
	}

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "missing_inputs[0].user_question exposes internal field name")
}

func TestOperationSelectionValidatorAcceptsInternalFieldsOutsideUserQuestion(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.Status = entities.OperationSelectionStatusNeedsClarification
	plan.MissingInputs = []entities.MissingBusinessInput{
		{
			Code:           "warehouse",
			UserQuestion:   "Provide the seller warehouse.",
			Accepts:        []string{"warehouse ID", "warehouse name"},
			InternalFields: []string{"warehouse_id"},
		},
	}

	err := NewOperationSelectionValidator().Validate(input, plan)

	if err != nil {
		t.Fatalf("expected internal fields to be allowed outside user question, got %v", err)
	}
}

func TestOperationSelectionValidatorSurfacesInputShapeErrors(t *testing.T) {
	input := validOperationSelectionInput()
	input.RegistryCandidates = nil
	plan := validOperationSelectionPlan()

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "invalid selection input:")
	assertOperationSelectionValidationErrorContains(t, err, "registry_candidates must be an array")
}

func TestOperationSelectionValidatorSurfacesPlanShapeErrors(t *testing.T) {
	input := validOperationSelectionInput()
	plan := validOperationSelectionPlan()
	plan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategy("semantic_guess")

	err := NewOperationSelectionValidator().Validate(input, plan)

	assertOperationSelectionValidationErrorContains(t, err, "invalid selection plan:")
	assertOperationSelectionValidationErrorContains(t, err, `selected_operations[0].input_strategy is unsupported: "semantic_guess"`)
}

func validOperationSelectionInput() entities.OperationSelectionInput {
	readonly := true

	return entities.OperationSelectionInput{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		BusinessRequest: entities.BusinessRequest{
			RequestID:              "request-1",
			Marketplace:            "wildberries",
			NaturalLanguageRequest: "Покажи товары",
		},
		RegistryCandidates: []entities.OperationSelectionCandidate{
			{
				OperationID:  "operation_cards",
				SourceFile:   "products.yaml",
				Method:       "POST",
				ServerURL:    "https://content-api.wildberries.ru",
				PathTemplate: "/content/v2/get/cards/list",
				Tags:         []string{"Карточки товаров"},
				Readonly:     &readonly,
				RiskLevel:    "read",
				XTokenTypes:  []string{},
			},
			{
				OperationID:  "operation_stocks",
				SourceFile:   "products.yaml",
				Method:       "POST",
				ServerURL:    "https://marketplace-api.wildberries.ru",
				PathTemplate: "/api/v3/stocks/{warehouseId}",
				Tags:         []string{"Остатки"},
				Readonly:     &readonly,
				RiskLevel:    "read",
				XTokenTypes:  []string{},
			},
		},
		Policies: entities.OperationSelectionPolicies{
			NoSecrets:       true,
			NoHTTPExecution: true,
			RegistryOnly:    true,
		},
	}
}

func validOperationSelectionPlan() entities.OperationSelectionPlan {
	return entities.OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        entities.OperationSelectionStatusReadyForComposition,
		SelectedOperations: []entities.SelectedOperation{
			{
				OperationID:   "operation_cards",
				Purpose:       "Fetch product cards.",
				DependsOn:     []string{},
				InputStrategy: entities.OperationInputStrategyStaticDefaults,
			},
		},
		MissingInputs:      []entities.MissingBusinessInput{},
		RejectedCandidates: []entities.RejectedOperationCandidate{},
		Warnings:           []entities.PlanWarning{},
	}
}

func assertOperationSelectionValidationErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	var validationError OperationSelectionValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected OperationSelectionValidationError, got %T: %v", err, err)
	}

	for _, message := range validationError.Errors {
		if strings.Contains(message, expected) {
			return
		}
	}

	t.Fatalf("expected validation error containing %q, got %v", expected, validationError.Errors)
}
