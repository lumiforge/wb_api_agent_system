package wb_api_agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Protects composer-layer boundaries before executable plan construction is implemented.
func TestRegistryApiPlanComposerImplementsDomainInterface(t *testing.T) {
	var _ planning.ApiPlanComposer = (*RegistryApiPlanComposer)(nil)
}

func TestRegistryApiPlanComposerRejectsInvalidInput(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectedRegistryOperations = nil

	plan, err := composer.Compose(context.Background(), input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	assertErrorContains(t, err, "invalid api plan composition input")
	assertErrorContains(t, err, "selected_registry_operations must be an array")
}

func TestRegistryApiPlanComposerComposesSingleReadonlyNoInputOperation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}
	if plan != nil {
		assertComposedSingleOperationPlan(t, input, plan)
		return
	}

	t.Fatalf("expected composed plan, got nil")
}

func TestRegistryApiPlanComposerComposesRequiredChrtIDsBodyFieldFromBusinessEntity(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectionPlan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategyBusinessEntities
	input.BusinessRequest.Entities = map[string]any{
		"chrt_ids": []any{float64(12345678), float64(87654321)},
	}
	input.SelectedRegistryOperations[0].RequestBodySchemaJSON = `{
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

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	chrtIDs, ok := plan.Inputs["chrt_ids"].Value.([]int)
	if !ok {
		t.Fatalf("expected chrt_ids input value to be []int, got %#v", plan.Inputs["chrt_ids"].Value)
	}
	if len(chrtIDs) != 2 || chrtIDs[0] != 12345678 || chrtIDs[1] != 87654321 {
		t.Fatalf("unexpected chrt_ids value %#v", chrtIDs)
	}

	body, ok := plan.Steps[0].Request.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected request body map, got %#v", plan.Steps[0].Request.Body)
	}

	binding, ok := body["chrtIds"].(entities.ValueBinding)
	if !ok {
		t.Fatalf("expected chrtIds body binding, got %#v", body["chrtIds"])
	}
	if binding.Source != "input" || binding.InputName != "chrt_ids" || !binding.Required {
		t.Fatalf("expected chrtIds source=input binding, got %#v", binding)
	}
}

func TestRegistryApiPlanComposerRejectsRequiredPathParams(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectedRegistryOperations[0].PathTemplate = "/api/v3/stocks/{warehouseId}"
	input.SelectedRegistryOperations[0].PathParamsSchemaJSON = `{"warehouseId":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	assertUnsupportedCompositionError(t, err, "required_path_param_value_missing")
}

func TestRegistryApiPlanComposerComposesRequiredDateFromQueryParamFromBusinessPeriod(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.BusinessRequest.Period = &entities.Period{
		From: "2026-04-28",
	}
	input.SelectedRegistryOperations[0].QueryParamsSchemaJSON = `{"dateFrom":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)

	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	if plan.Inputs["date_from"].Value != "2026-04-28" {
		t.Fatalf("expected date_from input value 2026-04-28, got %#v", plan.Inputs["date_from"])
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}

	binding := plan.Steps[0].Request.QueryParams["dateFrom"]
	if binding.Source != "input" {
		t.Fatalf("expected dateFrom source=input, got %#v", binding)
	}
	if binding.InputName != "date_from" {
		t.Fatalf("expected dateFrom input_name=date_from, got %#v", binding)
	}
	if !binding.Required {
		t.Fatalf("expected dateFrom binding to be required")
	}
}

func TestRegistryApiPlanComposerRejectsRequiredQueryParamWithoutExplicitBusinessValue(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectedRegistryOperations[0].QueryParamsSchemaJSON = `{"dateFrom":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	assertUnsupportedCompositionError(t, err, "required_query_param_value_missing")

}

func TestRegistryApiPlanComposerRejectsRequiredRequestBody(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectedRegistryOperations[0].RequestBodySchemaJSON = `{
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

	plan, err := composer.Compose(context.Background(), input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	assertUnsupportedCompositionError(t, err, "required_request_body_value_missing")
}

func TestRegistryApiPlanComposerRejectsStepOutputInputStrategy(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectionPlan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategyStepOutput

	plan, err := composer.Compose(context.Background(), input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	assertUnsupportedCompositionError(t, err, "step_output_input_strategy_not_supported")

}

func TestRegistryApiPlanComposerReturnsContextError(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	plan, err := composer.Compose(ctx, input)

	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %T: %v", err, err)
	}
}

func TestRegistryApiPlanComposerDateFromOutputPassesPlanPostProcessorValidation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.BusinessRequest.Period = &entities.Period{
		From: "2026-04-28",
	}
	input.SelectedRegistryOperations[0].QueryParamsSchemaJSON = `{"dateFrom":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	registry := &singleOperationRegistry{
		operation: input.SelectedRegistryOperations[0],
	}

	processor := NewPlanPostProcessor(registry)

	validationPlan, err := processor.Process(context.Background(), input.BusinessRequest, plan)
	if err != nil {
		t.Fatalf("expected post processor success, got %v", err)
	}

	if validationPlan != nil {
		t.Fatalf("expected no replacement validation plan, got %#v", validationPlan)
	}
}

func assertComposedSingleOperationPlan(
	t *testing.T,
	input entities.ApiPlanCompositionInput,
	plan *entities.ApiExecutionPlan,
) {
	t.Helper()

	if plan.SchemaVersion != "1.0" {
		t.Fatalf("expected schema_version 1.0, got %q", plan.SchemaVersion)
	}
	if plan.RequestID != input.RequestID {
		t.Fatalf("expected request_id %q, got %q", input.RequestID, plan.RequestID)
	}
	if plan.Status != "ready" {
		t.Fatalf("expected ready status, got %q", plan.Status)
	}
	if plan.ExecutionMode != "automatic" {
		t.Fatalf("expected automatic execution mode, got %q", plan.ExecutionMode)
	}
	if len(plan.Inputs) != 0 {
		t.Fatalf("expected no inputs, got %#v", plan.Inputs)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}

	step := plan.Steps[0]
	operation := input.SelectedRegistryOperations[0]

	if step.StepID != "operation_cards" {
		t.Fatalf("expected stable step id operation_cards, got %q", step.StepID)
	}
	if step.OperationID != operation.OperationID {
		t.Fatalf("expected operation_id %q, got %q", operation.OperationID, step.OperationID)
	}
	if step.Request.Method != operation.Method {
		t.Fatalf("expected method %q, got %q", operation.Method, step.Request.Method)
	}
	if step.Request.ServerURL != operation.ServerURL {
		t.Fatalf("expected server_url %q, got %q", operation.ServerURL, step.Request.ServerURL)
	}
	if step.Request.PathTemplate != operation.PathTemplate {
		t.Fatalf("expected path_template %q, got %q", operation.PathTemplate, step.Request.PathTemplate)
	}
	if step.Request.Headers["Authorization"].Source != "executor_secret" {
		t.Fatalf("expected executor_secret Authorization, got %#v", step.Request.Headers["Authorization"])
	}
	if step.ResponseMapping.Outputs == nil || len(step.ResponseMapping.Outputs) == 0 {
		t.Fatalf("expected response mapping outputs, got %#v", step.ResponseMapping.Outputs)
	}
	if len(plan.FinalOutput.Fields) == 0 {
		t.Fatalf("expected final output fields, got %#v", plan.FinalOutput.Fields)
	}
}

func assertUnsupportedCompositionError(t *testing.T, err error, expectedReason string) {
	t.Helper()

	var unsupportedError ApiPlanCompositionUnsupportedError
	if !errors.As(err, &unsupportedError) {
		t.Fatalf("expected ApiPlanCompositionUnsupportedError, got %T: %v", err, err)
	}

	if unsupportedError.Code != CompositionUnsupportedCode {
		t.Fatalf("expected code %q, got %q", CompositionUnsupportedCode, unsupportedError.Code)
	}

	if unsupportedError.Reason != expectedReason {
		t.Fatalf("expected reason %q, got %q", expectedReason, unsupportedError.Reason)
	}
}

func validComposerInput() entities.ApiPlanCompositionInput {
	readonly := true

	return entities.ApiPlanCompositionInput{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		BusinessRequest: entities.BusinessRequest{
			RequestID:              "request-1",
			Marketplace:            "wildberries",
			NaturalLanguageRequest: "Покажи товары",
			Constraints: entities.BusinessConstraints{
				ReadonlyOnly:      true,
				NoJamSubscription: true,
			},
		},
		SelectionPlan: entities.OperationSelectionPlan{
			SchemaVersion: "1.0",
			RequestID:     "request-1",
			Marketplace:   "wildberries",
			Status:        entities.OperationSelectionStatusReadyForComposition,
			SelectedOperations: []entities.SelectedOperation{
				{
					OperationID:   "operation_cards",
					Purpose:       "Fetch product cards.",
					DependsOn:     []string{},
					InputStrategy: entities.OperationInputStrategyNoUserInput,
				},
			},
			MissingInputs:      []entities.MissingBusinessInput{},
			RejectedCandidates: []entities.RejectedOperationCandidate{},
			Warnings:           []entities.PlanWarning{},
		},
		SelectedRegistryOperations: []entities.WBRegistryOperation{
			{
				Marketplace:              "wildberries",
				SourceFile:               "products.yaml",
				OperationID:              "operation_cards",
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
				RequestBodySchemaJSON:    `{"content":{"application/json":{"schema":{"type":"object","properties":{}}}}}`,
				ResponseSchemaJSON:       "{}",
				RateLimitNotes:           "",
				SubscriptionRequirements: "",
				RequiresJam:              false,
			},
		},
	}
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", expected)
	}

	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error %q to contain %q", err.Error(), expected)
	}
}

func TestRegistryApiPlanComposerOutputPassesPlanPostProcessorValidation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	registry := &singleOperationRegistry{
		operation: input.SelectedRegistryOperations[0],
	}

	processor := NewPlanPostProcessor(registry)

	validationPlan, err := processor.Process(context.Background(), input.BusinessRequest, plan)
	if err != nil {
		t.Fatalf("expected post processor success, got %v", err)
	}

	if validationPlan != nil {
		t.Fatalf("expected no replacement validation plan, got %#v", validationPlan)
	}

	if !plan.Validation.RegistryChecked {
		t.Fatalf("expected registry validation to be checked")
	}
	if !plan.Validation.SecretsPolicyChecked {
		t.Fatalf("expected secrets policy validation to be checked")
	}
}

