package wb_api_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Holds dependencies and prompts needed by the WB API agent.
type Config struct {
	Registry             wbregistry.Retriever
	DeterministicPlanner planning.DeterministicPlanner
	SessionService       adksession.Service
	Model                model.LLM
	Logger               *log.Logger
	SystemPrompt         string
	PlanPrompt           string
	ExplorePrompt        string
	GeneralPrompt        string
	DebugLogPlannerInput bool
	ModelName            string
}

// PURPOSE: Orchestrates deterministic planning first, then falls back to registry-grounded ADK planning.
type Agent struct {
	registry             wbregistry.Retriever
	deterministicPlanner planning.DeterministicPlanner
	postProcessor        *PlanPostProcessor
	logger               *log.Logger
	adkRunner            *runner.Runner
	debugLogPlannerInput bool
	systemPrompt         string
	planPrompt           string
	explorePrompt        string
	generalPrompt        string
}

func New(cfg Config) (*Agent, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	instruction := buildADKInstruction(cfg)

	adkPlannerAgent, err := llmagent.New(llmagent.Config{
		Name:        "wb_api_planner_agent",
		Description: "Builds registry-grounded Wildberries ApiExecutionPlan JSON objects.",
		Model:       cfg.Model,
		// WHY: Instruction contains JSON examples with braces; InstructionProvider prevents ADK state-template substitution.
		InstructionProvider: func(ctx adkagent.ReadonlyContext) (string, error) {
			return instruction, nil
		},
		IncludeContents: llmagent.IncludeContentsNone,
	})
	if err != nil {
		return nil, fmt.Errorf("create wb api planner agent: %w", err)
	}

	adkRunner, err := runner.New(runner.Config{
		AppName: "wb_api_agent_system",
		Agent:   adkPlannerAgent,

		SessionService:    cfg.SessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create adk runner: %w", err)
	}

	return &Agent{
		registry:             cfg.Registry,
		deterministicPlanner: cfg.DeterministicPlanner,
		postProcessor:        NewPlanPostProcessor(cfg.Registry),
		logger:               logger,
		adkRunner:            adkRunner,
		systemPrompt:         cfg.SystemPrompt,
		planPrompt:           cfg.PlanPrompt,
		explorePrompt:        cfg.ExplorePrompt,
		generalPrompt:        cfg.GeneralPrompt,
		debugLogPlannerInput: cfg.DebugLogPlannerInput,
	}, nil
}

func (a *Agent) Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error) {
	request.NormalizeCorrelationIdentifiers()

	questions := requiredQuestions(request)
	if len(questions) > 0 {
		// WHY: Boundary requests must be converted into ApiExecutionPlan instead of HTTP errors.
		return entities.NewNeedsClarificationPlan(request, questions), nil
	}

	plan, handled, err := a.deterministicPlanner.TryPlan(ctx, request)
	if handled {
		a.logger.Printf("deterministic planner handled request_id=%s correlation_id=%s session_id=%s run_id=%s tool_call_id=%s client_execution_id=%s",
			request.RequestID,
			metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.CorrelationID }),
			metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.SessionID }),
			metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.RunID }),
			metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ToolCallID }),
			metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ClientExecutionID }),
		)
	}
	if err != nil || handled {
		return plan, err
	}

	operations, err := a.registry.SearchOperations(ctx, wbregistry.SearchQuery{
		Query:        buildRegistrySearchQuery(request),
		Limit:        effectiveMaxSteps(request.Constraints.MaxSteps),
		ReadonlyOnly: request.Constraints.ReadonlyOnly,
		ExcludeJam:   request.Constraints.NoJamSubscription,
	})
	if err != nil {
		return nil, err
	}

	if len(operations) == 0 {
		return entities.NewBlockedPlan(
			request,
			"no_registry_operations_match_request_constraints",
			[]entities.PlanWarning{
				{
					Code:    "no_matching_operations",
					Message: "No WB API registry operations match the request and constraints.",
				},
			},
		), nil
	}

	// WHY: Non-deterministic requests are delegated to ADK after registry retrieval has constrained the operation set.
	a.logger.Printf("adk fallback started request_id=%s correlation_id=%s session_id=%s run_id=%s tool_call_id=%s client_execution_id=%s",
		request.RequestID,
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.CorrelationID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.SessionID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.RunID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ToolCallID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ClientExecutionID }),
	)
	return a.planWithADK(ctx, request, operations)
}

