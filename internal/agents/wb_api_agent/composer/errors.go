package composer

import "fmt"

const CompositionNotImplementedCode = "composition_not_implemented"
const CompositionUnsupportedCode = "composition_unsupported"

// PURPOSE: Reports that the composer contract is ready but deterministic plan construction is not implemented yet.
type ApiPlanCompositionNotImplementedError struct {
	Code      string
	RequestID string
}

func NewApiPlanCompositionNotImplementedError(requestID string) ApiPlanCompositionNotImplementedError {
	return ApiPlanCompositionNotImplementedError{
		Code:      CompositionNotImplementedCode,
		RequestID: requestID,
	}
}

func (e ApiPlanCompositionNotImplementedError) Error() string {
	return fmt.Sprintf("%s: request_id=%s", e.Code, e.RequestID)
}

// PURPOSE: Reports explicit deterministic composer capability gaps without falling back to LLM execution-plan generation.
type ApiPlanCompositionUnsupportedError struct {
	Code      string
	RequestID string
	Reason    string
	Message   string
}

func NewApiPlanCompositionUnsupportedError(
	requestID string,
	reason string,
	message string,
) ApiPlanCompositionUnsupportedError {
	return ApiPlanCompositionUnsupportedError{
		Code:      CompositionUnsupportedCode,
		RequestID: requestID,
		Reason:    reason,
		Message:   message,
	}
}

func (e ApiPlanCompositionUnsupportedError) Error() string {
	return fmt.Sprintf(
		"%s: request_id=%s reason=%s message=%s",
		e.Code,
		e.RequestID,
		e.Reason,
		e.Message,
	)
}
