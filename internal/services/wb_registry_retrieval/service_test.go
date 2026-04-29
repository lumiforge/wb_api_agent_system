package wb_registry_retrieval

import (
	"context"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Protects retrieval ownership boundary: service ranks raw storage records instead of delegating ranked search.
func TestServiceSearchOperationsUsesRawSearchOperations(t *testing.T) {
	store := &fakeRawOperationStore{
		rawOperations: []entities.WBRegistryOperation{
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
		},
	}

	service, err := New(ServiceConfig{
		Store: store,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	operations, err := service.SearchOperations(context.Background(), wbregistry.SearchQuery{
		Query:        "получить остатки и продажи",
		Limit:        2,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store.rawSearchCalls != 1 {
		t.Fatalf("expected RawSearchOperations to be called once, got %d", store.rawSearchCalls)
	}

	if store.lastRawQuery.Limit != 80 {
		t.Fatalf("expected expanded raw search limit 80, got %d", store.lastRawQuery.Limit)
	}

	if store.lastRawQuery.Query != "остатки остат stock stocks продажи продаж sale sales" {
		t.Fatalf("unexpected raw query %q", store.lastRawQuery.Query)
	}

	assertOperationIDs(t, operations, []string{
		"generated_post_api_v3_stocks_warehouseid",
		"generated_get_api_v1_supplier_sales",
	})
}

func TestServiceGetOperationDelegatesToRawStore(t *testing.T) {
	expected := retrievalRankingOperation(
		"generated_get_api_v1_supplier_sales",
		"/api/v1/supplier/sales",
		"Продажи",
		"Метод возвращает информацию о продажах и возвратах.",
		[]string{"Основные отчёты"},
	)

	store := &fakeRawOperationStore{
		operationByID: map[string]entities.WBRegistryOperation{
			expected.OperationID: expected,
		},
	}

	service, err := New(ServiceConfig{
		Store: store,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	operation, err := service.GetOperation(context.Background(), expected.OperationID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if operation == nil {
		t.Fatal("expected operation, got nil")
	}

	if operation.OperationID != expected.OperationID {
		t.Fatalf("expected %q, got %q", expected.OperationID, operation.OperationID)
	}
}

func TestServiceStatsDelegatesToRawStore(t *testing.T) {
	store := &fakeRawOperationStore{
		stats: wbregistry.Stats{
			Total: 3,
			Read:  2,
			Write: 1,
		},
	}

	service, err := New(ServiceConfig{
		Store: store,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	stats, err := service.Stats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stats.Total != 3 || stats.Read != 2 || stats.Write != 1 {
		t.Fatalf("unexpected stats %#v", stats)
	}
}

type fakeRawOperationStore struct {
	rawOperations []entities.WBRegistryOperation
	operationByID map[string]entities.WBRegistryOperation
	stats         wbregistry.Stats

	rawSearchCalls int
	lastRawQuery   wbregistry.SearchQuery
}

func (s *fakeRawOperationStore) RawSearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	s.rawSearchCalls++
	s.lastRawQuery = query

	return s.rawOperations, nil
}

func (s *fakeRawOperationStore) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	operation, ok := s.operationByID[operationID]
	if !ok {
		return nil, nil
	}

	return &operation, nil
}

func (s *fakeRawOperationStore) Stats(ctx context.Context) (wbregistry.Stats, error) {
	return s.stats, nil
}

func (s *fakeRawOperationStore) ListOperations(
	ctx context.Context,
) ([]entities.WBRegistryOperation, error) {
	return s.rawOperations, nil
}

func TestServiceSearchOperationsMergesSemanticExpansionBeforeRanking(t *testing.T) {
	readonly := true

	store := &fakeRawOperationStore{
		rawOperations: []entities.WBRegistryOperation{
			retrievalRankingOperation(
				"generated_get_api_v1_supplier_sales",
				"/api/v1/supplier/sales",
				"Продажи",
				"Метод возвращает информацию о продажах и возвратах.",
				[]string{"Основные отчёты"},
			),
		},
		operationByID: map[string]entities.WBRegistryOperation{},
		stats:         wbregistry.Stats{},
	}

	semanticOperation := retrievalRankingOperation(
		"generated_post_api_v3_stocks_warehouseid",
		"/api/v3/stocks/{warehouseId}",
		"Получить остатки товаров",
		"Метод возвращает данные об остатках товаров на складах продавца.",
		[]string{"Остатки на складах продавца"},
	)
	semanticOperation.XReadonlyMethod = &readonly

	semanticRetriever := &fakeSemanticCandidateRetriever{
		results: []SemanticOperationResult{
			{
				Operation: semanticOperation,
				Score:     0.99,
			},
		},
	}

	service, err := New(ServiceConfig{
		Store:                    store,
		SemanticRetriever:        semanticRetriever,
		SemanticExpansionEnabled: true,
		SemanticExpansionLimit:   5,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	operations, err := service.SearchOperations(context.Background(), wbregistry.SearchQuery{
		Query:        "остатки и продажи",
		Limit:        2,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("search operations: %v", err)
	}

	if semanticRetriever.calls != 1 {
		t.Fatalf("expected semantic retriever to be called once, got %d", semanticRetriever.calls)
	}

	if semanticRetriever.lastQuery.Limit != 5 {
		t.Fatalf("expected semantic limit 5, got %d", semanticRetriever.lastQuery.Limit)
	}

	assertOperationIDs(t, operations, []string{
		"generated_post_api_v3_stocks_warehouseid",
		"generated_get_api_v1_supplier_sales",
	})
}

func TestServiceSearchOperationsDoesNotCallSemanticExpansionWhenDisabled(t *testing.T) {
	store := &fakeRawOperationStore{
		rawOperations: []entities.WBRegistryOperation{
			retrievalRankingOperation(
				"generated_get_api_v1_supplier_sales",
				"/api/v1/supplier/sales",
				"Продажи",
				"Метод возвращает информацию о продажах и возвратах.",
				[]string{"Основные отчёты"},
			),
		},
	}

	semanticRetriever := &fakeSemanticCandidateRetriever{}

	service, err := New(ServiceConfig{
		Store:                    store,
		SemanticRetriever:        semanticRetriever,
		SemanticExpansionEnabled: false,
		SemanticExpansionLimit:   5,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.SearchOperations(context.Background(), wbregistry.SearchQuery{
		Query: "продажи",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("search operations: %v", err)
	}

	if semanticRetriever.calls != 0 {
		t.Fatalf("expected semantic retriever not to be called, got %d", semanticRetriever.calls)
	}
}

func TestNewServiceRejectsMissingSemanticRetrieverWhenEnabled(t *testing.T) {
	_, err := New(ServiceConfig{
		Store:                    &fakeRawOperationStore{},
		SemanticExpansionEnabled: true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type fakeSemanticCandidateRetriever struct {
	results   []SemanticOperationResult
	calls     int
	lastQuery wbregistry.SearchQuery
}

func (r *fakeSemanticCandidateRetriever) Search(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]SemanticOperationResult, error) {
	r.calls++
	r.lastQuery = query

	return r.results, nil
}

func TestServiceSearchOperationsWithDiagnosticsReportsCandidateCounts(t *testing.T) {
	readonly := true

	store := &fakeRawOperationStore{
		rawOperations: []entities.WBRegistryOperation{
			retrievalRankingOperation(
				"generated_get_api_v1_supplier_sales",
				"/api/v1/supplier/sales",
				"Продажи",
				"Метод возвращает информацию о продажах и возвратах.",
				[]string{"Основные отчёты"},
			),
		},
	}

	semanticOperation := retrievalRankingOperation(
		"generated_post_api_v3_stocks_warehouseid",
		"/api/v3/stocks/{warehouseId}",
		"Получить остатки товаров",
		"Метод возвращает данные об остатках товаров на складах продавца.",
		[]string{"Остатки на складах продавца"},
	)
	semanticOperation.XReadonlyMethod = &readonly

	service, err := New(ServiceConfig{
		Store: store,
		SemanticRetriever: &fakeSemanticCandidateRetriever{
			results: []SemanticOperationResult{
				{
					Operation: semanticOperation,
					Score:     0.99,
				},
			},
		},
		SemanticExpansionEnabled: true,
		SemanticExpansionLimit:   5,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.SearchOperationsWithDiagnostics(context.Background(), wbregistry.SearchQuery{
		Query:        "остатки и продажи",
		Limit:        2,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("search operations with diagnostics: %v", err)
	}

	if result.Diagnostics.LexicalCandidates != 1 {
		t.Fatalf("expected 1 lexical candidate, got %d", result.Diagnostics.LexicalCandidates)
	}

	if result.Diagnostics.SemanticCandidates != 1 {
		t.Fatalf("expected 1 semantic candidate, got %d", result.Diagnostics.SemanticCandidates)
	}

	if result.Diagnostics.MergedCandidates != 2 {
		t.Fatalf("expected 2 merged candidates, got %d", result.Diagnostics.MergedCandidates)
	}

	if result.Diagnostics.ReturnedCandidates != 2 {
		t.Fatalf("expected 2 returned candidates, got %d", result.Diagnostics.ReturnedCandidates)
	}

	if !result.Diagnostics.SemanticExpansionEnabled {
		t.Fatal("expected semantic expansion enabled")
	}
}

func TestServiceSearchOperationsKeepsLexicalResultsWhenSemanticIndexEmpty(t *testing.T) {
	store := &fakeRawOperationStore{
		rawOperations: []entities.WBRegistryOperation{
			retrievalRankingOperation(
				"generated_get_api_v1_supplier_sales",
				"/api/v1/supplier/sales",
				"Продажи",
				"Метод возвращает информацию о продажах и возвратах.",
				[]string{"Основные отчёты"},
			),
		},
	}

	semanticRetriever := &fakeSemanticCandidateRetriever{
		results: []SemanticOperationResult{},
	}

	service, err := New(ServiceConfig{
		Store:                    store,
		SemanticRetriever:        semanticRetriever,
		SemanticExpansionEnabled: true,
		SemanticExpansionLimit:   20,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.SearchOperationsWithDiagnostics(context.Background(), wbregistry.SearchQuery{
		Query:        "продажи",
		Limit:        10,
		ReadonlyOnly: true,
		ExcludeJam:   true,
	})
	if err != nil {
		t.Fatalf("search operations with diagnostics: %v", err)
	}

	assertOperationIDs(t, result.Operations, []string{
		"generated_get_api_v1_supplier_sales",
	})

	if result.Diagnostics.LexicalCandidates != 1 {
		t.Fatalf("expected 1 lexical candidate, got %d", result.Diagnostics.LexicalCandidates)
	}

	if result.Diagnostics.SemanticCandidates != 0 {
		t.Fatalf("expected 0 semantic candidates, got %d", result.Diagnostics.SemanticCandidates)
	}

	if result.Diagnostics.MergedCandidates != 1 {
		t.Fatalf("expected 1 merged candidate, got %d", result.Diagnostics.MergedCandidates)
	}

	if result.Diagnostics.ReturnedCandidates != 1 {
		t.Fatalf("expected 1 returned candidate, got %d", result.Diagnostics.ReturnedCandidates)
	}

	if !result.Diagnostics.SemanticExpansionEnabled {
		t.Fatal("expected semantic expansion enabled")
	}

	if semanticRetriever.calls != 1 {
		t.Fatalf("expected semantic retriever to be called once, got %d", semanticRetriever.calls)
	}
}
