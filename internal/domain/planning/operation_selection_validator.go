package planning

import (
	"fmt"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Validates selector output against retrieval candidates before deterministic composition.
type OperationSelectionValidator struct{}

func NewOperationSelectionValidator() *OperationSelectionValidator {
	return &OperationSelectionValidator{}
}

func (v *OperationSelectionValidator) Validate(
	input entities.OperationSelectionInput,
	plan entities.OperationSelectionPlan,
) error {
	errors := make([]string, 0)

	if err := input.ValidateShape(); err != nil {
		errors = append(errors, fmt.Sprintf("invalid selection input: %v", err))
	}

	if err := plan.ValidateShape(); err != nil {
		errors = append(errors, fmt.Sprintf("invalid selection plan: %v", err))
	}

	errors = append(errors, validateSelectionRequestIdentity(input, plan)...)
	errors = append(errors, validateSelectedOperationsExistInCandidates(input, plan)...)
	errors = append(errors, validateRejectedCandidatesExistInCandidates(input, plan)...)
	errors = append(errors, validateSelectedOperationDependencies(plan)...)
	errors = append(errors, validateMissingInputTechnicalLeakage(plan)...)

	if len(errors) > 0 {
		return OperationSelectionValidationError{Errors: errors}
	}

	return nil
}

// PURPOSE: Reports selector-output boundary violations without changing or interpreting the selected plan.
type OperationSelectionValidationError struct {
	Errors []string
}

func (e OperationSelectionValidationError) Error() string {
	return fmt.Sprintf("operation selection validation failed: %v", e.Errors)
}

func validateSelectionRequestIdentity(
	input entities.OperationSelectionInput,
	plan entities.OperationSelectionPlan,
) []string {
	errors := make([]string, 0)

	if plan.SchemaVersion != input.SchemaVersion {
		errors = append(errors, fmt.Sprintf("schema_version mismatch: input=%s plan=%s", input.SchemaVersion, plan.SchemaVersion))
	}
	if plan.RequestID != input.RequestID {
		errors = append(errors, fmt.Sprintf("request_id mismatch: input=%s plan=%s", input.RequestID, plan.RequestID))
	}
	if plan.Marketplace != input.Marketplace {
		errors = append(errors, fmt.Sprintf("marketplace mismatch: input=%s plan=%s", input.Marketplace, plan.Marketplace))
	}

	return errors
}

func validateSelectedOperationsExistInCandidates(
	input entities.OperationSelectionInput,
	plan entities.OperationSelectionPlan,
) []string {
	candidateIDs := operationSelectionCandidateIDs(input.RegistryCandidates)
	seenSelectedIDs := make(map[string]bool)
	errors := make([]string, 0)

	for index, selected := range plan.SelectedOperations {
		if selected.OperationID == "" {
			continue
		}

		if !candidateIDs[selected.OperationID] {
			errors = append(errors, fmt.Sprintf("selected_operations[%d].operation_id %s is not present in registry_candidates", index, selected.OperationID))
			continue
		}

		if seenSelectedIDs[selected.OperationID] {
			errors = append(errors, fmt.Sprintf("selected_operations[%d].operation_id %s is duplicated", index, selected.OperationID))
			continue
		}

		seenSelectedIDs[selected.OperationID] = true
	}

	return errors
}

func validateRejectedCandidatesExistInCandidates(
	input entities.OperationSelectionInput,
	plan entities.OperationSelectionPlan,
) []string {
	candidateIDs := operationSelectionCandidateIDs(input.RegistryCandidates)
	errors := make([]string, 0)

	for index, rejected := range plan.RejectedCandidates {
		if rejected.OperationID == "" {
			continue
		}

		if !candidateIDs[rejected.OperationID] {
			errors = append(errors, fmt.Sprintf("rejected_candidates[%d].operation_id %s is not present in registry_candidates", index, rejected.OperationID))
		}
	}

	return errors
}

func validateSelectedOperationDependencies(plan entities.OperationSelectionPlan) []string {
	selectedIDs := make(map[string]bool)
	errors := make([]string, 0)

	for _, selected := range plan.SelectedOperations {
		if selected.OperationID != "" {
			selectedIDs[selected.OperationID] = true
		}
	}

	for index, selected := range plan.SelectedOperations {
		for dependencyIndex, dependencyID := range selected.DependsOn {
			if dependencyID == "" {
				errors = append(errors, fmt.Sprintf("selected_operations[%d].depends_on[%d] is empty", index, dependencyIndex))
				continue
			}

			if dependencyID == selected.OperationID {
				errors = append(errors, fmt.Sprintf("selected_operations[%d].depends_on[%d] self-references operation %s", index, dependencyIndex, dependencyID))
				continue
			}

			if !selectedIDs[dependencyID] {
				errors = append(errors, fmt.Sprintf("selected_operations[%d].depends_on[%d] references unselected operation %s", index, dependencyIndex, dependencyID))
			}
		}
	}

	return errors
}

func validateMissingInputTechnicalLeakage(plan entities.OperationSelectionPlan) []string {
	errors := make([]string, 0)

	for index, input := range plan.MissingInputs {
		if containsTechnicalFieldName(input.UserQuestion, input.InternalFields) {
			errors = append(errors, fmt.Sprintf("missing_inputs[%d].user_question exposes internal field name", index))
		}
	}

	return errors
}

func containsTechnicalFieldName(value string, internalFields []string) bool {
	for _, internalField := range internalFields {
		if internalField == "" {
			continue
		}

		if containsExactSubstring(value, internalField) {
			return true
		}
	}

	return false
}

func containsExactSubstring(value string, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}

	return false
}

func operationSelectionCandidateIDs(candidates []entities.OperationSelectionCandidate) map[string]bool {
	result := make(map[string]bool, len(candidates))

	for _, candidate := range candidates {
		if candidate.OperationID != "" {
			result[candidate.OperationID] = true
		}
	}

	return result
}
