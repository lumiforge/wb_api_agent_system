package wb_registry_retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

var _ wbregistry.Retriever = (*Service)(nil)

// PURPOSE: Defines optional semantic candidate expansion without making embeddings source-of-truth.
type SemanticCandidateRetriever interface {
	Search(ctx context.Context, query wbregistry.SearchQuery) ([]SemanticOperationResult, error)
}

// PURPOSE: Configures deterministic registry retrieval and optional semantic candidate expansion.
type ServiceConfig struct {
	Store                    wbregistry.RawOperationStore
	SemanticRetriever        SemanticCandidateRetriever
	SemanticExpansionEnabled bool
	SemanticExpansionLimit   int
}

// PURPOSE: Owns deterministic registry retrieval and ranking above the storage adapter boundary.
type Service struct {
	store                    wbregistry.RawOperationStore
	semanticRetriever        SemanticCandidateRetriever
	semanticExpansionEnabled bool
	semanticExpansionLimit   int
}

func New(cfg ServiceConfig) (*Service, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("registry raw operation store is required")
	}

	if cfg.SemanticExpansionEnabled && cfg.SemanticRetriever == nil {
		return nil, fmt.Errorf("semantic retriever is required when semantic expansion is enabled")
	}

	semanticExpansionLimit := cfg.SemanticExpansionLimit
	if semanticExpansionLimit <= 0 {
		semanticExpansionLimit = 20
	}

	return &Service{
		store:                    cfg.Store,
		semanticRetriever:        cfg.SemanticRetriever,
		semanticExpansionEnabled: cfg.SemanticExpansionEnabled,
		semanticExpansionLimit:   semanticExpansionLimit,
	}, nil
}

func (s *Service) SearchOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	result, err := s.SearchOperationsWithDiagnostics(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Operations, nil
}

func (s *Service) SearchOperationsWithDiagnostics(
	ctx context.Context,
	query wbregistry.SearchQuery,
) (wbregistry.SearchResult, error) {
	limit := normalizedSearchLimit(query.Limit)
	rawQuery := query
	rawQuery.Limit = rankedSearchPreLimit(limit)
	rawQuery.Query = strings.Join(searchTokens(query.Query), " ")

	operations, err := s.store.RawSearchOperations(ctx, rawQuery)
	if err != nil {
		return wbregistry.SearchResult{}, fmt.Errorf("raw search registry operations: %w", err)
	}

	lexicalCandidateCount := len(operations)
	semanticCandidateCount := 0

	if s.semanticExpansionEnabled {
		semanticOperations, err := s.semanticExpansionOperations(ctx, query)
		if err != nil {
			return wbregistry.SearchResult{}, err
		}

		semanticCandidateCount = len(semanticOperations)

		// WHY: Semantic retrieval only expands candidates; final ordering remains deterministic ranking.
		operations = mergeOperationCandidates(operations, semanticOperations)
	}

	mergedCandidateCount := len(operations)

	// WHY: Retrieval ranking belongs above storage; SQLite remains source-of-record.
	ranked := rankOperations(operations, query.Query, limit)

	return wbregistry.SearchResult{
		Operations: ranked,
		Diagnostics: wbregistry.SearchDiagnostics{
			LexicalCandidates:        lexicalCandidateCount,
			SemanticCandidates:       semanticCandidateCount,
			MergedCandidates:         mergedCandidateCount,
			ReturnedCandidates:       len(ranked),
			SemanticExpansionEnabled: s.semanticExpansionEnabled,
		},
	}, nil
}

func (s *Service) semanticExpansionOperations(
	ctx context.Context,
	query wbregistry.SearchQuery,
) ([]entities.WBRegistryOperation, error) {
	semanticQuery := query
	semanticQuery.Limit = s.semanticExpansionLimit

	results, err := s.semanticRetriever.Search(ctx, semanticQuery)
	if err != nil {
		return nil, fmt.Errorf("semantic registry candidate expansion: %w", err)
	}

	operations := make([]entities.WBRegistryOperation, 0, len(results))
	for _, result := range results {
		operations = append(operations, result.Operation)
	}

	return operations, nil
}

func mergeOperationCandidates(
	primary []entities.WBRegistryOperation,
	expansion []entities.WBRegistryOperation,
) []entities.WBRegistryOperation {
	merged := make([]entities.WBRegistryOperation, 0, len(primary)+len(expansion))
	seen := make(map[string]bool, len(primary)+len(expansion))

	for _, operation := range primary {
		if operation.OperationID == "" || seen[operation.OperationID] {
			continue
		}

		seen[operation.OperationID] = true
		merged = append(merged, operation)
	}

	for _, operation := range expansion {
		if operation.OperationID == "" || seen[operation.OperationID] {
			continue
		}

		seen[operation.OperationID] = true
		merged = append(merged, operation)
	}

	return merged
}

func (s *Service) GetOperation(
	ctx context.Context,
	operationID string,
) (*entities.WBRegistryOperation, error) {
	operation, err := s.store.GetOperation(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("get registry operation: %w", err)
	}

	return operation, nil
}

func (s *Service) Stats(ctx context.Context) (wbregistry.Stats, error) {
	stats, err := s.store.Stats(ctx)
	if err != nil {
		return wbregistry.Stats{}, fmt.Errorf("get registry stats: %w", err)
	}

	return stats, nil
}

func normalizedSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}

	if limit > 100 {
		return 100
	}

	return limit
}

func rankedSearchPreLimit(limit int) int {
	preLimit := limit * 8
	if preLimit < 80 {
		return 80
	}
	if preLimit > 300 {
		return 300
	}

	return preLimit
}
