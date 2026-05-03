package wb_api_agent

import (
	"context"
	"encoding/json"
	"errors"
	composer "github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/composer"
	orch "github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/orchestration"
	"strings"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func TestMissingBusinessInputQuestionsSkipsEmptyQuestions(t *testing.T) {
	questions := missingBusinessInputQuestions([]entities.MissingBusinessInput{
		{
			Code:         "warehouse",
			UserQuestion: "Provide the seller warehouse.",
		},
		{
			Code:         "empty",
			UserQuestion: "",
		},
		{
			Code:         "period",
			UserQuestion: "Provide the period start date.",
		},
	})

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %#v", questions)
	}

	if questions[0] != "Provide the seller warehouse." {
		t.Fatalf("unexpected first question %q", questions[0])
	}

	if questions[1] != "Provide the period start date." {
		t.Fatalf("unexpected second question %q", questions[1])
	}
}

func TestPlanWithSelectorComposerReturnsNeedsClarification(t *testing.T) {
	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: &entities.OperationSelectionPlan{
				SchemaVersion: "1.0",
				RequestID:     "request-1",
				Marketplace:   "wildberries",
				Status:        entities.OperationSelectionStatusNeedsClarification,
				SelectedOperations: []entities.SelectedOperation{
					{
						OperationID:   "operation_stocks",
						Purpose:       "Fetch stocks.",
						DependsOn:     []string{},
						InputStrategy: entities.OperationInputStrategyBusinessEntities,
					},
				},
				MissingInputs: []entities.MissingBusinessInput{
					{
						Code:           "warehouse",
						UserQuestion:   "Provide the seller warehouse.",
						Accepts:        []string{"warehouse ID", "warehouse name"},
						InternalFields: []string{"warehouse_id"},
					},
				},
				RejectedCandidates: []entities.RejectedOperationCandidate{},
				Warnings:           []entities.PlanWarning{},
			},
		},
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{validPipelineRegistryOperation("operation_stocks")},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "needs_clarification" {
		t.Fatalf("expected needs_clarification, got %q", plan.Status)
	}

	if len(plan.ClarifyingQuestions) != 1 || plan.ClarifyingQuestions[0] != "Provide the seller warehouse." {
		t.Fatalf("unexpected clarifying questions %#v", plan.ClarifyingQuestions)
	}
}

func TestPlanWithSelectorComposerReturnsUnsupportedAsBlockedPlan(t *testing.T) {
	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: &entities.OperationSelectionPlan{
				SchemaVersion:      "1.0",
				RequestID:          "request-1",
				Marketplace:        "wildberries",
				Status:             entities.OperationSelectionStatusUnsupported,
				UserFacingSummary:  "No matching WB API operation.",
				SelectedOperations: []entities.SelectedOperation{},
				MissingInputs:      []entities.MissingBusinessInput{},
				RejectedCandidates: []entities.RejectedOperationCandidate{},
				Warnings:           []entities.PlanWarning{},
			},
		},
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{validPipelineRegistryOperation("operation_stocks")},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %q", plan.Status)
	}

	if plan.BlockReason != "operation_selection_unsupported" {
		t.Fatalf("unexpected block reason %q", plan.BlockReason)
	}
}

func TestPlanWithSelectorComposerReturnsBlockedPlan(t *testing.T) {
	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: &entities.OperationSelectionPlan{
				SchemaVersion:      "1.0",
				RequestID:          "request-1",
				Marketplace:        "wildberries",
				Status:             entities.OperationSelectionStatusBlocked,
				UserFacingSummary:  "Request is blocked by policy.",
				SelectedOperations: []entities.SelectedOperation{},
				MissingInputs:      []entities.MissingBusinessInput{},
				RejectedCandidates: []entities.RejectedOperationCandidate{},
				Warnings:           []entities.PlanWarning{},
			},
		},
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{validPipelineRegistryOperation("operation_stocks")},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %q", plan.Status)
	}

	if plan.BlockReason != "operation_selection_blocked" {
		t.Fatalf("unexpected block reason %q", plan.BlockReason)
	}
}

func TestPlanWithSelectorComposerReturnsReadyPlan(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")

	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: validPipelineSelectionPlan("operation_stocks"),
		},
		operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(),
		apiPlanComposer: &fakeApiPlanComposer{
			plan: validPipelineExecutionPlan(registryOperation),
		},
		postProcessor: orch.NewPlanPostProcessor(&singleOperationRegistry{
			operation: registryOperation,
		}),
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{registryOperation},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "ready" {
		t.Fatalf("expected ready, got %q", plan.Status)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}
}

