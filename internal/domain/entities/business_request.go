package entities

// PURPOSE: Represents the external business request accepted by the WB API planning agent.
type BusinessRequest struct {
	RequestID              string              `json:"request_id"`
	Marketplace            string              `json:"marketplace"`
	Intent                 string              `json:"intent,omitempty"`
	NaturalLanguageRequest string              `json:"natural_language_request"`
	Entities               map[string]any      `json:"entities,omitempty"`
	Period                 *Period             `json:"period,omitempty"`
	Constraints            BusinessConstraints `json:"constraints,omitempty"`
	Metadata               *RequestMetadata    `json:"metadata,omitempty"`
}

type RequestMetadata struct {
	CorrelationID     string         `json:"correlation_id,omitempty"`
	SessionID         string         `json:"session_id,omitempty"`
	RunID             string         `json:"run_id,omitempty"`
	ToolCallID        string         `json:"tool_call_id,omitempty"`
	ClientExecutionID string         `json:"client_execution_id,omitempty"`
	UserID            string         `json:"user_id,omitempty"`
	Source            string         `json:"source,omitempty"`
	Extra             map[string]any `json:"extra,omitempty"`
}

type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type BusinessConstraints struct {
	ReadonlyOnly      bool   `json:"readonly_only"`
	NoJamSubscription bool   `json:"no_jam_subscription"`
	MaxSteps          int    `json:"max_steps"`
	ExecutionMode     string `json:"execution_mode"`
}

// NormalizeCorrelationIdentifiers keeps request_id and metadata.correlation_id aligned
// while preserving existing boundary validation behavior.
func (r *BusinessRequest) NormalizeCorrelationIdentifiers() {
	if r == nil {
		return
	}

	if r.Metadata == nil {
		if r.RequestID == "" {
			return
		}

		r.Metadata = &RequestMetadata{
			CorrelationID: r.RequestID,
		}
		return
	}

	if r.Metadata.CorrelationID == "" && r.RequestID != "" {
		r.Metadata.CorrelationID = r.RequestID
		return
	}

	if r.RequestID == "" && r.Metadata.CorrelationID != "" {
		r.RequestID = r.Metadata.CorrelationID
	}
}