func (a *Agent) planWithADK(
	ctx context.Context,
	request entities.BusinessRequest,
	candidates []entities.WBRegistryOperation,
) (*entities.ApiExecutionPlan, error) {
	plannerInput := a.buildPlannerInput(request, candidates)

	inputJSON, err := json.Marshal(plannerInput)
	if err != nil {
		return nil, fmt.Errorf("marshal planner input: %w", err)
	}

	// WHY: Full planner input may contain large request context; log it only under explicit debug opt-in.
	if a.debugLogPlannerInput {
		a.logger.Printf("LLM planner input prepared: request_id=%s input=%s", request.RequestID, string(inputJSON))
	} else {
		a.logger.Printf(
			"LLM planner input prepared request_id=%s intent=%s candidates=%d operation_ids=%s",
			request.RequestID,
			request.Intent,
			len(candidates),
			safeOperationIDs(candidates),
		)
	}

	responseText, err := a.runADK(ctx, request, string(inputJSON))
	if err != nil {
		return entities.NewBlockedPlan(request, "adk_planner_execution_failed", []entities.PlanWarning{
			{
				Code:    "adk_planner_error",
				Message: err.Error(),
			},
		}), nil
	}

	// WHY: Invalid model output must be inspectable server-side without exposing raw LLM content to A2A clients.
	a.logger.Printf("ADK planner raw response: request_id=%s response=%s", request.RequestID, responseText)

	plan, err := parseApiExecutionPlan(responseText)
	if err != nil {
		return entities.NewBlockedPlan(request, "adk_planner_returned_invalid_api_execution_plan", []entities.PlanWarning{
			{
				Code:    "invalid_adk_output",
				Message: err.Error(),
			},
		}), nil
	}

	// WHY: ADK may use camelCase inputs or literal request values; normalize before registry validation and executor handoff.
	normalizePlan(request, plan)

	validationPlan, err := a.postProcessor.Process(ctx, request, plan)
	if err != nil {
		return nil, err
	}
	a.logger.Printf("plan post-processing completed request_id=%s correlation_id=%s session_id=%s run_id=%s tool_call_id=%s client_execution_id=%s",
		request.RequestID,
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.CorrelationID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.SessionID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.RunID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ToolCallID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ClientExecutionID }),
	)
	if validationPlan != nil {
		return validationPlan, nil
	}

	return plan, nil
}
func safeOperationIDs(candidates []entities.WBRegistryOperation) string {
	if len(candidates) == 0 {
		return ""
	}

	limit := len(candidates)
	if limit > 10 {
		limit = 10
	}

	ids := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		ids = append(ids, candidates[i].OperationID)
	}

	return strings.Join(ids, ",")
}
func (a *Agent) runADK(ctx context.Context, request entities.BusinessRequest, inputJSON string) (string, error) {
	userID := "a2a"
	sessionID := request.RequestID
	if sessionID == "" {
		sessionID = "unknown"
	}

	message := genai.NewContentFromText(inputJSON, genai.RoleUser)

	var finalText string
	for event, err := range a.adkRunner.Run(ctx, userID, sessionID, message, adkagent.RunConfig{
		StreamingMode: adkagent.StreamingModeNone,
	}) {
		if err != nil {
			return "", err
		}

		if event == nil || event.Content == nil {
			continue
		}

		text := contentText(event.Content)
		if text == "" {
			continue
		}

		finalText = text
		if event.IsFinalResponse() {
			break
		}
	}

	if finalText == "" {
		return "", fmt.Errorf("adk planner returned empty response")
	}

	return finalText, nil
}