type singleOperationRegistry struct {
	operation entities.WBRegistryOperation
}

func (r *singleOperationRegistry) SearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	return []entities.WBRegistryOperation{r.operation}, nil
}

func (r *singleOperationRegistry) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	if operationID != r.operation.OperationID {
		return nil, nil
	}

	operation := r.operation

	return &operation, nil
}

func (r *singleOperationRegistry) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{
		Total: 1,
		Read:  1,
	}, nil
}

func TestRegistryApiPlanComposerComposesRequiredWarehouseIDPathParamFromBusinessEntity(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.BusinessRequest.Entities = map[string]any{
		"warehouse_id": "507",
	}
	input.SelectedRegistryOperations[0].PathTemplate = "/api/v3/stocks/{warehouseId}"
	input.SelectedRegistryOperations[0].PathParamsSchemaJSON = `{"warehouseId":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	if plan.Inputs["warehouse_id"].Value != 507 {
		t.Fatalf("expected warehouse_id input value 507, got %#v", plan.Inputs["warehouse_id"])
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}

	binding := plan.Steps[0].Request.PathParams["warehouseId"]
	if binding.Source != "input" {
		t.Fatalf("expected warehouseId source=input, got %#v", binding)
	}
	if binding.InputName != "warehouse_id" {
		t.Fatalf("expected warehouseId input_name=warehouse_id, got %#v", binding)
	}
	if !binding.Required {
		t.Fatalf("expected warehouseId binding to be required")
	}
}

func TestRegistryApiPlanComposerWarehouseIDOutputPassesPlanPostProcessorValidation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.BusinessRequest.Entities = map[string]any{
		"warehouse_id": 507,
	}
	input.SelectedRegistryOperations[0].PathTemplate = "/api/v3/stocks/{warehouseId}"
	input.SelectedRegistryOperations[0].PathParamsSchemaJSON = `{"warehouseId":{"required":true}}`

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	registry := &singleOperationRegistry{
		operation: input.SelectedRegistryOperations[0],
	}

	processor := NewPlanPostProcessor(registry)

	validationPlan, err := processor.Process(context.Background(), input.BusinessRequest, plan)
	if err != nil {
		t.Fatalf("expected post processor success, got %v", err)
	}

	if validationPlan != nil {
		t.Fatalf("expected no replacement validation plan, got %#v", validationPlan)
	}
}

func TestRegistryApiPlanComposerChrtIDsOutputPassesPlanPostProcessorValidation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectionPlan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategyBusinessEntities
	input.BusinessRequest.Entities = map[string]any{
		"chrt_ids": []int{12345678, 87654321},
	}
	input.SelectedRegistryOperations[0].RequestBodySchemaJSON = `{
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

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	registry := &singleOperationRegistry{
		operation: input.SelectedRegistryOperations[0],
	}

	processor := NewPlanPostProcessor(registry)

	validationPlan, err := processor.Process(context.Background(), input.BusinessRequest, plan)
	if err != nil {
		t.Fatalf("expected post processor success, got %v", err)
	}

	if validationPlan != nil {
		t.Fatalf("expected no replacement validation plan, got %#v", validationPlan)
	}
}

func TestRegistryApiPlanComposerComposesWarehouseIDPathParamAndChrtIDsBodyField(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectionPlan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategyBusinessEntities
	input.BusinessRequest.Entities = map[string]any{
		"warehouse_id": "507",
		"chrt_ids":     []any{float64(12345678), float64(87654321)},
	}
	input.SelectedRegistryOperations[0].PathTemplate = "/api/v3/stocks/{warehouseId}"
	input.SelectedRegistryOperations[0].PathParamsSchemaJSON = `{"warehouseId":{"required":true}}`
	input.SelectedRegistryOperations[0].RequestBodySchemaJSON = `{
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

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	if plan.Inputs["warehouse_id"].Value != 507 {
		t.Fatalf("expected warehouse_id input value 507, got %#v", plan.Inputs["warehouse_id"])
	}

	chrtIDs, ok := plan.Inputs["chrt_ids"].Value.([]int)
	if !ok {
		t.Fatalf("expected chrt_ids input value to be []int, got %#v", plan.Inputs["chrt_ids"].Value)
	}
	if len(chrtIDs) != 2 || chrtIDs[0] != 12345678 || chrtIDs[1] != 87654321 {
		t.Fatalf("unexpected chrt_ids value %#v", chrtIDs)
	}

	step := plan.Steps[0]

	pathBinding := step.Request.PathParams["warehouseId"]
	if pathBinding.Source != "input" || pathBinding.InputName != "warehouse_id" || !pathBinding.Required {
		t.Fatalf("expected warehouseId source=input binding, got %#v", pathBinding)
	}

	body, ok := step.Request.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected request body map, got %#v", step.Request.Body)
	}

	bodyBinding, ok := body["chrtIds"].(entities.ValueBinding)
	if !ok {
		t.Fatalf("expected chrtIds body binding, got %#v", body["chrtIds"])
	}
	if bodyBinding.Source != "input" || bodyBinding.InputName != "chrt_ids" || !bodyBinding.Required {
		t.Fatalf("expected chrtIds source=input binding, got %#v", bodyBinding)
	}
}

func TestRegistryApiPlanComposerWarehouseIDAndChrtIDsOutputPassesPlanPostProcessorValidation(t *testing.T) {
	composer := NewRegistryApiPlanComposer()
	input := validComposerInput()
	input.SelectionPlan.SelectedOperations[0].InputStrategy = entities.OperationInputStrategyBusinessEntities
	input.BusinessRequest.Entities = map[string]any{
		"warehouse_id": 507,
		"chrt_ids":     []int{12345678, 87654321},
	}
	input.SelectedRegistryOperations[0].PathTemplate = "/api/v3/stocks/{warehouseId}"
	input.SelectedRegistryOperations[0].PathParamsSchemaJSON = `{"warehouseId":{"required":true}}`
	input.SelectedRegistryOperations[0].RequestBodySchemaJSON = `{
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

	plan, err := composer.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("expected composed plan, got %v", err)
	}

	registry := &singleOperationRegistry{
		operation: input.SelectedRegistryOperations[0],
	}

	processor := NewPlanPostProcessor(registry)

	validationPlan, err := processor.Process(context.Background(), input.BusinessRequest, plan)
	if err != nil {
		t.Fatalf("expected post processor success, got %v", err)
	}

	if validationPlan != nil {
		t.Fatalf("expected no replacement validation plan, got %#v", validationPlan)
	}
}
