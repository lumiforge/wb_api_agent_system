package wb_api_agent

import (
	"fmt"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

const OperationSelectionRegistryResolutionErrorCode = "operation_selection_registry_resolution_failed"

// PURPOSE: Resolves selector-chosen operation IDs to exact registry operations without semantic fallback.
type OperationSelectionRegistryResolver struct{}

func NewOperationSelectionRegistryResolver() *OperationSelectionRegistryResolver {
	return &OperationSelectionRegistryResolver{}
}

func (r *OperationSelectionRegistryResolver) Resolve(
	selectionPlan entities.OperationSelectionPlan,
	candidates []entities.WBRegistryOperation,
) ([]entities.WBRegistryOperation, error) {
	candidateByID := make(map[string]entities.WBRegistryOperation, len(candidates))
	for _, candidate := range candidates {
		if candidate.OperationID == "" {
			continue
		}

		// WHY: Registry candidates are source-of-truth; duplicate IDs would make deterministic composition ambiguous.
		if _, exists := candidateByID[candidate.OperationID]; exists {
			return nil, NewOperationSelectionRegistryResolutionError(
				"duplicate_candidate_operation_id",
				fmt.Sprintf("candidate operation_id %s appears more than once", candidate.OperationID),
			)
		}

		candidateByID[candidate.OperationID] = candidate
	}

	resolved := make([]entities.WBRegistryOperation, 0, len(selectionPlan.SelectedOperations))
	seenSelected := make(map[string]bool, len(selectionPlan.SelectedOperations))

	for index, selected := range selectionPlan.SelectedOperations {
		if selected.OperationID == "" {
			return nil, NewOperationSelectionRegistryResolutionError(
				"empty_selected_operation_id",
				fmt.Sprintf("selected_operations[%d].operation_id is empty", index),
			)
		}

		if seenSelected[selected.OperationID] {
			return nil, NewOperationSelectionRegistryResolutionError(
				"duplicate_selected_operation_id",
				fmt.Sprintf("selected operation_id %s appears more than once", selected.OperationID),
			)
		}
		seenSelected[selected.OperationID] = true

		candidate, ok := candidateByID[selected.OperationID]
		if !ok {
			return nil, NewOperationSelectionRegistryResolutionError(
				"selected_operation_not_in_candidates",
				fmt.Sprintf("selected operation_id %s is not present in registry candidates", selected.OperationID),
			)
		}

		resolved = append(resolved, candidate)
	}

	return resolved, nil
}

// PURPOSE: Reports deterministic failures while resolving selector output to registry source-of-truth records.
type OperationSelectionRegistryResolutionError struct {
	Code    string
	Reason  string
	Message string
}

func NewOperationSelectionRegistryResolutionError(
	reason string,
	message string,
) OperationSelectionRegistryResolutionError {
	return OperationSelectionRegistryResolutionError{
		Code:    OperationSelectionRegistryResolutionErrorCode,
		Reason:  reason,
		Message: message,
	}
}

func (e OperationSelectionRegistryResolutionError) Error() string {
	return fmt.Sprintf("%s: reason=%s message=%s", e.Code, e.Reason, e.Message)
}