func TestPlanWithSelectorComposerPropagatesResolvedPeriodWhenRequestPeriodMissing(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")
	fakeComposer := &fakeApiPlanComposer{plan: validPipelineExecutionPlan(registryOperation)}
	agent := &Agent{operationSelector: &fakeOperationSelector{plan: &entities.OperationSelectionPlan{SchemaVersion: "1.0", RequestID: "request-1", Marketplace: "wildberries", Status: entities.OperationSelectionStatusReadyForComposition, SelectedOperations: []entities.SelectedOperation{{OperationID: "operation_stocks", Purpose: "Fetch stocks.", DependsOn: []string{}, InputStrategy: entities.OperationInputStrategyBusinessEntities}}, MissingInputs: []entities.MissingBusinessInput{}, RejectedCandidates: []entities.RejectedOperationCandidate{}, Warnings: []entities.PlanWarning{}, ResolvedInputs: entities.OperationSelectionResolvedInputs{Period: &entities.Period{From: "2026-04-01", To: "2026-04-30"}}}}, operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(), apiPlanComposer: fakeComposer, postProcessor: orch.NewPlanPostProcessor(&singleOperationRegistry{operation: registryOperation})}
	_, err := agent.planWithSelectorComposer(context.Background(), validPipelineBusinessRequest(), []entities.WBRegistryOperation{registryOperation})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeComposer.lastInput.BusinessRequest.Period == nil || fakeComposer.lastInput.BusinessRequest.Period.From != "2026-04-01" || fakeComposer.lastInput.BusinessRequest.Period.To != "2026-04-30" {
		t.Fatalf("expected propagated period, got %#v", fakeComposer.lastInput.BusinessRequest.Period)
	}
}

func TestPlanWithSelectorComposerDoesNotOverwriteExistingRequestPeriod(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")
	fakeComposer := &fakeApiPlanComposer{plan: validPipelineExecutionPlan(registryOperation)}
	request := validPipelineBusinessRequest()
	request.Period = &entities.Period{From: "2026-03-01", To: "2026-03-31"}
	agent := &Agent{operationSelector: &fakeOperationSelector{plan: &entities.OperationSelectionPlan{SchemaVersion: "1.0", RequestID: "request-1", Marketplace: "wildberries", Status: entities.OperationSelectionStatusReadyForComposition, SelectedOperations: []entities.SelectedOperation{{OperationID: "operation_stocks", Purpose: "Fetch stocks.", DependsOn: []string{}, InputStrategy: entities.OperationInputStrategyBusinessEntities}}, MissingInputs: []entities.MissingBusinessInput{}, RejectedCandidates: []entities.RejectedOperationCandidate{}, Warnings: []entities.PlanWarning{}, ResolvedInputs: entities.OperationSelectionResolvedInputs{Period: &entities.Period{From: "2026-04-01", To: "2026-04-30"}}}}, operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(), apiPlanComposer: fakeComposer, postProcessor: orch.NewPlanPostProcessor(&singleOperationRegistry{operation: registryOperation})}
	_, err := agent.planWithSelectorComposer(context.Background(), request, []entities.WBRegistryOperation{registryOperation})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeComposer.lastInput.BusinessRequest.Period == nil || fakeComposer.lastInput.BusinessRequest.Period.From != "2026-03-01" || fakeComposer.lastInput.BusinessRequest.Period.To != "2026-03-31" {
		t.Fatalf("expected existing period preserved, got %#v", fakeComposer.lastInput.BusinessRequest.Period)
	}
}
func TestPlanWithSelectorComposerReturnsBlockedWhenSelectorFails(t *testing.T) {
	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			err: errors.New("selector failed"),
		},
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{validPipelineRegistryOperation("operation_stocks")},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %q", plan.Status)
	}

	if plan.BlockReason != "operation_selector_failed" {
		t.Fatalf("unexpected block reason %q", plan.BlockReason)
	}
}

func TestPlanWithSelectorComposerReturnsBlockedWhenResolutionFails(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")

	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: validPipelineSelectionPlan("invented_operation"),
		},
		operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(),
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{registryOperation},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %q", plan.Status)
	}

	if plan.BlockReason != "operation_selection_registry_resolution_failed" {
		t.Fatalf("unexpected block reason %q", plan.BlockReason)
	}
}