func (a *Agent) buildPlannerInput(
	request entities.BusinessRequest,
	candidates []entities.WBRegistryOperation,
) PlannerInput {
	return PlannerInput{
		BusinessRequest:    request,
		RegistryCandidates: registryCandidatesForPlanner(candidates),
		Prompts: PlannerPrompts{
			System:  a.systemPrompt,
			Plan:    a.planPrompt,
			Explore: a.explorePrompt,
			General: a.generalPrompt,
		},
		Policies: PlannerPolicies{
			ReadonlyOnly:      request.Constraints.ReadonlyOnly,
			NoJamSubscription: request.Constraints.NoJamSubscription,
			NoSecrets:         true,
			NoHTTPExecution:   true,
			RegistryOnly:      true,
		},
		Metadata: request.Metadata,
		OutputContract: strings.Join([]string{
			"Return exactly one ApiExecutionPlan JSON object.",
			"The JSON object must be the ApiExecutionPlan itself, not wrapped in result, data, plan, api_execution_plan, or markdown.",
			"schema_version must be \"1.0\".",
			"marketplace must be \"wildberries\".",
			"status must be one of: ready, needs_clarification, blocked.",
			"execution_mode is required and must never be empty.",
			"If status is \"ready\", execution_mode must be \"automatic\".",
			"If status is \"blocked\", execution_mode must be \"not_executable\".",
			"If status is \"needs_clarification\", execution_mode must be \"not_executable\".",
			"inputs must be map[string]InputValue, not primitive values.",
			"Each input value must have type, required, value, and description.",
			"steps must be []ApiPlanStep with full step structure.",
			"Every step must include step_id, operation_id, source_file, readonly, risk_level, purpose, depends_on, request, pagination, retry_policy, rate_limit_policy, response_mapping.",
			"HTTP method, server_url, path_template, params, headers, body, content_type, accept must be inside step.request.",
			"Do not put Authorization directly in step.",
			"Authorization must be request.headers.Authorization={source:\"executor_secret\", secret_name:\"WB_AUTHORIZATION\", required:true}.",
			"final_output must be an object with type, description, fields.",
			"validation must be fully populated.",
			"Do not execute HTTP.",
			"Do not return real secrets.",
			"Use only registry_candidates.",
			"If business_request.entities.chrt_ids is present and non-empty, use it for stocks request body chrtIds.",
			"If business_request.entities.warehouse_id is present, use it for warehouseId.",
			"If business_request.period.from is present, use it for dateFrom.",
			"Do not copy an empty business_request.constraints.execution_mode into the plan.",
			"Do not return needs_clarification when all required inputs are present.",
		}, " "),
	}
}

func metadataValue(metadata *entities.RequestMetadata, selector func(*entities.RequestMetadata) string) string {
	if metadata == nil {
		return ""
	}

	return selector(metadata)
}

