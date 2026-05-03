package orchestration

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

type testRegistry struct {
	operations map[string]entities.WBRegistryOperation
}

func (r *testRegistry) GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error) {
	operation, ok := r.operations[operationID]
	if !ok {
		return nil, nil
	}

	return &operation, nil
}
func (r *testRegistry) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}
func (r *testRegistry) SearchOperations(ctx context.Context, query wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	operations := make([]entities.WBRegistryOperation, 0, len(r.operations))
	for _, operation := range r.operations {
		operations = append(operations, operation)
	}

	return operations, nil
}

func TestPlanPostProcessorBlocksUnknownQueryParam(t *testing.T) {
	request := entities.BusinessRequest{
		RequestID:              "req_test_002",
		Marketplace:            "wildberries",
		Intent:                 "get_sales",
		NaturalLanguageRequest: "Получить продажи",
		Entities:               map[string]any{},
		Constraints: entities.BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
			MaxSteps:          10,
			ExecutionMode:     "automatic",
		},
	}

	plan := &entities.ApiExecutionPlan{
		SchemaVersion:    "1.0",
		RequestID:        "req_test_002",
		Marketplace:      "wildberries",
		Status:           "ready",
		Intent:           "get_sales",
		RiskLevel:        "read",
		RequiresApproval: false,
		ExecutionMode:    "automatic",
		Inputs: map[string]entities.InputValue{
			"date_from": {
				Type:        "string",
				Required:    true,
				Value:       "2026-03-26",
				Description: "Period start date.",
			},
		},
		Steps: []entities.ApiPlanStep{
			{
				StepID:      "get_sales",
				OperationID: "generated_get_api_v1_supplier_sales",
				SourceFile:  "reports.yaml",
				Readonly:    true,
				RiskLevel:   "read",
				Purpose:     "Получить продажи",
				DependsOn:   []string{},
				Request: entities.HttpRequestTemplate{
					ServerURL:    "https://statistics-api.wildberries.ru",
					Method:       "GET",
					PathTemplate: "/api/v1/supplier/sales",
					PathParams:   map[string]entities.ValueBinding{},
					QueryParams: map[string]entities.ValueBinding{
						"dateFrom": {
							Source:    "input",
							InputName: "date_from",
							Required:  true,
						},
						"dateTo": {
							Source:   "static",
							Value:    "2026-04-26",
							Required: true,
						},
					},
					Headers: map[string]entities.HeaderBinding{
						"Authorization": {
							Source:     "executor_secret",
							SecretName: "WB_AUTHORIZATION",
							Required:   true,
						},
					},
					Body:        map[string]any{},
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
					RetryOnStatus: []int{429},
					Backoff: entities.BackoffPolicy{
						Type:           "exponential",
						InitialDelayMS: 1000,
						MaxDelayMS:     20000,
					},
				},
				RateLimitPolicy: entities.RateLimitPolicy{
					Enabled:       true,
					Bucket:        "generated_get_api_v1_supplier_sales",
					MaxRequests:   1,
					PeriodSeconds: 60,
					MinIntervalMS: 1000,
				},
				ResponseMapping: entities.ResponseMapping{
					Outputs: map[string]entities.MappedOutput{
						"sales": {
							Type: "rows",
							Path: "$",
						},
					},
				},
			},
		},
		Transforms: []entities.TransformStep{},
		FinalOutput: entities.FinalOutput{
			Type:        "object",
			Description: "Sales.",
			Fields: map[string]any{
				"sales": "steps.get_sales.outputs.sales",
			},
		},
		Warnings: []entities.PlanWarning{},
		Validation: entities.PlanValidation{
			Errors: []string{},
		},
	}

	processor := NewPlanPostProcessor(&testRegistry{
		operations: map[string]entities.WBRegistryOperation{
			"generated_get_api_v1_supplier_sales": testSalesOperation(),
		},
	})

	validationPlan, err := processor.Process(context.Background(), request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if validationPlan == nil {
		t.Fatal("expected blocked validation plan")
	}
	if validationPlan.Status != "blocked" {
		t.Fatalf("expected blocked status, got %s", validationPlan.Status)
	}
	if validationPlan.BlockReason != "plan_failed_registry_validation" {
		t.Fatalf("unexpected block reason: %s", validationPlan.BlockReason)
	}
}

