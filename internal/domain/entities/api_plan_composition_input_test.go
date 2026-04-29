package entities

import (
	"errors"
	"strings"
	"testing"
)

// PURPOSE: Protects deterministic composer input from selector mistakes before executable plan construction.
func TestApiPlanCompositionInputValidateShapeAcceptsValidInput(t *testing.T) {
	input := validApiPlanCompositionInput()

	if err := input.ValidateShape(); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestApiPlanCompositionInputValidateShapeRejectsNonReadySelection(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectionPlan.Status = OperationSelectionStatusNeedsClarification
	input.SelectionPlan.MissingInputs = []MissingBusinessInput{
		{
			Code:           "warehouse",
			UserQuestion:   "Provide the seller warehouse.",
			Accepts:        []string{"warehouse ID", "warehouse name"},
			InternalFields: []string{"warehouse_id"},
		},
	}

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, `selection_plan.status must be ready_for_composition, got "needs_clarification"`)
}

func TestApiPlanCompositionInputValidateShapeRejectsMissingSelectedRegistryOperation(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectedRegistryOperations = []WBRegistryOperation{}

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations must contain at least one operation")
	assertApiPlanCompositionInputErrorContains(t, err, "selection_plan selected operation operation_cards is missing from selected_registry_operations")
}

func TestApiPlanCompositionInputValidateShapeRejectsNilSelectedRegistryOperations(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectedRegistryOperations = nil

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations must be an array")
}

func TestApiPlanCompositionInputValidateShapeRejectsUnselectedRegistryOperation(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectedRegistryOperations = append(input.SelectedRegistryOperations, validCompositionRegistryOperation("operation_stocks"))

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[1].operation_id operation_stocks is not selected by selection_plan")
}

func TestApiPlanCompositionInputValidateShapeRejectsDuplicateRegistryOperation(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectedRegistryOperations = append(input.SelectedRegistryOperations, validCompositionRegistryOperation("operation_cards"))

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[1].operation_id operation_cards is duplicated")
}

func TestApiPlanCompositionInputValidateShapeRejectsReadonlyPolicyViolation(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.BusinessRequest.Constraints.ReadonlyOnly = true
	write := false
	input.SelectedRegistryOperations[0].XReadonlyMethod = &write

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].operation_id operation_cards violates readonly_only policy")
}

func TestApiPlanCompositionInputValidateShapeRejectsUnknownReadonlyPolicyWhenReadonlyOnly(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.BusinessRequest.Constraints.ReadonlyOnly = true
	input.SelectedRegistryOperations[0].XReadonlyMethod = nil

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].operation_id operation_cards has unknown readonly policy")
}

func TestApiPlanCompositionInputValidateShapeRejectsJamPolicyViolation(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.BusinessRequest.Constraints.NoJamSubscription = true
	input.SelectedRegistryOperations[0].RequiresJam = true

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].operation_id operation_cards violates no_jam_subscription policy")
}

func TestApiPlanCompositionInputValidateShapeRejectsRequestIDMismatch(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectionPlan.RequestID = "other-request"

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "request_id mismatch: input=request-1 selection_plan=other-request")
}

func TestApiPlanCompositionInputValidateShapeRejectsBusinessRequestMismatch(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.BusinessRequest.RequestID = "other-request"

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "request_id mismatch: input=request-1 business_request=other-request")
}

func TestApiPlanCompositionInputValidateShapeRejectsInvalidRegistryOperationShape(t *testing.T) {
	input := validApiPlanCompositionInput()
	input.SelectedRegistryOperations[0] = WBRegistryOperation{}

	err := input.ValidateShape()

	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].marketplace must be wildberries")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].operation_id is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].source_file is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].method is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].server_url is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].path_template is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].tags must be an array")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].x_token_types must be an array")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].path_params_schema_json is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].query_params_schema_json is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].headers_schema_json is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].request_body_schema_json is empty")
	assertApiPlanCompositionInputErrorContains(t, err, "selected_registry_operations[0].response_schema_json is empty")
}

func TestNewApiPlanCompositionInputCopiesRequestIdentity(t *testing.T) {
	request := BusinessRequest{
		RequestID:              "request-1",
		Marketplace:            "wildberries",
		NaturalLanguageRequest: "Покажи товары",
	}
	selectionPlan := validCompositionSelectionPlan()
	operation := validCompositionRegistryOperation("operation_cards")

	input := NewApiPlanCompositionInput(request, selectionPlan, []WBRegistryOperation{operation})

	if input.SchemaVersion != "1.0" {
		t.Fatalf("expected schema_version 1.0, got %q", input.SchemaVersion)
	}
	if input.RequestID != request.RequestID {
		t.Fatalf("expected request_id %q, got %q", request.RequestID, input.RequestID)
	}
	if input.Marketplace != request.Marketplace {
		t.Fatalf("expected marketplace %q, got %q", request.Marketplace, input.Marketplace)
	}
}

func validApiPlanCompositionInput() ApiPlanCompositionInput {
	request := BusinessRequest{
		RequestID:              "request-1",
		Marketplace:            "wildberries",
		NaturalLanguageRequest: "Покажи товары",
		Constraints: BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
		},
	}

	return ApiPlanCompositionInput{
		SchemaVersion:   "1.0",
		RequestID:       "request-1",
		Marketplace:     "wildberries",
		BusinessRequest: request,
		SelectionPlan:   validCompositionSelectionPlan(),
		SelectedRegistryOperations: []WBRegistryOperation{
			validCompositionRegistryOperation("operation_cards"),
		},
	}
}

func validCompositionSelectionPlan() OperationSelectionPlan {
	return OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        OperationSelectionStatusReadyForComposition,
		SelectedOperations: []SelectedOperation{
			{
				OperationID:   "operation_cards",
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

func validCompositionRegistryOperation(operationID string) WBRegistryOperation {
	readonly := true

	return WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "products.yaml",
		OperationID:              operationID,
		Method:                   "POST",
		ServerURL:                "https://content-api.wildberries.ru",
		PathTemplate:             "/content/v2/get/cards/list",
		Tags:                     []string{"Карточки товаров"},
		Category:                 "content",
		Summary:                  "Список карточек товаров",
		Description:              "Метод возвращает список созданных карточек товаров.",
		XReadonlyMethod:          &readonly,
		XCategory:                "content",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        "{}",
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}

func assertApiPlanCompositionInputErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	var shapeError ApiPlanCompositionInputShapeValidationError
	if !errors.As(err, &shapeError) {
		t.Fatalf("expected ApiPlanCompositionInputShapeValidationError, got %T: %v", err, err)
	}

	for _, message := range shapeError.Errors {
		if strings.Contains(message, expected) {
			return
		}
	}

	t.Fatalf("expected shape error containing %q, got %v", expected, shapeError.Errors)
}
