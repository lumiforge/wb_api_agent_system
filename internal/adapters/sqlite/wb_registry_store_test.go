package sqlite

import "testing"

func TestSelectMultiIntentRecordsPromotesStocksAndSales(t *testing.T) {
	tokens := searchTokens("Получить остатки на складе 507 и по каждому товару продажи за последний месяц")

	records := []wbRegistryOperationRecord{
		{
			OperationID:  "generated_post_api_v3_stocks_warehouseid",
			SourceFile:   "products.yaml",
			Method:       "POST",
			PathTemplate: "/api/v3/stocks/{warehouseId}",
			TagsJSON:     `["Остатки на складах продавца"]`,
			Category:     "Остатки на складах продавца",
			Summary:      "Получить остатки товаров",
			Description:  "Метод возвращает данные об остатках товаров на складах продавца.",
			XCategory:    "marketplace",
		},
		{
			OperationID:  "postV1StocksReportWbWarehouses",
			SourceFile:   "analytics.yaml",
			Method:       "POST",
			PathTemplate: "/api/analytics/v1/stocks-report/wb-warehouses",
			TagsJSON:     `["История остатков"]`,
			Category:     "История остатков",
			Summary:      "Остатки на складах WB",
			Description:  "Метод возвращает текущие остатки товаров на складах WB.",
			XCategory:    "contentanalytics",
		},
		{
			OperationID:  "generated_get_api_v1_supplier_sales",
			SourceFile:   "reports.yaml",
			Method:       "GET",
			PathTemplate: "/api/v1/supplier/sales",
			TagsJSON:     `["Основные отчёты"]`,
			Category:     "Основные отчёты",
			Summary:      "Продажи",
			Description:  "Метод возвращает информацию о продажах и возвратах.",
			XCategory:    "statistics",
		},
		{
			OperationID:  "generated_get_api_v1_tariffs_box",
			SourceFile:   "tariffs.yaml",
			Method:       "GET",
			PathTemplate: "/api/v1/tariffs/box",
			TagsJSON:     `["Тарифы на остаток"]`,
			Category:     "Тарифы на остаток",
			Summary:      "Тарифы для коробов",
			Description:  "Для остатков товаров метод возвращает тарифы.",
			XCategory:    "commonapi",
		},
	}

	selected := selectMultiIntentRecords(records, tokens, 3)

	if len(selected) != 3 {
		t.Fatalf("expected 3 records, got %d", len(selected))
	}

	if selected[0].OperationID != "generated_post_api_v3_stocks_warehouseid" {
		t.Fatalf("expected stocks operation first, got %s", selected[0].OperationID)
	}

	if selected[1].OperationID != "generated_get_api_v1_supplier_sales" {
		t.Fatalf("expected sales operation second, got %s", selected[1].OperationID)
	}
}

func TestActiveIntentClusters(t *testing.T) {
	tokens := searchTokens("остатки продажи заказы")

	clusters := activeIntentClusters(tokens)

	if !containsString(clusters, "stocks") {
		t.Fatal("expected stocks cluster")
	}
	if !containsString(clusters, "sales") {
		t.Fatal("expected sales cluster")
	}
	if !containsString(clusters, "orders") {
		t.Fatal("expected orders cluster")
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}

	return false
}
