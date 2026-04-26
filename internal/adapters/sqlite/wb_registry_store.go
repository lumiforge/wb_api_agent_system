package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
	"gorm.io/gorm"
)

// PURPOSE: Persists indexed Wildberries OpenAPI operations in SQLite.
type WBRegistryStore struct {
	db *gorm.DB
}

type wbRegistryOperationRecord struct {
	ID                       int64  `gorm:"primaryKey;autoIncrement"`
	Marketplace              string `gorm:"not null"`
	SourceFile               string `gorm:"not null"`
	OperationID              string `gorm:"not null"`
	Method                   string `gorm:"not null"`
	ServerURL                string `gorm:"not null"`
	PathTemplate             string `gorm:"not null"`
	TagsJSON                 string `gorm:"not null"`
	Category                 string `gorm:"not null"`
	Summary                  string `gorm:"not null"`
	Description              string `gorm:"not null"`
	XReadonlyMethod          *bool
	XCategory                string `gorm:"not null"`
	XTokenTypesJSON          string `gorm:"not null"`
	PathParamsSchemaJSON     string `gorm:"not null"`
	QueryParamsSchemaJSON    string `gorm:"not null"`
	HeadersSchemaJSON        string `gorm:"not null"`
	RequestBodySchemaJSON    string `gorm:"not null"`
	ResponseSchemaJSON       string `gorm:"not null"`
	RateLimitNotes           string `gorm:"not null"`
	SubscriptionRequirements string `gorm:"not null"`
	MaxPeriodDays            *int
	MaxLookbackDays          *int
	RequiresJam              bool `gorm:"not null"`
}

func (wbRegistryOperationRecord) TableName() string {
	return "wb_registry_operations"
}

func NewWBRegistryStore(db *gorm.DB) *WBRegistryStore {
	return &WBRegistryStore{db: db}
}

func (s *WBRegistryStore) ReplaceAll(ctx context.Context, operations []entities.WBRegistryOperation) error {
	records := make([]wbRegistryOperationRecord, 0, len(operations))

	for _, operation := range operations {
		records = append(records, wbRegistryOperationRecord{
			Marketplace:              operation.Marketplace,
			SourceFile:               operation.SourceFile,
			OperationID:              operation.OperationID,
			Method:                   operation.Method,
			ServerURL:                operation.ServerURL,
			PathTemplate:             operation.PathTemplate,
			TagsJSON:                 mustJSONArray(operation.Tags),
			Category:                 operation.Category,
			Summary:                  operation.Summary,
			Description:              operation.Description,
			XReadonlyMethod:          operation.XReadonlyMethod,
			XCategory:                operation.XCategory,
			XTokenTypesJSON:          mustJSONArray(operation.XTokenTypes),
			PathParamsSchemaJSON:     operation.PathParamsSchemaJSON,
			QueryParamsSchemaJSON:    operation.QueryParamsSchemaJSON,
			HeadersSchemaJSON:        operation.HeadersSchemaJSON,
			RequestBodySchemaJSON:    operation.RequestBodySchemaJSON,
			ResponseSchemaJSON:       operation.ResponseSchemaJSON,
			RateLimitNotes:           operation.RateLimitNotes,
			SubscriptionRequirements: operation.SubscriptionRequirements,
			MaxPeriodDays:            operation.MaxPeriodDays,
			MaxLookbackDays:          operation.MaxLookbackDays,
			RequiresJam:              operation.RequiresJam,
		})
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// WHY: Replacing avoids stale rows from older parser versions, including rows with empty operation_id.
		if err := tx.Exec("DELETE FROM wb_registry_operations").Error; err != nil {
			return err
		}

		if len(records) == 0 {
			return nil
		}

		if err := tx.CreateInBatches(records, 100).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("replace wb registry operations: %w", err)
	}

	return nil
}

func (s *WBRegistryStore) SearchOperations(ctx context.Context, searchQuery wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	limit := searchQuery.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	tokens := searchTokens(searchQuery.Query)

	db := s.db.WithContext(ctx)

	if searchQuery.ReadonlyOnly {
		db = db.Where("x_readonly_method = ?", true)
	}

	if searchQuery.ExcludeJam {
		db = db.Where("requires_jam = ?", false)
	}

	if len(tokens) > 0 {
		conditions := make([]string, 0, len(tokens))
		args := make([]any, 0, len(tokens))

		for _, token := range tokens {
			conditions = append(
				conditions,
				`LOWER(operation_id || ' ' || source_file || ' ' || method || ' ' || path_template || ' ' || tags_json || ' ' || category || ' ' || summary || ' ' || description || ' ' || x_category) LIKE ?`,
			)
			args = append(args, "%"+token+"%")
		}

		// WHY: Registry retrieval should keep recall broad, then rank in Go so LLM fallback receives the most relevant operations first.
		db = db.Where(strings.Join(conditions, " OR "), args...)
	}

	preLimit := limit * 8
	if preLimit < 80 {
		preLimit = 80
	}
	if preLimit > 300 {
		preLimit = 300
	}

	var records []wbRegistryOperationRecord
	if err := db.
		Order("source_file ASC, operation_id ASC").
		Limit(preLimit).
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf("search wb registry operations: %w", err)
	}

	sort.SliceStable(records, func(i, j int) bool {
		leftScore := operationScore(records[i], tokens)
		rightScore := operationScore(records[j], tokens)

		if leftScore == rightScore {
			return records[i].OperationID < records[j].OperationID
		}

		return leftScore > rightScore
	})

	// WHY: Compound business requests need at least one strong candidate per detected intent, not only global top-N.
	records = selectMultiIntentRecords(records, tokens, limit)

	operations := make([]entities.WBRegistryOperation, 0, len(records))
	for _, record := range records {
		operations = append(operations, record.toEntity())
	}

	return operations, nil
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

