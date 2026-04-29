package wb_registry_retrieval

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Ranks raw registry candidates deterministically without changing source-of-record storage data.
func rankOperations(
	operations []entities.WBRegistryOperation,
	query string,
	limit int,
) []entities.WBRegistryOperation {
	if limit <= 0 {
		limit = 20
	}

	tokens := searchTokens(query)

	ranked := append([]entities.WBRegistryOperation{}, operations...)
	sort.SliceStable(ranked, func(i, j int) bool {
		leftScore := operationScore(ranked[i], tokens)
		rightScore := operationScore(ranked[j], tokens)

		if leftScore == rightScore {
			return ranked[i].OperationID < ranked[j].OperationID
		}

		return leftScore > rightScore
	})

	// WHY: Existing multi-intent retrieval behavior is preserved while moving ownership out of SQLite.
	ranked = selectMultiIntentOperations(ranked, tokens, limit)

	return ranked
}

func searchTokens(query string) []string {
	stopWords := map[string]bool{
		"get": true, "and": true, "the": true, "api": true,
		"получить": true, "по": true, "и": true, "за": true, "на": true, "в": true,
		"каждому": true, "каждый": true, "последний": true, "последние": true,
		"месяц": true, "товару": true, "товарам": true,
		"warehouse": true, "id": true, "warehouse_id": true,
	}

	rawFields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("_,.;:!?()[]{}\"'`/\\|-+", r)
	})

	seen := make(map[string]bool)
	tokens := make([]string, 0, len(rawFields))

	for _, field := range rawFields {
		field = strings.TrimSpace(field)
		if field == "" || stopWords[field] || seen[field] {
			continue
		}

		if _, err := strconv.Atoi(field); err == nil {
			continue
		}

		addSearchToken(&tokens, seen, field)

		for _, alias := range tokenAliases(field) {
			addSearchToken(&tokens, seen, alias)
		}
	}

	return tokens
}

func addSearchToken(tokens *[]string, seen map[string]bool, token string) {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" || seen[token] {
		return
	}

	seen[token] = true
	*tokens = append(*tokens, token)
}

func tokenAliases(token string) []string {
	switch token {
	case "inventory", "остатки", "остаток", "остатков", "остаткам":
		return []string{"остат", "stock", "stocks"}
	case "sales", "sale", "продажи", "продаж", "продажам", "продажах":
		return []string{"продаж", "sale", "sales"}
	case "склад", "склада", "складе", "склады", "складах":
		return []string{"склад", "warehouse", "warehouses"}
	case "orders", "order", "заказы", "заказов", "заказ":
		return []string{"заказ", "order", "orders"}
	default:
		return nil
	}
}

func operationScore(operation entities.WBRegistryOperation, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}

	score := 0

	score += weightedFieldScore(operation.OperationID, tokens, 20)
	score += weightedFieldScore(operation.Summary, tokens, 18)
	score += weightedFieldScore(strings.Join(operation.Tags, " "), tokens, 16)
	score += weightedFieldScore(operation.Category, tokens, 14)
	score += weightedFieldScore(operation.PathTemplate, tokens, 12)
	score += weightedFieldScore(operation.XCategory, tokens, 8)
	score += weightedFieldScore(operation.SourceFile, tokens, 6)
	score += weightedFieldScore(operation.Description, tokens, 3)

	score += businessRelevanceBoost(operation, tokens)

	return score
}

func weightedFieldScore(value string, tokens []string, weight int) int {
	normalized := strings.ToLower(value)
	score := 0

	for _, token := range tokens {
		if strings.Contains(normalized, token) {
			score += weight
		}
	}

	return score
}

func businessRelevanceBoost(operation entities.WBRegistryOperation, tokens []string) int {
	combined := strings.ToLower(
		operation.OperationID + " " +
			operation.SourceFile + " " +
			operation.PathTemplate + " " +
			strings.Join(operation.Tags, " ") + " " +
			operation.Category + " " +
			operation.Summary + " " +
			operation.Description + " " +
			operation.XCategory,
	)

	hasStockIntent := containsAny(tokens, "остат", "stock", "stocks", "inventory")
	hasSalesIntent := containsAny(tokens, "продаж", "sale", "sales")
	hasWarehouseIntent := containsAny(tokens, "склад", "warehouse", "warehouses")

	score := 0

	if hasStockIntent && strings.Contains(combined, "остат") {
		score += 30
	}
	if hasStockIntent && strings.Contains(combined, "stock") {
		score += 20
	}
	if hasSalesIntent && strings.Contains(combined, "продаж") {
		score += 30
	}
	if hasSalesIntent && strings.Contains(combined, "sales") {
		score += 20
	}
	if hasWarehouseIntent && strings.Contains(combined, "склад") {
		score += 15
	}
	if hasWarehouseIntent && strings.Contains(combined, "warehouse") {
		score += 10
	}

	if hasSalesIntent && strings.Contains(operation.PathTemplate, "/api/v1/supplier/sales") {
		score += 80
	}
	if hasStockIntent && strings.Contains(operation.PathTemplate, "/api/v3/stocks/") {
		score += 70
	}
	if hasStockIntent && strings.Contains(operation.PathTemplate, "/stocks-report/") {
		score += 60
	}

	if strings.Contains(operation.PathTemplate, "/tariffs/") {
		score -= 60
	}
	if strings.Contains(operation.PathTemplate, "/brand-share/") {
		score -= 50
	}
	if strings.Contains(operation.PathTemplate, "/click-collect/") {
		score -= 40
	}
	if strings.Contains(operation.PathTemplate, "/dbs/orders") && !containsAny(tokens, "dbs") {
		score -= 30
	}

	return score
}