func buildADKInstruction(cfg Config) string {
	return strings.Join([]string{
		cfg.SystemPrompt,
		cfg.ExplorePrompt,
		cfg.PlanPrompt,
		cfg.GeneralPrompt,

		"You must return exactly one JSON object.",
		"Do not wrap the JSON in markdown.",
		"Do not return explanations.",
		"Do not return a nested object like {\"plan\": ...}, {\"result\": ...}, or {\"api_execution_plan\": ...}.",
		"The root JSON object must be ApiExecutionPlan.",

		"CRITICAL ApiExecutionPlan shape:",
		`{
  "schema_version": "1.0",
  "request_id": "same as business_request.request_id",
  "marketplace": "wildberries",
  "status": "ready",
  "intent": "same as business_request.intent",
  "natural_language_summary": "short summary",
  "risk_level": "read",
  "requires_approval": false,
  "execution_mode": "automatic",
  "inputs": {
    "input_name": {
      "type": "string|integer|array|object|boolean",
      "required": true,
      "value": "actual value from business_request when available",
      "description": "short description"
    }
  },
  "steps": [
    {
      "step_id": "stable_snake_case_id",
      "operation_id": "must exactly match registry_candidates[].operation_id",
      "source_file": "must exactly match registry_candidates[].source_file",
      "readonly": true,
      "risk_level": "read",
      "purpose": "why this step exists",
      "depends_on": [],
      "request": {
        "server_url": "must exactly match registry_candidates[].server_url",
        "method": "must exactly match registry_candidates[].method",
        "path_template": "must exactly match registry_candidates[].path_template",
        "path_params": {},
        "query_params": {},
        "headers": {
          "Authorization": {
            "source": "executor_secret",
            "secret_name": "WB_AUTHORIZATION",
            "required": true
          }
        },
        "body": {},
        "content_type": "application/json",
        "accept": "application/json"
      },
      "pagination": {
        "enabled": false,
        "strategy": "none"
      },
      "retry_policy": {
        "enabled": true,
        "max_attempts": 3,
        "retry_on_status": [429, 500, 502, 503, 504],
        "backoff": {
          "type": "exponential",
          "initial_delay_ms": 1000,
          "max_delay_ms": 20000
        }
      },
      "rate_limit_policy": {
        "enabled": true,
        "bucket": "operation_id",
        "max_requests": 1,
        "period_seconds": 60,
        "min_interval_ms": 1000
      },
      "response_mapping": {
        "outputs": {},
        "post_filters": []
      }
    }
  ],
  "transforms": [],
  "final_output": {
    "type": "object",
    "description": "what executor should return",
    "fields": {}
  },
  "warnings": [],
  "validation": {
    "registry_checked": true,
    "output_schema_checked": true,
    "readonly_policy_checked": true,
    "secrets_policy_checked": true,
    "jam_policy_checked": true,
    "errors": []
  }
}`,

		"Do not use primitive values inside inputs. Every inputs value must be an object with type, required, value, and description.",
		"execution_mode is required and must never be empty.",
		"If status=\"ready\", execution_mode must be \"automatic\".",
		"If status=\"blocked\", execution_mode must be \"not_executable\".",
		"If status=\"needs_clarification\", execution_mode must be \"not_executable\".",
		"Do not copy an empty business_request.constraints.execution_mode into the plan.",
		"Do not put method, server_url, path_template, or Authorization directly in a step.",
		"Every HTTP detail must be inside step.request.",
		"Authorization must be inside step.request.headers.Authorization and must use source=executor_secret, secret_name=WB_AUTHORIZATION.",
		"final_output must be an object, never null.",
		"validation must be fully populated.",
		"If required input data is missing, return status=\"needs_clarification\" with empty steps and clarifying_questions.",
		"If a policy blocks the request, return status=\"blocked\" with empty steps and block_reason.",
		"If business_request.entities.warehouse_id exists, use it for warehouseId path param.",
		"If business_request.entities.chrt_ids exists and is non-empty, use it for chrtIds body field.",
		"If business_request.period.from exists, use it for dateFrom.",
		"If warehouse_id, chrt_ids, and period.from are present, do not return needs_clarification for inventory and sales planning.",
		"For /api/v3/stocks/{warehouseId}, body.chrtIds must be bound to input chrt_ids or contain the provided chrt_ids values. It must not be an empty array when business_request.entities.chrt_ids exists.",
		"For /api/v1/supplier/sales, use only query param dateFrom unless registry_candidates schema explicitly contains another query param.",
	}, "\n\n")
}

