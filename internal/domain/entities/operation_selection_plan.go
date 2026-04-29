package entities

import "fmt"

type OperationSelectionStatus string

const (
	OperationSelectionStatusReadyForComposition OperationSelectionStatus = "ready_for_composition"
	OperationSelectionStatusNeedsClarification  OperationSelectionStatus = "needs_clarification"
	OperationSelectionStatusUnsupported         OperationSelectionStatus = "unsupported"
	OperationSelectionStatusBlocked             OperationSelectionStatus = "blocked"
)

type OperationInputStrategy string

const (
	OperationInputStrategyNoUserInput      OperationInputStrategy = "no_user_input"
	OperationInputStrategyStaticDefaults   OperationInputStrategy = "static_defaults"
	OperationInputStrategyBusinessEntities OperationInputStrategy = "business_entities"
	OperationInputStrategyStepOutput       OperationInputStrategy = "step_output"
)

// PURPOSE: Defines the bounded LLM selector output consumed by deterministic plan composition.
type OperationSelectionPlan struct {
	SchemaVersion      string                       `json:"schema_version"`
	RequestID          string                       `json:"request_id"`
	Marketplace        string                       `json:"marketplace"`
	Status             OperationSelectionStatus     `json:"status"`
	UserFacingSummary  string                       `json:"user_facing_summary,omitempty"`
	SelectedOperations []SelectedOperation          `json:"selected_operations"`
	MissingInputs      []MissingBusinessInput       `json:"missing_inputs"`
	RejectedCandidates []RejectedOperationCandidate `json:"rejected_candidates"`
	Warnings           []PlanWarning                `json:"warnings"`
	Metadata           *RequestMetadata             `json:"metadata,omitempty"`
}

type SelectedOperation struct {
	OperationID   string                 `json:"operation_id"`
	Purpose       string                 `json:"purpose"`
	DependsOn     []string               `json:"depends_on"`
	InputStrategy OperationInputStrategy `json:"input_strategy"`
}

type MissingBusinessInput struct {
	Code           string   `json:"code"`
	UserQuestion   string   `json:"user_question"`
	Accepts        []string `json:"accepts"`
	InternalFields []string `json:"internal_fields"`
}

type RejectedOperationCandidate struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}

func NewOperationSelectionShapeValidationError(errors []string) OperationSelectionShapeValidationError {
	return OperationSelectionShapeValidationError{Errors: nonNilOperationSelectionErrors(errors)}
}

// PURPOSE: Reports deterministic selection-contract validation failures without interpreting business intent.
type OperationSelectionShapeValidationError struct {
	Errors []string
}

func (e OperationSelectionShapeValidationError) Error() string {
	return fmt.Sprintf("operation selection plan shape validation failed: %v", e.Errors)
}

func (p OperationSelectionPlan) ValidateShape() error {
	errors := p.ShapeErrors()
	if len(errors) > 0 {
		return NewOperationSelectionShapeValidationError(errors)
	}

	return nil
}

func (p OperationSelectionPlan) ShapeErrors() []string {
	errors := make([]string, 0)

	if p.SchemaVersion != "1.0" {
		errors = append(errors, "schema_version must be 1.0")
	}
	if p.RequestID == "" {
		errors = append(errors, "request_id is empty")
	}
	if p.Marketplace != "wildberries" {
		errors = append(errors, "marketplace must be wildberries")
	}

	switch p.Status {
	case OperationSelectionStatusReadyForComposition:
		errors = append(errors, validateReadyForCompositionSelectionShape(p)...)
	case OperationSelectionStatusNeedsClarification:
		errors = append(errors, validateNeedsClarificationSelectionShape(p)...)
	case OperationSelectionStatusUnsupported:
		errors = append(errors, validateUnsupportedSelectionShape(p)...)
	case OperationSelectionStatusBlocked:
		errors = append(errors, validateBlockedSelectionShape(p)...)
	default:
		errors = append(errors, fmt.Sprintf("unsupported status %q", p.Status))
	}

	if p.SelectedOperations == nil {
		errors = append(errors, "selected_operations must be an array")
	}
	if p.MissingInputs == nil {
		errors = append(errors, "missing_inputs must be an array")
	}
	if p.RejectedCandidates == nil {
		errors = append(errors, "rejected_candidates must be an array")
	}
	if p.Warnings == nil {
		errors = append(errors, "warnings must be an array")
	}

	for index, operation := range p.SelectedOperations {
		errors = append(errors, validateSelectedOperationShape(index, operation)...)
	}

	for index, input := range p.MissingInputs {
		errors = append(errors, validateMissingBusinessInputShape(index, input)...)
	}

	for index, candidate := range p.RejectedCandidates {
		errors = append(errors, validateRejectedCandidateShape(index, candidate)...)
	}

	return errors
}