func containsAny(values []string, candidates ...string) bool {
	for _, value := range values {
		for _, candidate := range candidates {
			if value == candidate {
				return true
			}
		}
	}

	return false
}

func selectMultiIntentOperations(
	operations []entities.WBRegistryOperation,
	tokens []string,
	limit int,
) []entities.WBRegistryOperation {
	if limit <= 0 || len(operations) <= limit {
		return operations
	}

	clusters := activeIntentClusters(tokens)
	if len(clusters) == 0 {
		return operations[:limit]
	}

	selected := make([]entities.WBRegistryOperation, 0, limit)
	seen := make(map[string]bool)

	for _, cluster := range clusters {
		operation, ok := bestOperationForIntentCluster(operations, tokens, cluster, seen)
		if !ok {
			continue
		}

		selected = append(selected, operation)
		seen[operation.OperationID] = true

		if len(selected) == limit {
			return selected
		}
	}

	for _, operation := range operations {
		if seen[operation.OperationID] {
			continue
		}

		selected = append(selected, operation)
		seen[operation.OperationID] = true

		if len(selected) == limit {
			return selected
		}
	}

	return selected
}

func activeIntentClusters(tokens []string) []string {
	clusters := make([]string, 0, 4)

	hasStocks := containsAny(tokens, "остат", "stock", "stocks", "inventory")
	hasSales := containsAny(tokens, "продаж", "sale", "sales")
	hasOrders := containsAny(tokens, "заказ", "order", "orders")
	hasWarehouseList := containsAny(tokens, "список", "list") &&
		containsAny(tokens, "склад", "warehouse", "warehouses")

	if hasStocks {
		clusters = append(clusters, "stocks")
	}

	if hasSales {
		clusters = append(clusters, "sales")
	}

	if hasOrders {
		clusters = append(clusters, "orders")
	}

	if hasWarehouseList && !hasStocks && !hasSales {
		clusters = append(clusters, "warehouses")
	}

	return clusters
}

func bestOperationForIntentCluster(
	operations []entities.WBRegistryOperation,
	tokens []string,
	cluster string,
	seen map[string]bool,
) (entities.WBRegistryOperation, bool) {
	bestIndex := -1
	bestScore := -1

	for index, operation := range operations {
		if seen[operation.OperationID] {
			continue
		}

		if !operationMatchesIntentCluster(operation, cluster) {
			continue
		}

		score := operationScore(operation, tokens) + intentClusterScore(operation, cluster)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}

	if bestIndex == -1 {
		return entities.WBRegistryOperation{}, false
	}

	return operations[bestIndex], true
}

func operationMatchesIntentCluster(operation entities.WBRegistryOperation, cluster string) bool {
	combined := normalizedOperationText(operation)

	switch cluster {
	case "stocks":
		return strings.Contains(combined, "остат") ||
			strings.Contains(combined, "stock") ||
			strings.Contains(combined, "warehouse_remains")
	case "sales":
		return strings.Contains(combined, "/supplier/sales") ||
			strings.Contains(combined, "продаж") ||
			strings.Contains(combined, "sales")
	case "orders":
		return strings.Contains(combined, "/supplier/orders") ||
			strings.Contains(combined, "/orders") ||
			strings.Contains(combined, "заказ") ||
			strings.Contains(combined, "order")
	case "warehouses":
		return strings.Contains(combined, "/warehouses") ||
			strings.Contains(combined, "склады продавца") ||
			strings.Contains(combined, "warehouse")
	default:
		return false
	}
}

func intentClusterScore(operation entities.WBRegistryOperation, cluster string) int {
	combined := normalizedOperationText(operation)

	switch cluster {
	case "stocks":
		score := 0

		if strings.Contains(operation.PathTemplate, "/api/v3/stocks/") {
			score += 300
		}
		if strings.Contains(operation.PathTemplate, "/stocks-report/") {
			score += 220
		}
		if strings.Contains(operation.PathTemplate, "/warehouse_remains") {
			score += 120
		}
		if strings.Contains(combined, "остатки на складах продавца") {
			score += 120
		}
		if strings.Contains(operation.PathTemplate, "/tariffs/") {
			score -= 200
		}

		return score

	case "sales":
		score := 0

		if strings.Contains(operation.PathTemplate, "/api/v1/supplier/sales") {
			score += 350
		}
		if strings.Contains(operation.Summary, "Продажи") {
			score += 120
		}
		if strings.Contains(operation.PathTemplate, "/brand-share/") {
			score -= 250
		}
		if strings.Contains(operation.PathTemplate, "/advert") {
			score -= 150
		}

		return score

	case "orders":
		score := 0

		if strings.Contains(operation.PathTemplate, "/api/v1/supplier/orders") {
			score += 300
		}
		if strings.Contains(operation.PathTemplate, "/api/v3/orders") {
			score += 220
		}
		if strings.Contains(operation.PathTemplate, "/dbs/orders") {
			score += 160
		}

		return score

	case "warehouses":
		score := 0

		if strings.Contains(operation.PathTemplate, "/api/v3/warehouses") {
			score += 300
		}
		if strings.Contains(combined, "список складов продавца") {
			score += 160
		}

		return score

	default:
		return 0
	}
}

func normalizedOperationText(operation entities.WBRegistryOperation) string {
	return strings.ToLower(
		operation.OperationID + " " +
			operation.SourceFile + " " +
			operation.Method + " " +
			operation.PathTemplate + " " +
			strings.Join(operation.Tags, " ") + " " +
			operation.Category + " " +
			operation.Summary + " " +
			operation.Description + " " +
			operation.XCategory,
	)
}
