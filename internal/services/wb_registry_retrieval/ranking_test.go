package wb_registry_retrieval

import (
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func TestSearchTokensExpandsBusinessAliases(t *testing.T) {
	tokens := searchTokens("получить остатки и продажи")

	assertContainsToken(t, tokens, "остатки")
	assertContainsToken(t, tokens, "остат")
	assertContainsToken(t, tokens, "stock")
	assertContainsToken(t, tokens, "stocks")
	assertContainsToken(t, tokens, "продажи")
	assertContainsToken(t, tokens, "продаж")
	assertContainsToken(t, tokens, "sale")
	assertContainsToken(t, tokens, "sales")
}

func TestRankOperationsPreservesMultiIntentCoverage(t *testing.T) {
	operations := []entities.WBRegistryOperation{
		retrievalRankingOperation(
			"generated_get_api_v1_supplier_tariffs",
			"/api/v1/supplier/tariffs",
			"Тарифы",
			"Тарифы комиссий",
			[]string{"Тарифы"},
		),
		retrievalRankingOperation(
			"generated_get_api_v1_supplier_sales",
			"/api/v1/supplier/sales",
			"Продажи",
			"Метод возвращает информацию о продажах и возвратах.",
			[]string{"Основные отчёты"},
		),
		retrievalRankingOperation(
			"generated_post_api_v3_stocks_warehouseid",
			"/api/v3/stocks/{warehouseId}",
			"Получить остатки товаров",
			"Метод возвращает данные об остатках товаров на складах продавца.",
			[]string{"Остатки на складах продавца"},
		),
	}

	ranked := rankOperations(operations, "получить остатки и продажи", 2)

	if len(ranked) != 2 {
		t.Fatalf("expected 2 operations, got %#v", ranked)
	}

	assertOperationIDs(t, ranked, []string{
		"generated_post_api_v3_stocks_warehouseid",
		"generated_get_api_v1_supplier_sales",
	})
}

func TestActiveIntentClustersDetectsStocksAndSales(t *testing.T) {
	tokens := searchTokens("остатки продажи")

	clusters := activeIntentClusters(tokens)

	assertContainsToken(t, clusters, "stocks")
	assertContainsToken(t, clusters, "sales")
}

func retrievalRankingOperation(
	operationID string,
	pathTemplate string,
	summary string,
	description string,
	tags []string,
) entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "test.yaml",
		OperationID:              operationID,
		Method:                   "GET",
		ServerURL:                "https://example.test",
		PathTemplate:             pathTemplate,
		Tags:                     tags,
		Category:                 "",
		Summary:                  summary,
		Description:              description,
		XReadonlyMethod:          &readonly,
		XCategory:                "",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        "{}",
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              false,
	}
}

func assertContainsToken(t *testing.T, values []string, expected string) {
	t.Helper()

	for _, value := range values {
		if value == expected {
			return
		}
	}

	t.Fatalf("expected %#v to contain %q", values, expected)
}

func assertOperationIDs(
	t *testing.T,
	operations []entities.WBRegistryOperation,
	expected []string,
) {
	t.Helper()

	if len(operations) != len(expected) {
		t.Fatalf("expected %d operations, got %#v", len(expected), operations)
	}

	for index, operation := range operations {
		if operation.OperationID != expected[index] {
			t.Fatalf("expected operation[%d]=%q, got %q", index, expected[index], operation.OperationID)
		}
	}
}
