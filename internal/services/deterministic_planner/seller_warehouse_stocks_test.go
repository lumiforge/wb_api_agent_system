package deterministic_planner

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

func (r *testRegistry) SearchOperations(ctx context.Context, query wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	operations := make([]entities.WBRegistryOperation, 0, len(r.operations))
	for _, operation := range r.operations {
		operations = append(operations, operation)
	}

	return operations, nil
}
func (r *testRegistry) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return wbregistry.Stats{}, nil
}
func TestSellerWarehouseStocksScenarioReady(t *testing.T) {
	registry := &testRegistry{
		operations: map[string]entities.WBRegistryOperation{
			"generated_post_api_v3_stocks_warehouseid": testStocksOperation(),
		},
	}

	scenario := NewSellerWarehouseStocksScenario(registry)

	request := entities.BusinessRequest{
		RequestID:              "req_test_deterministic",
		Marketplace:            "wildberries",
		Intent:                 "get_seller_warehouse_stocks",
		NaturalLanguageRequest: "Получить остатки товаров на складе продавца 507",
		Entities: map[string]any{
			"warehouse_id": float64(507),
			"chrt_ids":     []any{float64(12345678), float64(87654321)},
		},
		Constraints: entities.BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
			MaxSteps:          10,
			ExecutionMode:     "automatic",
		},
	}

	plan, handled, err := scenario.TryPlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected scenario to handle request")
	}
	if plan.Status != "ready" {
		t.Fatalf("expected ready plan, got %s", plan.Status)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].OperationID != "generated_post_api_v3_stocks_warehouseid" {
		t.Fatalf("unexpected operation_id: %s", plan.Steps[0].OperationID)
	}

	body, ok := plan.Steps[0].Request.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected request body map, got %T", plan.Steps[0].Request.Body)
	}

	binding, ok := body["chrtIds"].(entities.ValueBinding)
	if !ok {
		t.Fatalf("expected chrtIds ValueBinding, got %T", body["chrtIds"])
	}
	if binding.Source != "input" || binding.InputName != "chrt_ids" {
		t.Fatalf("unexpected chrtIds binding: %#v", binding)
	}
}

func TestSellerWarehouseStocksScenarioNeedsChrtIDs(t *testing.T) {
	registry := &testRegistry{
		operations: map[string]entities.WBRegistryOperation{
			"generated_post_api_v3_stocks_warehouseid": testStocksOperation(),
		},
	}

	scenario := NewSellerWarehouseStocksScenario(registry)

	request := entities.BusinessRequest{
		RequestID:              "req_test_deterministic_missing",
		Marketplace:            "wildberries",
		Intent:                 "get_seller_warehouse_stocks",
		NaturalLanguageRequest: "Получить остатки товаров на складе продавца 507",
		Entities: map[string]any{
			"warehouse_id": float64(507),
		},
		Constraints: entities.BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
			MaxSteps:          10,
			ExecutionMode:     "automatic",
		},
	}

	plan, handled, err := scenario.TryPlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected scenario to handle request")
	}
	if plan.Status != "needs_clarification" {
		t.Fatalf("expected needs_clarification, got %s", plan.Status)
	}
	if len(plan.ClarifyingQuestions) == 0 {
		t.Fatal("expected clarifying question")
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
