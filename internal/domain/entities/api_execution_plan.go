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
	Metadata               *RequestMetadata      `json:"metadata,omitempty"`
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
type ValidationRule struct {
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity,omitempty"`
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
		Metadata: request.Metadata,
	}
}
func NewBlockedPlan(request BusinessRequest, blockReason string, warnings []PlanWarning) *ApiExecutionPlan {
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
		SchemaVersion:    "1.0",
		RequestID:        requestID,
		Marketplace:      "wildberries",
		Status:           "blocked",
		Intent:           intent,
		RiskLevel:        "unknown",
		RequiresApproval: false,
		BlockReason:      blockReason,
		ExecutionMode:    executionMode,
		Inputs:           map[string]InputValue{},
		Steps:            []ApiPlanStep{},
		Transforms:       []TransformStep{},
		FinalOutput: FinalOutput{
			Type:        "none",
			Description: "Plan cannot be built from the current request and constraints.",
		},
		Warnings: warnings,
		Validation: PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: request.Constraints.ReadonlyOnly,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      request.Constraints.NoJamSubscription,
			Errors:                []string{blockReason},
		},
		Metadata: request.Metadata,
	}
}

func NewPlanningNotImplementedPlan(request BusinessRequest) *ApiExecutionPlan {
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
		ClarifyingQuestions: []string{"Planner found candidate WB API operations, but final step composition is not implemented yet."},
		ExecutionMode:       executionMode,
		Inputs:              map[string]InputValue{},
		Steps:               []ApiPlanStep{},
		Transforms:          []TransformStep{},
		FinalOutput: FinalOutput{
			Type:        "none",
			Description: "Plan cannot be finalized until step composition is implemented.",
		},
		Warnings: []PlanWarning{},
		Validation: PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: request.Constraints.ReadonlyOnly,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      request.Constraints.NoJamSubscription,
			Errors:                []string{},
		},
		Metadata: request.Metadata,
	}
}

func NewSellerWarehouseStocksPlan(
	request BusinessRequest,
	operation WBRegistryOperation,
	warehouseID int,
	chrtIDs []int,
) *ApiExecutionPlan {
	executionMode := request.Constraints.ExecutionMode
	if executionMode == "" {
		executionMode = "automatic"
	}

	return &ApiExecutionPlan{
		SchemaVersion:          "1.0",
		RequestID:              request.RequestID,
		Marketplace:            "wildberries",
		Status:                 "ready",
		Intent:                 request.Intent,
		NaturalLanguageSummary: "Get product stocks from seller warehouse.",
		RiskLevel:              "read",
		RequiresApproval:       false,
		ExecutionMode:          executionMode,
		Inputs: map[string]InputValue{
			"warehouse_id": {
				Type:        "integer",
				Required:    true,
				Value:       warehouseID,
				Description: "Seller warehouse ID.",
			},
			"chrt_ids": {
				Type:        "array",
				Required:    true,
				Value:       chrtIDs,
				Description: "Product size IDs requested from WB stocks endpoint.",
			},
		},
		Steps: []ApiPlanStep{
			{
				StepID:      "get_seller_warehouse_stocks",
				OperationID: operation.OperationID,
				SourceFile:  operation.SourceFile,
				Readonly:    true,
				RiskLevel:   "read",
				Purpose:     "Get stock amounts for requested product sizes on the selected seller warehouse.",
				DependsOn:   []string{},
				Request: HttpRequestTemplate{
					ServerURL:    operation.ServerURL,
					Method:       operation.Method,
					PathTemplate: operation.PathTemplate,
					PathParams: map[string]ValueBinding{
						"warehouseId": {
							Source:    "input",
							InputName: "warehouse_id",
							Required:  true,
						},
					},
					QueryParams: map[string]ValueBinding{},
					Headers: map[string]HeaderBinding{
						"Authorization": {
							Source:     "executor_secret",
							SecretName: "WB_AUTHORIZATION",
							Required:   true,
						},
					},
					// WHY: Executor must resolve body values from declared plan inputs instead of relying on duplicated literal payload data.
					Body: map[string]any{
						"chrtIds": ValueBinding{
							Source:    "input",
							InputName: "chrt_ids",
							Required:  true,
						},
					},
					ContentType: "application/json",
					Accept:      "application/json",
				},
				Pagination: PaginationPlan{
					Enabled:  false,
					Strategy: "none",
				},
				RetryPolicy: RetryPolicy{
					Enabled:       true,
					MaxAttempts:   3,
					RetryOnStatus: []int{429, 500, 502, 503, 504},
					Backoff: BackoffPolicy{
						Type:           "exponential",
						InitialDelayMS: 1000,
						MaxDelayMS:     20000,
					},
				},
				RateLimitPolicy: RateLimitPolicy{
					Enabled:       true,
					Bucket:        "marketplace_stocks",
					MaxRequests:   300,
					PeriodSeconds: 60,
					MinIntervalMS: 200,
				},
				ResponseMapping: ResponseMapping{
					Outputs: map[string]MappedOutput{
						"stocks": {
							Type: "rows",
							Path: "$.stocks",
						},
					},
					PostFilters: []PostFilter{},
				},
			},
		},
		Transforms: []TransformStep{},
		FinalOutput: FinalOutput{
			Type:        "object",
			Description: "Stocks by chrtId for the requested seller warehouse.",
			Fields: map[string]any{
				"warehouse_id": "input.warehouse_id",
				"stocks":       "steps.get_seller_warehouse_stocks.outputs.stocks",
			},
		},
		Warnings: []PlanWarning{},
		Validation: PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: true,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      request.Constraints.NoJamSubscription,
			Errors:                []string{},
		},
		Metadata: request.Metadata,
	}
}

func NewLLMPlanningNotImplementedPlan(request BusinessRequest, candidates []WBRegistryOperation) *ApiExecutionPlan {
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
		ClarifyingQuestions: []string{"LLM planner fallback input is prepared, but ADK planner and formatter execution are not implemented yet."},
		ExecutionMode:       executionMode,
		Inputs:              map[string]InputValue{},
		Steps:               []ApiPlanStep{},
		Transforms:          []TransformStep{},
		FinalOutput: FinalOutput{
			Type:        "none",
			Description: "Registry candidates and planner input were prepared, but ADK planner and formatter are not connected yet.",
		},
		Warnings: candidateOperationWarnings(candidates),
		Validation: PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: request.Constraints.ReadonlyOnly,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      request.Constraints.NoJamSubscription,
			Errors:                []string{},
		},
		Metadata: request.Metadata,
	}
}

func candidateOperationWarnings(candidates []WBRegistryOperation) []PlanWarning {
	if len(candidates) == 0 {
		return []PlanWarning{}
	}

	limit := len(candidates)
	if limit > 5 {
		limit = 5
	}

	warnings := make([]PlanWarning, 0, limit)
	for i := 0; i < limit; i++ {
		operation := candidates[i]

		warnings = append(warnings, PlanWarning{
			Code:    "candidate_registry_operation",
			Message: operation.OperationID + " " + operation.Method + " " + operation.PathTemplate,
		})
	}

	return warnings
}

func NewRegistryValidatedNeedsClarificationPlan(
	request BusinessRequest,
	questions []string,
	warnings []PlanWarning,
) *ApiExecutionPlan {
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
			Description: "Plan cannot be finalized until required input values are provided.",
		},
		Warnings: warnings,
		Validation: PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: request.Constraints.ReadonlyOnly,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      request.Constraints.NoJamSubscription,
			Errors:                []string{},
		},
		Metadata: request.Metadata,
	}
}