func testStocksOperation() entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "products.yaml",
		OperationID:              "generated_post_api_v3_stocks_warehouseid",
		Method:                   "POST",
		ServerURL:                "https://marketplace-api.wildberries.ru",
		PathTemplate:             "/api/v3/stocks/{warehouseId}",
		Tags:                     []string{"Остатки на складах продавца"},
		Category:                 "Остатки на складах продавца",
		Summary:                  "Получить остатки товаров",
		Description:              "Метод возвращает данные об остатках товаров на складах продавца.",
		XReadonlyMethod:          &readonly,
		XCategory:                "marketplace",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        `{"Authorization":{"description":"WB API authorization header supplied by executor_secret.","in":"header","required":true,"type":"apiKey"}}`,
		RequestBodySchemaJSON:    `{"content":{"application/json":{"schema":{"nullable":false,"properties":{"chrtIds":{"description":"Массив ID размеров товаров","items":{"type":"integer"},"maxItems":1000,"type":"array"}},"required":["chrtIds"],"type":"object"}}},"required":true}`,
		ResponseSchemaJSON:       `{"200":{"content":{"application/json":{"schema":{"properties":{"stocks":{"items":{"type":"object"},"nullable":false,"type":"array"}},"type":"object"}},"description":"Успешно"}}`,
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}

func testSalesOperation() entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "reports.yaml",
		OperationID:              "generated_get_api_v1_supplier_sales",
		Method:                   "GET",
		ServerURL:                "https://statistics-api.wildberries.ru",
		PathTemplate:             "/api/v1/supplier/sales",
		Tags:                     []string{"Основные отчёты"},
		Category:                 "Основные отчёты",
		Summary:                  "Продажи",
		Description:              "Метод возвращает информацию о продажах и возвратах.",
		XReadonlyMethod:          &readonly,
		XCategory:                "statistics",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    `{"dateFrom":{"description":"Дата и время последнего изменения по продаже/возврату.","required":true,"schema":{"format":"date-time","type":"string"}}}`,
		HeadersSchemaJSON:        `{"Authorization":{"description":"WB API authorization header supplied by executor_secret.","in":"header","required":true,"type":"apiKey"}}`,
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       `{"200":{"content":{"application/json":{"schema":{"items":{"type":"object"},"type":"array"}},"description":"Успешно"}}}`,
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}

func TestPlanPostProcessorPreservesInputMetadata(t *testing.T) {
	request := entities.BusinessRequest{
		RequestID: "req_meta_1",
		Metadata: &entities.RequestMetadata{
			CorrelationID:     "corr_input",
			SessionID:         "sess_input",
			RunID:             "run_input",
			ToolCallID:        "call_input",
			ClientExecutionID: "exec_input",
		},
	}

	plan := &entities.ApiExecutionPlan{
		Status: "needs_clarification",
		Metadata: &entities.RequestMetadata{
			CorrelationID: "corr_llm",
		},
		ClarifyingQuestions: []string{"q1"},
	}

	processor := NewPlanPostProcessor(&testRegistry{operations: map[string]entities.WBRegistryOperation{}})

	validationPlan, err := processor.Process(context.Background(), request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if validationPlan != nil {
		t.Fatalf("expected nil validationPlan, got %#v", validationPlan)
	}
	if plan.Metadata == nil || plan.Metadata.CorrelationID != "corr_input" {
		t.Fatalf("expected metadata from request, got %#v", plan.Metadata)
	}
}

func TestPlanPostProcessorOverridesLLMMetadataInReplacementPlan(t *testing.T) {
	request := entities.BusinessRequest{
		RequestID: "req_meta_2",
		Metadata: &entities.RequestMetadata{
			CorrelationID: "corr_input",
			SessionID:     "sess_input",
		},
	}

	plan := &entities.ApiExecutionPlan{
		Status:   "blocked",
		Metadata: &entities.RequestMetadata{CorrelationID: "corr_llm"},
	}

	processor := NewPlanPostProcessor(&testRegistry{operations: map[string]entities.WBRegistryOperation{}})

	validationPlan, err := processor.Process(context.Background(), request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if validationPlan == nil {
		t.Fatal("expected replacement blocked plan")
	}
	if validationPlan.Metadata == nil || validationPlan.Metadata.CorrelationID != "corr_input" {
		t.Fatalf("expected replacement metadata from request, got %#v", validationPlan.Metadata)
	}
}
