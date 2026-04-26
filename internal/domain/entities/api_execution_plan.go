package entities

// PURPOSE: Defines the stable machine-executable output contract returned by the service.
type ApiExecutionPlan struct {
	SchemaVersion          string                `json:"schema_version"`
	RequestID              string                `json:"request_id"`
	Marketplace            string                `json:"marketplace"`
	Status                 string                `json:"status"`
	Intent                 string                `json:"intent"`
	NaturalLanguageSummary string                `json:"natural_language_summary,omitempty"`
	RiskLevel              string                `json:"risk_level"`
	RequiresApproval       bool                  `json:"requires_approval"`
	ApprovalReason         string                `json:"approval_reason,omitempty"`
	BlockReason            string                `json:"block_reason,omitempty"`
	ClarifyingQuestions    []string              `json:"clarifying_questions,omitempty"`
	ExecutionMode          string                `json:"execution_mode"`
	Inputs                 map[string]InputValue `json:"inputs"`
	Steps                  []ApiPlanStep         `json:"steps"`
	Transforms             []TransformStep       `json:"transforms"`
	FinalOutput            FinalOutput           `json:"final_output"`
	Warnings               []PlanWarning         `json:"warnings"`
	Validation             PlanValidation        `json:"validation"`
}

type InputValue struct {
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Value       any    `json:"value,omitempty"`
	Format      string `json:"format,omitempty"`
	Description string `json:"description,omitempty"`
}

type ApiPlanStep struct {
	StepID          string              `json:"step_id"`
	OperationID     string              `json:"operation_id"`
	SourceFile      string              `json:"source_file"`
	Readonly        bool                `json:"readonly"`
	RiskLevel       string              `json:"risk_level"`
	Purpose         string              `json:"purpose"`
	DependsOn       []string            `json:"depends_on"`
	Request         HttpRequestTemplate `json:"request"`
	Pagination      PaginationPlan      `json:"pagination"`
	RetryPolicy     RetryPolicy         `json:"retry_policy"`
	RateLimitPolicy RateLimitPolicy     `json:"rate_limit_policy"`
	ResponseMapping ResponseMapping     `json:"response_mapping"`
	ValidationRules []ValidationRule    `json:"validation_rules,omitempty"`
}

type HttpRequestTemplate struct {
	ServerURL    string                   `json:"server_url"`
	Method       string                   `json:"method"`
	PathTemplate string                   `json:"path_template"`
	PathParams   map[string]ValueBinding  `json:"path_params"`
	QueryParams  map[string]ValueBinding  `json:"query_params"`
	Headers      map[string]HeaderBinding `json:"headers"`
	Body         any                      `json:"body"`
	ContentType  string                   `json:"content_type"`
	Accept       string                   `json:"accept"`
}

type ValueBinding struct {
	Source     string `json:"source"`
	Value      any    `json:"value,omitempty"`
	InputName  string `json:"input_name,omitempty"`
	StepID     string `json:"step_id,omitempty"`
	OutputName string `json:"output_name,omitempty"`
	Expression string `json:"expression,omitempty"`
	SecretName string `json:"secret_name,omitempty"`
	Required   bool   `json:"required,omitempty"`
}

type HeaderBinding struct {
	Source     string `json:"source"`
	Value      string `json:"value,omitempty"`
	SecretName string `json:"secret_name,omitempty"`
	InputName  string `json:"input_name,omitempty"`
	Required   bool   `json:"required"`
}

type PaginationPlan struct {
	Enabled              bool   `json:"enabled"`
	Strategy             string `json:"strategy"`
	LimitParam           string `json:"limit_param,omitempty"`
	OffsetParam          string `json:"offset_param,omitempty"`
	CursorParam          string `json:"cursor_param,omitempty"`
	PageParam            string `json:"page_param,omitempty"`
	Limit                int    `json:"limit,omitempty"`
	MaxLimit             int    `json:"max_limit,omitempty"`
	InitialOffset        int    `json:"initial_offset,omitempty"`
	NextOffsetExpression string `json:"next_offset_expression,omitempty"`
	ItemsPath            string `json:"items_path,omitempty"`
	StopCondition        string `json:"stop_condition,omitempty"`
}

type RetryPolicy struct {
	Enabled       bool          `json:"enabled"`
	MaxAttempts   int           `json:"max_attempts,omitempty"`
	RetryOnStatus []int         `json:"retry_on_status,omitempty"`
	Backoff       BackoffPolicy `json:"backoff,omitempty"`
}

type BackoffPolicy struct {
	Type           string `json:"type"`
	InitialDelayMS int    `json:"initial_delay_ms,omitempty"`
	MaxDelayMS     int    `json:"max_delay_ms,omitempty"`
}

type RateLimitPolicy struct {
	Enabled       bool   `json:"enabled"`
	Bucket        string `json:"bucket,omitempty"`
	MaxRequests   int    `json:"max_requests,omitempty"`
	PeriodSeconds int    `json:"period_seconds,omitempty"`
	MinIntervalMS int    `json:"min_interval_ms,omitempty"`
}

type ResponseMapping struct {
	Outputs     map[string]MappedOutput `json:"outputs"`
	PostFilters []PostFilter            `json:"post_filters,omitempty"`
}

type MappedOutput struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type PostFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type TransformStep struct {
	TransformID string         `json:"transform_id"`
	Type        string         `json:"type"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Outputs     map[string]any `json:"outputs,omitempty"`
}

type FinalOutput struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Fields      map[string]any `json:"fields,omitempty"`
}

type PlanWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PlanValidation struct {
	RegistryChecked       bool     `json:"registry_checked"`
	OutputSchemaChecked   bool     `json:"output_schema_checked"`
	ReadonlyPolicyChecked bool     `json:"readonly_policy_checked"`
	SecretsPolicyChecked  bool     `json:"secrets_policy_checked"`
	JamPolicyChecked      bool     `json:"jam_policy_checked"`
	Errors                []string `json:"errors"`
}

func NewNeedsClarificationPlan(request BusinessRequest, questions []string) *ApiExecutionPlan {
	requestID := request.RequestID
	if requestID == "" {
		requestID = "unknown"
	}

	intent := request.Intent
	if intent == "" {
		intent = "unknown"
	}

	executionMode := request.Constraints.ExecutionMode
	if executionMode == "" {
		executionMode = "not_executable"
	}

	return &ApiExecutionPlan{
		SchemaVersion:       "1.0",
		RequestID:           requestID,
		Marketplace:         "wildberries",
		Status:              "needs_clarification",
		Intent:              intent,
		RiskLevel:           "unknown",
		RequiresApproval:    false,
		ClarifyingQuestions: questions,
		ExecutionMode:       executionMode,
		Inputs:              map[string]InputValue{},
		Steps:               []ApiPlanStep{},
		Transforms:          []TransformStep{},
		FinalOutput: FinalOutput{
			Type:        "none",
			Description: "Plan cannot be built until required request fields are provided.",
		},
		Warnings: []PlanWarning{},
		Validation: PlanValidation{
			RegistryChecked:       false,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: false,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      false,
			Errors:                []string{},
		},
	}
}