func parseApiExecutionPlan(responseText string) (*entities.ApiExecutionPlan, error) {
	cleaned := cleanModelJSON(responseText)

	var plan entities.ApiExecutionPlan
	if err := json.Unmarshal([]byte(cleaned), &plan); err == nil {
		if err := validateParsedPlan(plan); err != nil {
			return nil, err
		}

		return &plan, nil
	}

	var raw rawApiExecutionPlan
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parse ApiExecutionPlan JSON: %w; response=%s", err, responseText)
	}

	plan = raw.toPlan()

	if err := validateParsedPlan(plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

func cleanModelJSON(responseText string) string {
	cleaned := strings.TrimSpace(responseText)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")

	return strings.TrimSpace(cleaned)
}

func validateParsedPlan(plan entities.ApiExecutionPlan) error {
	if plan.SchemaVersion == "" {
		return fmt.Errorf("ApiExecutionPlan.schema_version is empty")
	}
	if plan.Status == "" {
		return fmt.Errorf("ApiExecutionPlan.status is empty")
	}
	if plan.Marketplace != "wildberries" {
		return fmt.Errorf("ApiExecutionPlan.marketplace must be wildberries")
	}
	if plan.Inputs == nil {
		return fmt.Errorf("ApiExecutionPlan.inputs is nil")
	}
	if plan.Steps == nil {
		return fmt.Errorf("ApiExecutionPlan.steps is nil")
	}
	if plan.Transforms == nil {
		return fmt.Errorf("ApiExecutionPlan.transforms is nil")
	}
	if plan.Warnings == nil {
		return fmt.Errorf("ApiExecutionPlan.warnings is nil")
	}
	if plan.Validation.Errors == nil {
		return fmt.Errorf("ApiExecutionPlan.validation.errors is nil")
	}

	return nil
}

type rawApiExecutionPlan struct {
	SchemaVersion          string                         `json:"schema_version"`
	RequestID              string                         `json:"request_id"`
	Marketplace            string                         `json:"marketplace"`
	Status                 string                         `json:"status"`
	Intent                 string                         `json:"intent"`
	NaturalLanguageSummary string                         `json:"natural_language_summary,omitempty"`
	RiskLevel              string                         `json:"risk_level"`
	RequiresApproval       bool                           `json:"requires_approval"`
	ApprovalReason         string                         `json:"approval_reason,omitempty"`
	BlockReason            string                         `json:"block_reason,omitempty"`
	ClarifyingQuestions    []string                       `json:"clarifying_questions,omitempty"`
	ExecutionMode          string                         `json:"execution_mode"`
	Inputs                 map[string]entities.InputValue `json:"inputs"`
	Steps                  []rawApiPlanStep               `json:"steps"`
	Transforms             []entities.TransformStep       `json:"transforms"`
	FinalOutput            entities.FinalOutput           `json:"final_output"`
	Warnings               []entities.PlanWarning         `json:"warnings"`
	Validation             entities.PlanValidation        `json:"validation"`
	Metadata               *entities.RequestMetadata      `json:"metadata,omitempty"`
}

type rawApiPlanStep struct {
	StepID          string                    `json:"step_id"`
	OperationID     string                    `json:"operation_id"`
	SourceFile      string                    `json:"source_file"`
	Readonly        bool                      `json:"readonly"`
	RiskLevel       string                    `json:"risk_level"`
	Purpose         string                    `json:"purpose"`
	DependsOn       []string                  `json:"depends_on"`
	Request         rawHttpRequestTemplate    `json:"request"`
	Pagination      entities.PaginationPlan   `json:"pagination"`
	RetryPolicy     entities.RetryPolicy      `json:"retry_policy"`
	RateLimitPolicy entities.RateLimitPolicy  `json:"rate_limit_policy"`
	ResponseMapping entities.ResponseMapping  `json:"response_mapping"`
	ValidationRules []entities.ValidationRule `json:"validation_rules,omitempty"`
}

type rawHttpRequestTemplate struct {
	ServerURL    string                            `json:"server_url"`
	Method       string                            `json:"method"`
	PathTemplate string                            `json:"path_template"`
	PathParams   map[string]any                    `json:"path_params"`
	QueryParams  map[string]any                    `json:"query_params"`
	Headers      map[string]entities.HeaderBinding `json:"headers"`
	Body         any                               `json:"body"`
	ContentType  string                            `json:"content_type"`
	Accept       string                            `json:"accept"`
}

func (p rawApiExecutionPlan) toPlan() entities.ApiExecutionPlan {
	steps := make([]entities.ApiPlanStep, 0, len(p.Steps))
	for _, step := range p.Steps {
		steps = append(steps, step.toStep())
	}

	return entities.ApiExecutionPlan{
		SchemaVersion:          p.SchemaVersion,
		RequestID:              p.RequestID,
		Marketplace:            p.Marketplace,
		Status:                 p.Status,
		Intent:                 p.Intent,
		NaturalLanguageSummary: p.NaturalLanguageSummary,
		RiskLevel:              p.RiskLevel,
		RequiresApproval:       p.RequiresApproval,
		ApprovalReason:         p.ApprovalReason,
		BlockReason:            p.BlockReason,
		ClarifyingQuestions:    nonNilStringSlice(p.ClarifyingQuestions),
		ExecutionMode:          p.ExecutionMode,
		Inputs:                 nonNilInputs(p.Inputs),
		Steps:                  steps,
		Transforms:             nonNilTransforms(p.Transforms),
		FinalOutput:            p.FinalOutput,
		Warnings:               nonNilWarnings(p.Warnings),
		Validation:             normalizeValidation(p.Validation),
		Metadata:               p.Metadata,
	}
}

func (s rawApiPlanStep) toStep() entities.ApiPlanStep {
	return entities.ApiPlanStep{
		StepID:          s.StepID,
		OperationID:     s.OperationID,
		SourceFile:      s.SourceFile,
		Readonly:        s.Readonly,
		RiskLevel:       s.RiskLevel,
		Purpose:         s.Purpose,
		DependsOn:       nonNilStringSlice(s.DependsOn),
		Request:         s.Request.toRequest(),
		Pagination:      s.Pagination,
		RetryPolicy:     s.RetryPolicy,
		RateLimitPolicy: s.RateLimitPolicy,
		ResponseMapping: s.ResponseMapping,
		ValidationRules: s.ValidationRules,
	}
}

func (r rawHttpRequestTemplate) toRequest() entities.HttpRequestTemplate {
	return entities.HttpRequestTemplate{
		ServerURL:    r.ServerURL,
		Method:       r.Method,
		PathTemplate: r.PathTemplate,
		PathParams:   normalizeValueBindings(r.PathParams),
		QueryParams:  normalizeValueBindings(r.QueryParams),
		Headers:      nonNilHeaders(r.Headers),
		Body:         normalizeBodyBindings(r.Body),
		ContentType:  r.ContentType,
		Accept:       r.Accept,
	}
}

func normalizeValueBindings(values map[string]any) map[string]entities.ValueBinding {
	result := make(map[string]entities.ValueBinding)

	for key, value := range values {
		switch typed := value.(type) {
		case entities.ValueBinding:
			result[key] = typed
		case map[string]any:
			result[key] = valueBindingFromMap(typed)
		default:
			// WHY: LLMs often return literal params; normalize them so executor still receives a binding-shaped plan.
			result[key] = entities.ValueBinding{
				Source:   "static",
				Value:    typed,
				Required: true,
			}
		}
	}

	return result
}

func normalizeBodyBindings(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if itemMap, ok := item.(map[string]any); ok {
				if source, ok := itemMap["source"].(string); ok && source != "" {
					result[key] = valueBindingFromMap(itemMap)
					continue
				}
			}

			result[key] = item
		}

		return result
	default:
		return value
	}
}

