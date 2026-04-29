package entities

import "fmt"

// PURPOSE: Defines the deterministic composer input after operation selection has been validated.
type ApiPlanCompositionInput struct {
	SchemaVersion              string                 `json:"schema_version"`
	RequestID                  string                 `json:"request_id"`
	Marketplace                string                 `json:"marketplace"`
	BusinessRequest            BusinessRequest        `json:"business_request"`
	SelectionPlan              OperationSelectionPlan `json:"selection_plan"`
	SelectedRegistryOperations []WBRegistryOperation  `json:"selected_registry_operations"`
	Metadata                   *RequestMetadata       `json:"metadata,omitempty"`
}

func NewApiPlanCompositionInput(
	request BusinessRequest,
	selectionPlan OperationSelectionPlan,
	selectedRegistryOperations []WBRegistryOperation,
) ApiPlanCompositionInput {
	return ApiPlanCompositionInput{
		SchemaVersion:              "1.0",
		RequestID:                  request.RequestID,
		Marketplace:                request.Marketplace,
		BusinessRequest:            request,
		SelectionPlan:              selectionPlan,
		SelectedRegistryOperations: selectedRegistryOperations,
		Metadata:                   request.Metadata,
	}
}

func (i ApiPlanCompositionInput) ValidateShape() error {
	errors := i.ShapeErrors()
	if len(errors) > 0 {
		return ApiPlanCompositionInputShapeValidationError{Errors: errors}
	}

	return nil
}

func (i ApiPlanCompositionInput) ShapeErrors() []string {
	errors := make([]string, 0)

	if i.SchemaVersion != "1.0" {
		errors = append(errors, "schema_version must be 1.0")
	}
	if i.RequestID == "" {
		errors = append(errors, "request_id is empty")
	}
	if i.Marketplace != "wildberries" {
		errors = append(errors, "marketplace must be wildberries")
	}
	if i.BusinessRequest.RequestID == "" {
		errors = append(errors, "business_request.request_id is empty")
	}
	if i.BusinessRequest.RequestID != "" && i.RequestID != i.BusinessRequest.RequestID {
		errors = append(errors, fmt.Sprintf("request_id mismatch: input=%s business_request=%s", i.RequestID, i.BusinessRequest.RequestID))
	}
	if i.BusinessRequest.Marketplace != "" && i.Marketplace != i.BusinessRequest.Marketplace {
		errors = append(errors, fmt.Sprintf("marketplace mismatch: input=%s business_request=%s", i.Marketplace, i.BusinessRequest.Marketplace))
	}

	if err := i.SelectionPlan.ValidateShape(); err != nil {
		errors = append(errors, fmt.Sprintf("invalid selection_plan: %v", err))
	}

	if i.SelectionPlan.Status != OperationSelectionStatusReadyForComposition {
		errors = append(errors, fmt.Sprintf("selection_plan.status must be ready_for_composition, got %q", i.SelectionPlan.Status))
	}
	if i.SelectionPlan.RequestID != "" && i.RequestID != i.SelectionPlan.RequestID {
		errors = append(errors, fmt.Sprintf("request_id mismatch: input=%s selection_plan=%s", i.RequestID, i.SelectionPlan.RequestID))
	}
	if i.SelectionPlan.Marketplace != "" && i.Marketplace != i.SelectionPlan.Marketplace {
		errors = append(errors, fmt.Sprintf("marketplace mismatch: input=%s selection_plan=%s", i.Marketplace, i.SelectionPlan.Marketplace))
	}

	if i.SelectedRegistryOperations == nil {
		errors = append(errors, "selected_registry_operations must be an array")
	}
	if len(i.SelectedRegistryOperations) == 0 {
		errors = append(errors, "selected_registry_operations must contain at least one operation")
	}

	errors = append(errors, validateCompositionSelectedRegistryOperations(i)...)

	return errors
}

// PURPOSE: Reports deterministic composer-input contract failures before ApiExecutionPlan construction.
type ApiPlanCompositionInputShapeValidationError struct {
	Errors []string
}

func (e ApiPlanCompositionInputShapeValidationError) Error() string {
	return fmt.Sprintf("api plan composition input shape validation failed: %v", e.Errors)
}

func validateCompositionSelectedRegistryOperations(input ApiPlanCompositionInput) []string {
	errors := make([]string, 0)
	selectedIDs := compositionSelectedOperationIDs(input.SelectionPlan.SelectedOperations)
	registryIDs := make(map[string]bool, len(input.SelectedRegistryOperations))

	for index, operation := range input.SelectedRegistryOperations {
		errors = append(errors, validateCompositionRegistryOperationShape(index, operation)...)

		if operation.OperationID == "" {
			continue
		}

		if registryIDs[operation.OperationID] {
			errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id %s is duplicated", index, operation.OperationID))
			continue
		}
		registryIDs[operation.OperationID] = true

		if !selectedIDs[operation.OperationID] {
			errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id %s is not selected by selection_plan", index, operation.OperationID))
		}

		if input.BusinessRequest.Constraints.ReadonlyOnly {
			if operation.XReadonlyMethod == nil {
				errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id %s has unknown readonly policy", index, operation.OperationID))
			} else if !*operation.XReadonlyMethod {
				errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id %s violates readonly_only policy", index, operation.OperationID))
			}
		}

		if input.BusinessRequest.Constraints.NoJamSubscription && operation.RequiresJam {
			errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id %s violates no_jam_subscription policy", index, operation.OperationID))
		}
	}

	for selectedID := range selectedIDs {
		if !registryIDs[selectedID] {
			errors = append(errors, fmt.Sprintf("selection_plan selected operation %s is missing from selected_registry_operations", selectedID))
		}
	}

	return errors
}

func validateCompositionRegistryOperationShape(index int, operation WBRegistryOperation) []string {
	errors := make([]string, 0)

	if operation.Marketplace != "wildberries" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].marketplace must be wildberries", index))
	}
	if operation.OperationID == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].operation_id is empty", index))
	}
	if operation.SourceFile == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].source_file is empty", index))
	}
	if operation.Method == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].method is empty", index))
	}
	if operation.ServerURL == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].server_url is empty", index))
	}
	if operation.PathTemplate == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].path_template is empty", index))
	}
	if operation.Tags == nil {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].tags must be an array", index))
	}
	if operation.XTokenTypes == nil {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].x_token_types must be an array", index))
	}
	if operation.PathParamsSchemaJSON == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].path_params_schema_json is empty", index))
	}
	if operation.QueryParamsSchemaJSON == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].query_params_schema_json is empty", index))
	}
	if operation.HeadersSchemaJSON == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].headers_schema_json is empty", index))
	}
	if operation.RequestBodySchemaJSON == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].request_body_schema_json is empty", index))
	}
	if operation.ResponseSchemaJSON == "" {
		errors = append(errors, fmt.Sprintf("selected_registry_operations[%d].response_schema_json is empty", index))
	}

	return errors
}

func compositionSelectedOperationIDs(selectedOperations []SelectedOperation) map[string]bool {
	result := make(map[string]bool, len(selectedOperations))

	for _, operation := range selectedOperations {
		if operation.OperationID != "" {
			result[operation.OperationID] = true
		}
	}

	return result
}
