package entities

// PURPOSE: Represents the external business request accepted by the WB API planning agent.
type BusinessRequest struct {
	RequestID              string              `json:"request_id"`
	Marketplace            string              `json:"marketplace"`
	Intent                 string              `json:"intent"`
	NaturalLanguageRequest string              `json:"natural_language_request"`
	Entities               map[string]any      `json:"entities"`
	Period                 *Period             `json:"period,omitempty"`
	Constraints            BusinessConstraints `json:"constraints"`
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