func valueBindingFromMap(value map[string]any) entities.ValueBinding {
	binding := entities.ValueBinding{
		Source:     stringFromAny(value["source"]),
		Value:      value["value"],
		InputName:  stringFromAny(value["input_name"]),
		StepID:     stringFromAny(value["step_id"]),
		OutputName: stringFromAny(value["output_name"]),
		Expression: stringFromAny(value["expression"]),
		SecretName: stringFromAny(value["secret_name"]),
		Required:   boolFromAny(value["required"]),
	}

	if binding.Source == "" {
		binding.Source = "static"
	}
	if _, ok := value["required"]; !ok {
		binding.Required = true
	}

	return binding
}

func stringFromAny(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}

	return typed
}

func boolFromAny(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}

	return typed
}

func nonNilInputs(value map[string]entities.InputValue) map[string]entities.InputValue {
	if value == nil {
		return map[string]entities.InputValue{}
	}

	return value
}

func nonNilHeaders(value map[string]entities.HeaderBinding) map[string]entities.HeaderBinding {
	if value == nil {
		return map[string]entities.HeaderBinding{}
	}

	return value
}

func nonNilTransforms(value []entities.TransformStep) []entities.TransformStep {
	if value == nil {
		return []entities.TransformStep{}
	}

	return value
}

