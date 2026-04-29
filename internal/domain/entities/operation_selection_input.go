package entities

import "fmt"

// PURPOSE: Defines the bounded input contract for probabilistic operation selection.
type OperationSelectionInput struct {
	SchemaVersion      string                        `json:"schema_version"`
	RequestID          string                        `json:"request_id"`
	Marketplace        string                        `json:"marketplace"`
	BusinessRequest    BusinessRequest               `json:"business_request"`
	RegistryCandidates []OperationSelectionCandidate `json:"registry_candidates"`
	Policies           OperationSelectionPolicies    `json:"policies"`
	Metadata           *RequestMetadata              `json:"metadata,omitempty"`
}

type OperationSelectionCandidate struct {
	OperationID              string   `json:"operation_id"`
	SourceFile               string   `json:"source_file"`
	Method                   string   `json:"method"`
	ServerURL                string   `json:"server_url"`
	PathTemplate             string   `json:"path_template"`
	Tags                     []string `json:"tags"`
	Category                 string   `json:"category"`
	Summary                  string   `json:"summary"`
	Description              string   `json:"description"`
	Readonly                 *bool    `json:"readonly"`
	RiskLevel                string   `json:"risk_level"`
	XCategory                string   `json:"x_category"`
	XTokenTypes              []string `json:"x_token_types"`
	PathParamsSchemaJSON     string   `json:"path_params_schema_json"`
	QueryParamsSchemaJSON    string   `json:"query_params_schema_json"`
	RequestBodySchemaJSON    string   `json:"request_body_schema_json"`
	ResponseSchemaJSON       string   `json:"response_schema_json"`
	RateLimitNotes           string   `json:"rate_limit_notes"`
	SubscriptionRequirements string   `json:"subscription_requirements"`
	RequiresJam              bool     `json:"requires_jam"`
}

type OperationSelectionPolicies struct {
	ReadonlyOnly      bool `json:"readonly_only"`
	NoJamSubscription bool `json:"no_jam_subscription"`
	NoSecrets         bool `json:"no_secrets"`
	NoHTTPExecution   bool `json:"no_http_execution"`
	RegistryOnly      bool `json:"registry_only"`
}

func NewOperationSelectionInput(
	request BusinessRequest,
	candidates []WBRegistryOperation,
) OperationSelectionInput {
	return OperationSelectionInput{
		SchemaVersion:   "1.0",
		RequestID:       request.RequestID,
		Marketplace:     request.Marketplace,
		BusinessRequest: request,
		RegistryCandidates: OperationSelectionCandidatesFromRegistry(
			candidates,
		),
		Policies: OperationSelectionPolicies{
			ReadonlyOnly:      request.Constraints.ReadonlyOnly,
			NoJamSubscription: request.Constraints.NoJamSubscription,
			NoSecrets:         true,
			NoHTTPExecution:   true,
			RegistryOnly:      true,
		},
		Metadata: request.Metadata,
	}
}

func OperationSelectionCandidatesFromRegistry(
	candidates []WBRegistryOperation,
) []OperationSelectionCandidate {
	result := make([]OperationSelectionCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		result = append(result, OperationSelectionCandidateFromRegistry(candidate))
	}

	return result
}

func OperationSelectionCandidateFromRegistry(
	candidate WBRegistryOperation,
) OperationSelectionCandidate {
	return OperationSelectionCandidate{
		OperationID:              candidate.OperationID,
		SourceFile:               candidate.SourceFile,
		Method:                   candidate.Method,
		ServerURL:                candidate.ServerURL,
		PathTemplate:             candidate.PathTemplate,
		Tags:                     nonNilOperationSelectionStrings(candidate.Tags),
		Category:                 candidate.Category,
		Summary:                  candidate.Summary,
		Description:              candidate.Description,
		Readonly:                 candidate.XReadonlyMethod,
		RiskLevel:                riskLevelFromReadonly(candidate.XReadonlyMethod),
		XCategory:                candidate.XCategory,
		XTokenTypes:              nonNilOperationSelectionStrings(candidate.XTokenTypes),
		PathParamsSchemaJSON:     candidate.PathParamsSchemaJSON,
		QueryParamsSchemaJSON:    candidate.QueryParamsSchemaJSON,
		RequestBodySchemaJSON:    candidate.RequestBodySchemaJSON,
		ResponseSchemaJSON:       candidate.ResponseSchemaJSON,
		RateLimitNotes:           candidate.RateLimitNotes,
		SubscriptionRequirements: candidate.SubscriptionRequirements,
		RequiresJam:              candidate.RequiresJam,
	}
}

func (i OperationSelectionInput) ValidateShape() error {
	errors := i.ShapeErrors()
	if len(errors) > 0 {
		return OperationSelectionInputShapeValidationError{Errors: errors}
	}

	return nil
}

func (i OperationSelectionInput) ShapeErrors() []string {
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
	if i.BusinessRequest.NaturalLanguageRequest == "" {
		errors = append(errors, "business_request.natural_language_request is empty")
	}
	if i.RegistryCandidates == nil {
		errors = append(errors, "registry_candidates must be an array")
	}
	if len(i.RegistryCandidates) == 0 {
		errors = append(errors, "registry_candidates must contain at least one candidate")
	}
	if !i.Policies.NoSecrets {
		errors = append(errors, "policies.no_secrets must be true")
	}
	if !i.Policies.NoHTTPExecution {
		errors = append(errors, "policies.no_http_execution must be true")
	}
	if !i.Policies.RegistryOnly {
		errors = append(errors, "policies.registry_only must be true")
	}

	for index, candidate := range i.RegistryCandidates {
		errors = append(errors, validateOperationSelectionCandidateShape(index, candidate)...)
	}

	return errors
}

// PURPOSE: Reports deterministic selector-input contract failures before any LLM call is allowed.
type OperationSelectionInputShapeValidationError struct {
	Errors []string
}

func (e OperationSelectionInputShapeValidationError) Error() string {
	return fmt.Sprintf("operation selection input shape validation failed: %v", e.Errors)
}

func validateOperationSelectionCandidateShape(
	index int,
	candidate OperationSelectionCandidate,
) []string {
	errors := make([]string, 0)

	if candidate.OperationID == "" {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].operation_id is empty", index))
	}
	if candidate.SourceFile == "" {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].source_file is empty", index))
	}
	if candidate.Method == "" {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].method is empty", index))
	}
	if candidate.ServerURL == "" {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].server_url is empty", index))
	}
	if candidate.PathTemplate == "" {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].path_template is empty", index))
	}
	if candidate.Tags == nil {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].tags must be an array", index))
	}
	if candidate.XTokenTypes == nil {
		errors = append(errors, fmt.Sprintf("registry_candidates[%d].x_token_types must be an array", index))
	}

	return errors
}

func riskLevelFromReadonly(readonly *bool) string {
	if readonly == nil {
		return "unknown"
	}

	if *readonly {
		return "read"
	}

	return "write"
}

func nonNilOperationSelectionStrings(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}
