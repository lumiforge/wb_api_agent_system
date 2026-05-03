package composer

import (
	"context"
	"fmt"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
)

var _ planning.ApiPlanComposer = (*RegistryApiPlanComposer)(nil)

// PURPOSE: Owns deterministic ApiExecutionPlan construction from validated registry metadata.
type RegistryApiPlanComposer struct{}

func NewRegistryApiPlanComposer() *RegistryApiPlanComposer {
	return &RegistryApiPlanComposer{}
}

func (c *RegistryApiPlanComposer) Compose(
	ctx context.Context,
	input entities.ApiPlanCompositionInput,
) (*entities.ApiExecutionPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("compose api plan context: %w", err)
	}

	if err := input.ValidateShape(); err != nil {
		return nil, fmt.Errorf("invalid api plan composition input: %w", err)
	}

	if err := validateSingleOperationCompositionSupport(input); err != nil {
		return nil, err
	}

	plan := composeSingleOperationPlan(input)

	return plan, nil
}

func validateSingleOperationCompositionSupport(input entities.ApiPlanCompositionInput) error {
	if len(input.SelectionPlan.SelectedOperations) != 1 {
		return NewApiPlanCompositionUnsupportedError(
			input.RequestID,
			"multi_operation_composition_not_supported",
			"deterministic composer currently supports exactly one selected operation",
		)
	}

	if len(input.SelectedRegistryOperations) != 1 {
		return NewApiPlanCompositionUnsupportedError(
			input.RequestID,
			"multi_registry_operation_composition_not_supported",
			"deterministic composer currently supports exactly one selected registry operation",
		)
	}

	selectedOperation := input.SelectionPlan.SelectedOperations[0]
	if selectedOperation.InputStrategy == entities.OperationInputStrategyStepOutput {
		return NewApiPlanCompositionUnsupportedError(
			input.RequestID,
			"step_output_input_strategy_not_supported",
			"step_output input strategy requires multi-step composition",
		)
	}

	operation := input.SelectedRegistryOperations[0]
	if operation.XReadonlyMethod == nil || !*operation.XReadonlyMethod {
		return NewApiPlanCompositionUnsupportedError(
			input.RequestID,
			"readonly_operation_required",
			"single-operation composition supports only registry-confirmed readonly operations",
		)
	}

	for _, pathParam := range requiredPathParams(operation) {
		if _, _, ok := pathParamBindingFromBusinessRequest(pathParam, input.BusinessRequest); !ok {
			return NewApiPlanCompositionUnsupportedError(
				input.RequestID,
				"required_path_param_value_missing",
				"operation requires path param value that is not present as an explicit business request field: "+pathParam,
			)
		}
	}

	for _, queryParam := range requiredSchemaFields(schemaParamNames(operation.QueryParamsSchemaJSON)) {
		if _, _, ok := queryParamBindingFromBusinessRequest(queryParam, input.BusinessRequest); !ok {
			return NewApiPlanCompositionUnsupportedError(
				input.RequestID,
				"required_query_param_value_missing",
				"operation requires query param value that is not present as an explicit business request field: "+queryParam,
			)
		}
	}

	for _, bodyField := range requiredRequestBodyFields(operation.RequestBodySchemaJSON) {
		if _, _, ok := bodyFieldBindingFromBusinessRequest(bodyField, input.BusinessRequest); !ok {
			return NewApiPlanCompositionUnsupportedError(
				input.RequestID,
				"required_request_body_value_missing",
				"operation requires request body field value that is not present as an explicit business request field: "+bodyField,
			)
		}
	}

	if requiredRequestBodyNotComposable(operation.RequestBodySchemaJSON, input.BusinessRequest) {
		return NewApiPlanCompositionUnsupportedError(
			input.RequestID,
			"required_request_body_not_composable",
			"operation requires request body that deterministic composer cannot build from explicit business request fields",
		)
	}

	return nil
}
func requiredRequestBodyNotComposable(requestBodySchemaJSON string, request entities.BusinessRequest) bool {
	if !requestBodyRequired(requestBodySchemaJSON) {
		return false
	}

	return len(bodyBindingsFromBusinessRequest(requestBodySchemaJSON, request)) == 0
}

func composeSingleOperationPlan(input entities.ApiPlanCompositionInput) *entities.ApiExecutionPlan {
	selectedOperation := input.SelectionPlan.SelectedOperations[0]
	operation := input.SelectedRegistryOperations[0]

	inputs := pathInputsFromBusinessRequest(operation, input.BusinessRequest)
	mergeInputValues(inputs, queryInputsFromBusinessRequest(operation.QueryParamsSchemaJSON, input.BusinessRequest))
	mergeInputValues(inputs, bodyInputsFromBusinessRequest(operation.RequestBodySchemaJSON, input.BusinessRequest))

	pathParams := pathParamBindingsFromBusinessRequest(operation, input.BusinessRequest)
	queryParams := queryParamBindingsFromBusinessRequest(operation.QueryParamsSchemaJSON, input.BusinessRequest)
	body := bodyBindingsFromBusinessRequest(operation.RequestBodySchemaJSON, input.BusinessRequest)

	step := entities.ApiPlanStep{
		StepID:      stableStepID(operation.OperationID),
		OperationID: operation.OperationID,
		SourceFile:  operation.SourceFile,
		Readonly:    true,
		RiskLevel:   "read",
		Purpose:     selectedOperation.Purpose,
		DependsOn:   nonNilStringSlice(selectedOperation.DependsOn),
		Request: entities.HttpRequestTemplate{
			ServerURL:    operation.ServerURL,
			Method:       operation.Method,
			PathTemplate: operation.PathTemplate,
			PathParams:   pathParams,
			QueryParams:  queryParams,
			Headers: map[string]entities.HeaderBinding{
				"Authorization": {
					Source:     "executor_secret",
					SecretName: "WB_AUTHORIZATION",
					Required:   true,
				},
			},
			Body:        body,
			ContentType: "application/json",
			Accept:      "application/json",
		},
		Pagination: entities.PaginationPlan{
			Enabled:  false,
			Strategy: "none",
		},
		RetryPolicy:     defaultRetryPolicy(),
		RateLimitPolicy: defaultRateLimitPolicy(operation.OperationID),
		ResponseMapping: entities.ResponseMapping{
			Outputs:     map[string]entities.MappedOutput{},
			PostFilters: []entities.PostFilter{},
		},
	}

	plan := &entities.ApiExecutionPlan{
		SchemaVersion:          "1.0",
		RequestID:              input.RequestID,
		Marketplace:            "wildberries",
		Status:                 "ready",
		Intent:                 input.BusinessRequest.Intent,
		NaturalLanguageSummary: naturalLanguageSummary(input),
		RiskLevel:              "read",
		RequiresApproval:       false,
		ExecutionMode:          executionMode(input.BusinessRequest),
		Inputs:                 inputs,
		Steps:                  []entities.ApiPlanStep{step},
		Transforms:             []entities.TransformStep{},
		FinalOutput: entities.FinalOutput{
			Type:        "object",
			Description: naturalLanguageSummary(input),
			Fields:      map[string]any{},
		},
		Warnings: nonNilWarnings(input.SelectionPlan.Warnings),
		Validation: entities.PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: input.BusinessRequest.Constraints.ReadonlyOnly,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      input.BusinessRequest.Constraints.NoJamSubscription,
			Errors:                []string{},
		},
		Metadata: input.Metadata,
	}

	return plan
}