func nonNilWarnings(value []entities.PlanWarning) []entities.PlanWarning {
	if value == nil {
		return []entities.PlanWarning{}
	}

	return value
}

func nonNilStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}

	return value
}

func normalizeValidation(value entities.PlanValidation) entities.PlanValidation {
	if value.Errors == nil {
		value.Errors = []string{}
	}

	return value
}

func registryCandidatesForPlanner(candidates []entities.WBRegistryOperation) []RegistryCandidateForPlanner {
	result := make([]RegistryCandidateForPlanner, 0, len(candidates))

	for _, candidate := range candidates {
		result = append(result, RegistryCandidateForPlanner{
			OperationID:              candidate.OperationID,
			SourceFile:               candidate.SourceFile,
			Method:                   candidate.Method,
			ServerURL:                candidate.ServerURL,
			PathTemplate:             candidate.PathTemplate,
			Tags:                     candidate.Tags,
			Category:                 candidate.Category,
			Summary:                  candidate.Summary,
			Description:              candidate.Description,
			Readonly:                 candidate.XReadonlyMethod,
			RiskLevel:                riskLevel(candidate.XReadonlyMethod),
			XCategory:                candidate.XCategory,
			XTokenTypes:              candidate.XTokenTypes,
			PathParamsSchemaJSON:     candidate.PathParamsSchemaJSON,
			QueryParamsSchemaJSON:    candidate.QueryParamsSchemaJSON,
			HeadersSchemaJSON:        candidate.HeadersSchemaJSON,
			RequestBodySchemaJSON:    candidate.RequestBodySchemaJSON,
			ResponseSchemaJSON:       candidate.ResponseSchemaJSON,
			RateLimitNotes:           candidate.RateLimitNotes,
			SubscriptionRequirements: candidate.SubscriptionRequirements,
			RequiresJam:              candidate.RequiresJam,
		})
	}

	return result
}

func riskLevel(readonly *bool) string {
	if readonly == nil {
		return "unknown"
	}

	if *readonly {
		return "read"
	}

	return "write"
}

func contentText(content *genai.Content) string {
	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func requiredQuestions(request entities.BusinessRequest) []string {
	questions := make([]string, 0)

	if strings.TrimSpace(request.RequestID) == "" {
		questions = append(questions, "Provide request_id.")
	}

	if strings.TrimSpace(request.Marketplace) == "" {
		questions = append(questions, "Provide marketplace.")
	}

	if request.Marketplace != "" && request.Marketplace != "wildberries" {
		questions = append(questions, "Only marketplace=wildberries is supported.")
	}

	if strings.TrimSpace(request.Intent) == "" {
		questions = append(questions, "Provide intent.")
	}

	if strings.TrimSpace(request.NaturalLanguageRequest) == "" {
		questions = append(questions, "Provide natural_language_request.")
	}

	return questions
}

func buildRegistrySearchQuery(request entities.BusinessRequest) string {
	parts := []string{
		request.Intent,
		request.NaturalLanguageRequest,
	}

	for key, value := range request.Entities {
		parts = append(parts, key, stringifyEntityValue(value))
	}

	return strings.Join(parts, " ")
}

func stringifyEntityValue(value any) string {
	return fmt.Sprint(value)
}

func effectiveMaxSteps(maxSteps int) int {
	if maxSteps <= 0 {
		return 10
	}

	if maxSteps > 20 {
		return 20
	}

	return maxSteps
}