func validateReadyForCompositionSelectionShape(plan OperationSelectionPlan) []string {
	errors := make([]string, 0)

	if len(plan.SelectedOperations) == 0 {
		errors = append(errors, "ready_for_composition requires at least one selected operation")
	}
	if len(plan.MissingInputs) > 0 {
		errors = append(errors, "ready_for_composition must not contain missing inputs")
	}

	return errors
}

func validateNeedsClarificationSelectionShape(plan OperationSelectionPlan) []string {
	errors := make([]string, 0)

	if len(plan.MissingInputs) == 0 {
		errors = append(errors, "needs_clarification requires at least one missing input")
	}

	return errors
}

func validateUnsupportedSelectionShape(plan OperationSelectionPlan) []string {
	errors := make([]string, 0)

	if plan.UserFacingSummary == "" {
		errors = append(errors, "unsupported requires user_facing_summary")
	}

	return errors
}

func validateBlockedSelectionShape(plan OperationSelectionPlan) []string {
	errors := make([]string, 0)

	if plan.UserFacingSummary == "" {
		errors = append(errors, "blocked requires user_facing_summary")
	}
	if len(plan.SelectedOperations) > 0 {
		errors = append(errors, "blocked must not contain selected operations")
	}

	return errors
}

func validateSelectedOperationShape(index int, operation SelectedOperation) []string {
	errors := make([]string, 0)

	if operation.OperationID == "" {
		errors = append(errors, fmt.Sprintf("selected_operations[%d].operation_id is empty", index))
	}
	if operation.Purpose == "" {
		errors = append(errors, fmt.Sprintf("selected_operations[%d].purpose is empty", index))
	}
	if operation.DependsOn == nil {
		errors = append(errors, fmt.Sprintf("selected_operations[%d].depends_on must be an array", index))
	}
	if !isSupportedOperationInputStrategy(operation.InputStrategy) {
		errors = append(errors, fmt.Sprintf("selected_operations[%d].input_strategy is unsupported: %q", index, operation.InputStrategy))
	}

	return errors
}

func validateMissingBusinessInputShape(index int, input MissingBusinessInput) []string {
	errors := make([]string, 0)

	if input.Code == "" {
		errors = append(errors, fmt.Sprintf("missing_inputs[%d].code is empty", index))
	}
	if input.UserQuestion == "" {
		errors = append(errors, fmt.Sprintf("missing_inputs[%d].user_question is empty", index))
	}
	if input.Accepts == nil {
		errors = append(errors, fmt.Sprintf("missing_inputs[%d].accepts must be an array", index))
	}
	if input.InternalFields == nil {
		errors = append(errors, fmt.Sprintf("missing_inputs[%d].internal_fields must be an array", index))
	}

	return errors
}

func validateRejectedCandidateShape(index int, candidate RejectedOperationCandidate) []string {
	errors := make([]string, 0)

	if candidate.OperationID == "" {
		errors = append(errors, fmt.Sprintf("rejected_candidates[%d].operation_id is empty", index))
	}
	if candidate.Reason == "" {
		errors = append(errors, fmt.Sprintf("rejected_candidates[%d].reason is empty", index))
	}

	return errors
}

func isSupportedOperationInputStrategy(strategy OperationInputStrategy) bool {
	switch strategy {
	case OperationInputStrategyNoUserInput,
		OperationInputStrategyStaticDefaults,
		OperationInputStrategyBusinessEntities,
		OperationInputStrategyStepOutput:
		return true
	default:
		return false
	}
}

func nonNilOperationSelectionErrors(errors []string) []string {
	if errors == nil {
		return []string{}
	}

	return errors
}
