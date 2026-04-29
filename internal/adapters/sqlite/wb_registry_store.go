package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
	"gorm.io/gorm"
)

// PURPOSE: Persists indexed Wildberries OpenAPI operations as the registry source-of-record.
type WBRegistryStore struct {
	db *gorm.DB
}

var _ wbregistry.RawOperationStore = (*WBRegistryStore)(nil)

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

// PURPOSE: Returns storage-owned registry records without applying retrieval ranking or business-intent clustering.
func (s *WBRegistryStore) RawSearchOperations(ctx context.Context, searchQuery wbregistry.SearchQuery) ([]entities.WBRegistryOperation, error) {
	limit := normalizedRawSearchLimit(searchQuery.Limit)

	records, err := s.searchOperationRecords(ctx, searchQuery, limit)
	if err != nil {
		return nil, err
	}

	return registryRecordsToEntities(records), nil
}

func (s *WBRegistryStore) ListOperations(ctx context.Context) ([]entities.WBRegistryOperation, error) {
	var records []wbRegistryOperationRecord

	if err := s.db.WithContext(ctx).
		Order("source_file ASC, operation_id ASC").
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf("list wb registry operations: %w", err)
	}

	return registryRecordsToEntities(records), nil
}

func (s *WBRegistryStore) searchOperationRecords(
	ctx context.Context,
	searchQuery wbregistry.SearchQuery,
	limit int,
) ([]wbRegistryOperationRecord, error) {
	terms := storageSearchTerms(searchQuery.Query)

	db := s.db.WithContext(ctx)

	if searchQuery.ReadonlyOnly {
		db = db.Where("x_readonly_method = ?", true)
	}

	if searchQuery.ExcludeJam {
		db = db.Where("requires_jam = ?", false)
	}

	if len(terms) > 0 {
		conditions := make([]string, 0, len(terms))
		args := make([]any, 0, len(terms))

		for _, term := range terms {
			conditions = append(
				conditions,
				`LOWER(operation_id || ' ' || source_file || ' ' || method || ' ' || path_template || ' ' || tags_json || ' ' || category || ' ' || summary || ' ' || description || ' ' || x_category) LIKE ?`,
			)
			args = append(args, "%"+term+"%")
		}

		// WHY: Storage only applies broad lexical filtering over retrieval-provided terms; ranking lives above this layer.
		db = db.Where(strings.Join(conditions, " OR "), args...)
	}

	var records []wbRegistryOperationRecord
	if err := db.
		Order("source_file ASC, operation_id ASC").
		Limit(limit).
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf("search wb registry operations: %w", err)
	}

	return records, nil
}

func normalizedRawSearchLimit(limit int) int {
	if limit <= 0 {
		return 300
	}

	if limit > 500 {
		return 500
	}

	return limit
}

func storageSearchTerms(query string) []string {
	rawFields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("_,.;:!?()[]{}\"'`/\\|-+", r)
	})

	seen := make(map[string]bool)
	terms := make([]string, 0, len(rawFields))

	for _, field := range rawFields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}

		seen[field] = true
		terms = append(terms, field)
	}

	return terms
}

func registryRecordsToEntities(records []wbRegistryOperationRecord) []entities.WBRegistryOperation {
	operations := make([]entities.WBRegistryOperation, 0, len(records))
	for _, record := range records {
		operations = append(operations, record.toEntity())
	}

	return operations
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