func TestPlanWithSelectorComposerReturnsBlockedWhenCompositionUnsupported(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")
	agent := &Agent{operationSelector: &fakeOperationSelector{plan: validPipelineSelectionPlan("operation_stocks")}, operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(), apiPlanComposer: &fakeApiPlanComposer{err: composer.NewApiPlanCompositionUnsupportedError("request-1", "required_request_body_not_composable", "operation requires request body that deterministic composer cannot build from explicit business request fields")}}
	plan, err := agent.planWithSelectorComposer(context.Background(), validPipelineBusinessRequest(), []entities.WBRegistryOperation{registryOperation})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plan.Status != "blocked" || plan.BlockReason != "api_plan_composition_failed" {
		t.Fatalf("unexpected plan %#v", plan)
	}
	if len(plan.Warnings) == 0 || plan.Warnings[0].Code != "api_plan_composition_error" {
		t.Fatalf("expected api_plan_composition_error warning, got %#v", plan.Warnings)
	}
	raw, _ := json.Marshal(plan)
	if strings.Contains(string(raw), "ready_for_composition") || strings.Contains(string(raw), "selected_operations") {
		t.Fatalf("plan leaked selector payload: %s", string(raw))
	}
}

func TestPlanWithSelectorComposerReturnsBlockedWhenCompositionFails(t *testing.T) {
	registryOperation := validPipelineRegistryOperation("operation_stocks")

	agent := &Agent{
		operationSelector: &fakeOperationSelector{
			plan: validPipelineSelectionPlan("operation_stocks"),
		},
		operationSelectionResolver: orch.NewOperationSelectionRegistryResolver(),
		apiPlanComposer: &fakeApiPlanComposer{
			err: errors.New("composition failed"),
		},
	}

	plan, err := agent.planWithSelectorComposer(
		context.Background(),
		validPipelineBusinessRequest(),
		[]entities.WBRegistryOperation{registryOperation},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %q", plan.Status)
	}

	if plan.BlockReason != "api_plan_composition_failed" {
		t.Fatalf("unexpected block reason %q", plan.BlockReason)
	}
}

type fakeOperationSelector struct {
	plan *entities.OperationSelectionPlan
	err  error
}

func (s *fakeOperationSelector) SelectOperations(
	ctx context.Context,
	input entities.OperationSelectionInput,
) (*entities.OperationSelectionPlan, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.plan, nil
}

type fakeApiPlanComposer struct {
	plan      *entities.ApiExecutionPlan
	err       error
	lastInput entities.ApiPlanCompositionInput
}

func (c *fakeApiPlanComposer) Compose(
	ctx context.Context,
	input entities.ApiPlanCompositionInput,
) (*entities.ApiExecutionPlan, error) {
	c.lastInput = input

	if c.err != nil {
		return nil, c.err
	}

	return c.plan, nil
}

func validPipelineBusinessRequest() entities.BusinessRequest {
	return entities.BusinessRequest{
		RequestID:              "request-1",
		Marketplace:            "wildberries",
		NaturalLanguageRequest: "Покажи остатки",
		Entities: map[string]any{
			"warehouse_id": 507,
			"chrt_ids":     []int{12345678},
		},
		Constraints: entities.BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
		},
	}
}

func validPipelineSelectionPlan(operationID string) *entities.OperationSelectionPlan {
	return &entities.OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        entities.OperationSelectionStatusReadyForComposition,
		SelectedOperations: []entities.SelectedOperation{
			{
				OperationID:   operationID,
				Purpose:       "Fetch stocks.",
				DependsOn:     []string{},
				InputStrategy: entities.OperationInputStrategyBusinessEntities,
			},
		},
		MissingInputs:      []entities.MissingBusinessInput{},
		RejectedCandidates: []entities.RejectedOperationCandidate{},
		Warnings:           []entities.PlanWarning{},
	}
}

