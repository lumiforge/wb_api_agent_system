package wb_api_agent

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
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

func TestPlanPostProcessorInventoryAndSalesGolden(t *testing.T) {
	request := entities.BusinessRequest{
		RequestID:              "req_test_001",
		Marketplace:            "wildberries",
		Intent:                 "get_inventory_and_sales",
		NaturalLanguageRequest: "Получить остатки на складе 507 и продажи за период",
		Entities: map[string]any{
			"warehouse_id": float64(507),
			"chrt_ids":     []any{float64(12345678), float64(87654321)},
		},
		Period: &entities.Period{
			From: "2026-03-26",
			To:   "2026-04-26",
		},
		Constraints: entities.BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
			MaxSteps:          10,
			ExecutionMode:     "automatic",
		},
	}

	plan := &entities.ApiExecutionPlan{
		SchemaVersion:          "1.0",
		RequestID:              "req_test_001",
		Marketplace:            "wildberries",
		Status:                 "ready",
		Intent:                 "get_inventory_and_sales",
		NaturalLanguageSummary: "Получить остатки на складе 507 и продажи за период",
		RiskLevel:              "read",
		RequiresApproval:       false,
		ExecutionMode:          "automatic",
		Inputs: map[string]entities.InputValue{
			"warehouseId": {
				Type:        "integer",
				Required:    true,
				Value:       float64(507),
				Description: "ID склада",
			},
			"chrtIds": {
				Type:        "array",
				Required:    true,
				Value:       []any{float64(12345678), float64(87654321)},
				Description: "Массив ID размеров товаров",
			},
			"dateFrom": {
				Type:        "string",
				Required:    true,
				Value:       "2026-03-26",
				Description: "Дата начала периода",
			},
		},
		Steps: []entities.ApiPlanStep{
			{
				StepID:      "get_inventory",
				OperationID: "generated_post_api_v3_stocks_warehouseid",
				SourceFile:  "products.yaml",
				Readonly:    true,
				RiskLevel:   "read",
				Purpose:     "Получить остатки товаров на складе",
				DependsOn:   []string{},
				Request: entities.HttpRequestTemplate{
					ServerURL:    "https://marketplace-api.wildberries.ru",
					Method:       "POST",
					PathTemplate: "/api/v3/stocks/{warehouseId}",
					PathParams: map[string]entities.ValueBinding{
						"warehouseId": {
							Source:   "static",
							Value:    float64(507),
							Required: true,
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
						"chrtIds": []any{float64(12345678), float64(87654321)},
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
					Bucket:        "generated_post_api_v3_stocks_warehouseid",
					MaxRequests:   1,
					PeriodSeconds: 60,
					MinIntervalMS: 1000,
				},
				ResponseMapping: entities.ResponseMapping{
					Outputs: map[string]entities.MappedOutput{},
				},
			},
			{
				StepID:      "get_sales",
				OperationID: "generated_get_api_v1_supplier_sales",
				SourceFile:  "reports.yaml",
				Readonly:    true,
				RiskLevel:   "read",
				Purpose:     "Получить данные о продажах за указанный период",
				DependsOn:   []string{},
				Request: entities.HttpRequestTemplate{
					ServerURL:    "https://statistics-api.wildberries.ru",
					Method:       "GET",
					PathTemplate: "/api/v1/supplier/sales",
					PathParams:   map[string]entities.ValueBinding{},
					QueryParams: map[string]entities.ValueBinding{
						"dateFrom": {
							Source:   "static",
							Value:    "2026-03-26",
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
					RetryOnStatus: []int{429, 500, 502, 503, 504},
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
					Outputs: map[string]entities.MappedOutput{},
				},
			},
		},
		Transforms: []entities.TransformStep{},
		FinalOutput: entities.FinalOutput{
			Type:        "object",
			Description: "Получить остатки на складе 507 и продажи за период",
			Fields:      map[string]any{},
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

	normalizePlan(request, plan)

	processor := NewPlanPostProcessor(&testRegistry{
		operations: map[string]entities.WBRegistryOperation{
			"generated_post_api_v3_stocks_warehouseid": testStocksOperation(),
			"generated_get_api_v1_supplier_sales":      testSalesOperation(),
		},
	})

	validationPlan, err := processor.Process(context.Background(), request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if validationPlan != nil {
		t.Fatalf("expected valid plan, got validation replacement: %#v", validationPlan)
	}

	gotJSON := mustPrettyJSON(t, plan)
	wantJSON := mustReadFile(t, "testdata/get_inventory_and_sales_ready.golden.json")

	assertJSONEqual(t, wantJSON, gotJSON)
}
func assertJSONEqual(t *testing.T, wantJSON string, gotJSON string) {
	t.Helper()

	var want any
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}

	var got any
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(want, got) {
		wantPretty, err := json.MarshalIndent(want, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		gotPretty, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(wantPretty), string(gotPretty))
	}
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
	if validationPlan.BlockReason != "adk_plan_failed_registry_validation" {
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

func mustPrettyJSON(t *testing.T, value any) string {
	t.Helper()

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	return string(payload) + "\n"
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(payload)
}

func TestValuesEquivalent(t *testing.T) {
	if !valuesEquivalent([]any{float64(1), float64(2)}, []int{1, 2}) {
		t.Fatal("expected numeric arrays to be equivalent")
	}

	if reflect.DeepEqual(unwrapInputValue(entities.InputValue{Value: "x"}), entities.InputValue{Value: "x"}) {
		t.Fatal("expected nested InputValue to be unwrapped")
	}
}