func operationScore(record wbRegistryOperationRecord, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}

	score := 0

	score += weightedFieldScore(record.OperationID, tokens, 20)
	score += weightedFieldScore(record.Summary, tokens, 18)
	score += weightedFieldScore(record.TagsJSON, tokens, 16)
	score += weightedFieldScore(record.Category, tokens, 14)
	score += weightedFieldScore(record.PathTemplate, tokens, 12)
	score += weightedFieldScore(record.XCategory, tokens, 8)
	score += weightedFieldScore(record.SourceFile, tokens, 6)
	score += weightedFieldScore(record.Description, tokens, 3)

	score += businessRelevanceBoost(record, tokens)

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

func businessRelevanceBoost(record wbRegistryOperationRecord, tokens []string) int {
	combined := strings.ToLower(
		record.OperationID + " " +
			record.SourceFile + " " +
			record.PathTemplate + " " +
			record.TagsJSON + " " +
			record.Category + " " +
			record.Summary + " " +
			record.Description + " " +
			record.XCategory,
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

	if hasSalesIntent && strings.Contains(record.PathTemplate, "/api/v1/supplier/sales") {
		score += 80
	}
	if hasStockIntent && strings.Contains(record.PathTemplate, "/api/v3/stocks/") {
		score += 70
	}
	if hasStockIntent && strings.Contains(record.PathTemplate, "/stocks-report/") {
		score += 60
	}

	if strings.Contains(record.PathTemplate, "/tariffs/") {
		score -= 60
	}
	if strings.Contains(record.PathTemplate, "/brand-share/") {
		score -= 50
	}
	if strings.Contains(record.PathTemplate, "/click-collect/") {
		score -= 40
	}
	if strings.Contains(record.PathTemplate, "/dbs/orders") && !containsAny(tokens, "dbs") {
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

func (s *WBRegistryStore) GetOperation(ctx context.Context, operationID string) (*entities.WBRegistryOperation, error) {
	var record wbRegistryOperationRecord

	err := s.db.WithContext(ctx).
		Where("operation_id = ?", operationID).
		Limit(1).
		Find(&record).
		Error
	if err != nil {
		return nil, fmt.Errorf("get wb registry operation: %w", err)
	}

	if record.OperationID == "" {
		return nil, nil
	}

	operation := record.toEntity()

	return &operation, nil
}

func (s *WBRegistryStore) Stats(ctx context.Context) (wbregistry.Stats, error) {
	var stats wbregistry.Stats

	if err := countInto(ctx, s.db, &stats.Total, "1 = 1"); err != nil {
		return wbregistry.Stats{}, err
	}

	if err := countInto(ctx, s.db, &stats.Read, "x_readonly_method = 1"); err != nil {
		return wbregistry.Stats{}, err
	}

	if err := countInto(ctx, s.db, &stats.Write, "x_readonly_method = 0"); err != nil {
		return wbregistry.Stats{}, err
	}

	if err := countInto(ctx, s.db, &stats.UnknownReadonly, "x_readonly_method IS NULL"); err != nil {
		return wbregistry.Stats{}, err
	}

	if err := countInto(ctx, s.db, &stats.JamOnly, "requires_jam = 1"); err != nil {
		return wbregistry.Stats{}, err
	}

	if err := countInto(ctx, s.db, &stats.GeneratedOperationID, "operation_id LIKE 'generated_%'"); err != nil {
		return wbregistry.Stats{}, err
	}

	return stats, nil
}

func countInto(ctx context.Context, db *gorm.DB, target *int64, condition string) error {
	if err := db.WithContext(ctx).
		Model(&wbRegistryOperationRecord{}).
		Where(condition).
		Count(target).
		Error; err != nil {
		return fmt.Errorf("count wb registry operations: %w", err)
	}

	return nil
}

func (r wbRegistryOperationRecord) toEntity() entities.WBRegistryOperation {
	return entities.WBRegistryOperation{
		Marketplace:              r.Marketplace,
		SourceFile:               r.SourceFile,
		OperationID:              r.OperationID,
		Method:                   r.Method,
		ServerURL:                r.ServerURL,
		PathTemplate:             r.PathTemplate,
		Tags:                     mustStringSlice(r.TagsJSON),
		Category:                 r.Category,
		Summary:                  r.Summary,
		Description:              r.Description,
		XReadonlyMethod:          r.XReadonlyMethod,
		XCategory:                r.XCategory,
		XTokenTypes:              mustStringSlice(r.XTokenTypesJSON),
		PathParamsSchemaJSON:     r.PathParamsSchemaJSON,
		QueryParamsSchemaJSON:    r.QueryParamsSchemaJSON,
		HeadersSchemaJSON:        r.HeadersSchemaJSON,
		RequestBodySchemaJSON:    r.RequestBodySchemaJSON,
		ResponseSchemaJSON:       r.ResponseSchemaJSON,
		RateLimitNotes:           r.RateLimitNotes,
		SubscriptionRequirements: r.SubscriptionRequirements,
		MaxPeriodDays:            r.MaxPeriodDays,
		MaxLookbackDays:          r.MaxLookbackDays,
		RequiresJam:              r.RequiresJam,
	}
}

func mustStringSlice(value string) []string {
	if value == "" {
		return []string{}
	}

	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return []string{}
	}

	if result == nil {
		return []string{}
	}

	return result
}

func selectMultiIntentRecords(
	records []wbRegistryOperationRecord,
	tokens []string,
	limit int,
) []wbRegistryOperationRecord {
	if limit <= 0 || len(records) <= limit {
		return records
	}

	clusters := activeIntentClusters(tokens)
	if len(clusters) == 0 {
		return records[:limit]
	}

	selected := make([]wbRegistryOperationRecord, 0, limit)
	seen := make(map[string]bool)

	for _, cluster := range clusters {
		record, ok := bestRecordForIntentCluster(records, tokens, cluster, seen)
		if !ok {
			continue
		}

		selected = append(selected, record)
		seen[record.OperationID] = true

		if len(selected) == limit {
			return selected
		}
	}

	for _, record := range records {
		if seen[record.OperationID] {
			continue
		}

		selected = append(selected, record)
		seen[record.OperationID] = true

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

func bestRecordForIntentCluster(
	records []wbRegistryOperationRecord,
	tokens []string,
	cluster string,
	seen map[string]bool,
) (wbRegistryOperationRecord, bool) {
	bestIndex := -1
	bestScore := -1

	for index, record := range records {
		if seen[record.OperationID] {
			continue
		}

		if !recordMatchesIntentCluster(record, cluster) {
			continue
		}

		score := operationScore(record, tokens) + intentClusterScore(record, cluster)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}

	if bestIndex == -1 {
		return wbRegistryOperationRecord{}, false
	}

	return records[bestIndex], true
}

func recordMatchesIntentCluster(record wbRegistryOperationRecord, cluster string) bool {
	combined := normalizedOperationText(record)

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

func intentClusterScore(record wbRegistryOperationRecord, cluster string) int {
	combined := normalizedOperationText(record)

	switch cluster {
	case "stocks":
		score := 0

		if strings.Contains(record.PathTemplate, "/api/v3/stocks/") {
			score += 300
		}
		if strings.Contains(record.PathTemplate, "/stocks-report/") {
			score += 220
		}
		if strings.Contains(record.PathTemplate, "/warehouse_remains") {
			score += 120
		}
		if strings.Contains(combined, "остатки на складах продавца") {
			score += 120
		}
		if strings.Contains(record.PathTemplate, "/tariffs/") {
			score -= 200
		}

		return score

	case "sales":
		score := 0

		if strings.Contains(record.PathTemplate, "/api/v1/supplier/sales") {
			score += 350
		}
		if strings.Contains(record.Summary, "Продажи") {
			score += 120
		}
		if strings.Contains(record.PathTemplate, "/brand-share/") {
			score -= 250
		}
		if strings.Contains(record.PathTemplate, "/advert") {
			score -= 150
		}

		return score

	case "orders":
		score := 0

		if strings.Contains(record.PathTemplate, "/api/v1/supplier/orders") {
			score += 300
		}
		if strings.Contains(record.PathTemplate, "/api/v3/orders") {
			score += 220
		}
		if strings.Contains(record.PathTemplate, "/dbs/orders") {
			score += 160
		}

		return score

	case "warehouses":
		score := 0

		if strings.Contains(record.PathTemplate, "/api/v3/warehouses") {
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

func normalizedOperationText(record wbRegistryOperationRecord) string {
	return strings.ToLower(
		record.OperationID + " " +
			record.SourceFile + " " +
			record.Method + " " +
			record.PathTemplate + " " +
			record.TagsJSON + " " +
			record.Category + " " +
			record.Summary + " " +
			record.Description + " " +
			record.XCategory,
	)
}
