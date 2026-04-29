package sqlite

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWBRegistryStoreRawSearchOperationsAppliesPolicyFilters(t *testing.T) {
	store := newTestWBRegistryStore(t)

	readonly := true
	write := false

	operations := []entities.WBRegistryOperation{
		testRawSearchOperation("read_operation", readonlyPtr(readonly), false, "sales", "/api/v1/supplier/sales"),
		testRawSearchOperation("write_operation", readonlyPtr(write), false, "sales", "/api/v1/write/sales"),
		testRawSearchOperation("jam_operation", readonlyPtr(readonly), true, "sales", "/api/v1/jam/sales"),
	}

	if err := store.ReplaceAll(context.Background(), operations); err != nil {
		t.Fatalf("replace operations: %v", err)
	}

	found, err := store.RawSearchOperations(context.Background(), wbregistry.SearchQuery{
		Query:        "sales",
		Limit:        10,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("raw search operations: %v", err)
	}

	assertRawSearchOperationIDs(t, found, []string{"read_operation"})
}

func TestWBRegistryStoreRawSearchOperationsUsesStableStorageOrderingWithoutRanking(t *testing.T) {
	store := newTestWBRegistryStore(t)

	readonly := true

	operations := []entities.WBRegistryOperation{
		testRawSearchOperation("z_operation", readonlyPtr(readonly), false, "sales", "/api/v1/supplier/sales"),
		testRawSearchOperation("a_operation", readonlyPtr(readonly), false, "sales", "/api/v3/stocks/{warehouseId}"),
		testRawSearchOperation("m_operation", readonlyPtr(readonly), false, "sales", "/api/v1/supplier/orders"),
	}

	if err := store.ReplaceAll(context.Background(), operations); err != nil {
		t.Fatalf("replace operations: %v", err)
	}

	found, err := store.RawSearchOperations(context.Background(), wbregistry.SearchQuery{
		Query:        "sales",
		Limit:        10,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("raw search operations: %v", err)
	}

	assertRawSearchOperationIDs(t, found, []string{
		"a_operation",
		"m_operation",
		"z_operation",
	})
}

func TestWBRegistryStoreRawSearchOperationsRespectsLimit(t *testing.T) {
	store := newTestWBRegistryStore(t)

	readonly := true

	operations := []entities.WBRegistryOperation{
		testRawSearchOperation("a_operation", readonlyPtr(readonly), false, "sales", "/api/v1/a"),
		testRawSearchOperation("b_operation", readonlyPtr(readonly), false, "sales", "/api/v1/b"),
		testRawSearchOperation("c_operation", readonlyPtr(readonly), false, "sales", "/api/v1/c"),
	}

	if err := store.ReplaceAll(context.Background(), operations); err != nil {
		t.Fatalf("replace operations: %v", err)
	}

	found, err := store.RawSearchOperations(context.Background(), wbregistry.SearchQuery{
		Query:        "sales",
		Limit:        2,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("raw search operations: %v", err)
	}

	assertRawSearchOperationIDs(t, found, []string{
		"a_operation",
		"b_operation",
	})
}

func newTestWBRegistryStore(t *testing.T) *WBRegistryStore {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&wbRegistryOperationRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return NewWBRegistryStore(db)
}

func testRawSearchOperation(
	operationID string,
	readonly *bool,
	requiresJam bool,
	summary string,
	pathTemplate string,
) entities.WBRegistryOperation {
	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "test.yaml",
		OperationID:              operationID,
		Method:                   "GET",
		ServerURL:                "https://example.test",
		PathTemplate:             pathTemplate,
		Tags:                     []string{"test"},
		Category:                 "test",
		Summary:                  summary,
		Description:              summary,
		XReadonlyMethod:          readonly,
		XCategory:                "test",
		XTokenTypes:              []string{},
		PathParamsSchemaJSON:     "{}",
		QueryParamsSchemaJSON:    "{}",
		HeadersSchemaJSON:        "{}",
		RequestBodySchemaJSON:    "{}",
		ResponseSchemaJSON:       "{}",
		RateLimitNotes:           "",
		SubscriptionRequirements: "",
		RequiresJam:              requiresJam,
	}
}

func readonlyPtr(value bool) *bool {
	return &value
}

func assertRawSearchOperationIDs(
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