func validPipelineRegistryOperation(operationID string) entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:           "wildberries",
		SourceFile:            "products.yaml",
		OperationID:           operationID,
		Method:                "POST",
		ServerURL:             "https://marketplace-api.wildberries.ru",
		PathTemplate:          "/api/v3/stocks/{warehouseId}",
		Tags:                  []string{"Остатки"},
		Category:              "marketplace",
		Summary:               "Остатки товаров",
		Description:           "Возвращает остатки товаров.",
		XReadonlyMethod:       &readonly,
		XCategory:             "marketplace",
		XTokenTypes:           []string{},
		PathParamsSchemaJSON:  `{"warehouseId":{"required":true}}`,
		QueryParamsSchemaJSON: "{}",
		HeadersSchemaJSON:     "{}",
		RequestBodySchemaJSON: `{
			"content": {
				"application/json": {
					"schema": {
						"type": "object",
						"required": ["chrtIds"],
						"properties": {
							"chrtIds": {
								"type": "array"
							}
						}
					}
				}
			}
		}`,
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}

func validPipelineExecutionPlan(operation entities.WBRegistryOperation) *entities.ApiExecutionPlan {
	return &entities.ApiExecutionPlan{
		SchemaVersion:          "1.0",
		RequestID:              "request-1",
		Marketplace:            "wildberries",
		Status:                 "ready",
		Intent:                 "",
		NaturalLanguageSummary: "Покажи остатки",
		RiskLevel:              "read",
		RequiresApproval:       false,
		ExecutionMode:          "automatic",
		Inputs: map[string]entities.InputValue{
			"warehouse_id": {
				Type:        "integer",
				Required:    true,
				Value:       507,
				Description: "Seller warehouse ID.",
			},
			"chrt_ids": {
				Type:        "array",
				Required:    true,
				Value:       []int{12345678},
				Description: "Product size IDs.",
			},
		},
		Steps: []entities.ApiPlanStep{
			{
				StepID:      "operation_stocks",
				OperationID: operation.OperationID,
				SourceFile:  operation.SourceFile,
				Readonly:    true,
				RiskLevel:   "read",
				Purpose:     "Fetch stocks.",
				DependsOn:   []string{},
				Request: entities.HttpRequestTemplate{
					ServerURL:    operation.ServerURL,
					Method:       operation.Method,
					PathTemplate: operation.PathTemplate,
					PathParams: map[string]entities.ValueBinding{
						"warehouseId": {
							Source:    "input",
							InputName: "warehouse_id",
							Required:  true,
						},
					},
					QueryParams: map[string]entities.ValueBinding{},
					Headers: map[string]entities.HeaderBinding{
						"Authorization": {
							Source:     "executor_secret",
							SecretName: "WB_AUTHORIZATION",
							Required:   true,
						},
					},
					Body: map[string]any{
						"chrtIds": entities.ValueBinding{
							Source:    "input",
							InputName: "chrt_ids",
							Required:  true,
						},
					},
					ContentType: "application/json",
					Accept:      "application/json",
				},
				Pagination: entities.PaginationPlan{
					Enabled:  false,
					Strategy: "none",
				},
				RetryPolicy: entities.RetryPolicy{
					Enabled:       true,
					MaxAttempts:   3,
					RetryOnStatus: []int{429, 500, 502, 503, 504},
					Backoff: entities.BackoffPolicy{
						Type:           "exponential",
						InitialDelayMS: 1000,
						MaxDelayMS:     20000,
					},
				},
				RateLimitPolicy: entities.RateLimitPolicy{
					Enabled:       true,
					Bucket:        operation.OperationID,
					MaxRequests:   1,
					PeriodSeconds: 60,
					MinIntervalMS: 1000,
				},
				ResponseMapping: entities.ResponseMapping{
					Outputs: map[string]entities.MappedOutput{
						"raw": {
							Type: "object",
							Path: "$",
						},
					},
					PostFilters: []entities.PostFilter{},
				},
			},
		},
		Transforms: []entities.TransformStep{},
		FinalOutput: entities.FinalOutput{
			Type:        "object",
			Description: "Покажи остатки",
			Fields: map[string]any{
				"raw": "steps.operation_stocks.outputs.raw",
			},
		},
		Warnings: []entities.PlanWarning{},
		Validation: entities.PlanValidation{
			RegistryChecked:       true,
			OutputSchemaChecked:   true,
			ReadonlyPolicyChecked: true,
			SecretsPolicyChecked:  true,
			JamPolicyChecked:      true,
			Errors:                []string{},
		},
	}
}

func assertStringContainsForPipeline(t *testing.T, value string, expected string) {
	t.Helper()

	if !strings.Contains(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}
