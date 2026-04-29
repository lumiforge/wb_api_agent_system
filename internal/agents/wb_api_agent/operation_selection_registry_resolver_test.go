package wb_api_agent

import (
	"errors"
	"testing"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Protects selector-to-composer registry resolution from semantic fallback or invented operations.
func TestOperationSelectionRegistryResolverResolvesSelectedOperationsInSelectorOrder(t *testing.T) {
	resolver := NewOperationSelectionRegistryResolver()

	plan := validResolverSelectionPlan()
	plan.SelectedOperations = []entities.SelectedOperation{
		{
			OperationID:   "operation_sales",
			Purpose:       "Fetch sales.",
			DependsOn:     []string{},
			InputStrategy: entities.OperationInputStrategyBusinessEntities,
		},
		{
			OperationID:   "operation_stocks",
			Purpose:       "Fetch stocks.",
			DependsOn:     []string{},
			InputStrategy: entities.OperationInputStrategyBusinessEntities,
		},
	}

	candidates := []entities.WBRegistryOperation{
		validResolverRegistryOperation("operation_stocks"),
		validResolverRegistryOperation("operation_sales"),
	}

	resolved, err := resolver.Resolve(plan, candidates)
	if err != nil {
		t.Fatalf("expected resolved operations, got %v", err)
	}

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved operations, got %#v", resolved)
	}

	if resolved[0].OperationID != "operation_sales" {
		t.Fatalf("expected first resolved operation_sales, got %q", resolved[0].OperationID)
	}

	if resolved[1].OperationID != "operation_stocks" {
		t.Fatalf("expected second resolved operation_stocks, got %q", resolved[1].OperationID)
	}
}

func TestOperationSelectionRegistryResolverRejectsSelectedOperationOutsideCandidates(t *testing.T) {
	resolver := NewOperationSelectionRegistryResolver()

	plan := validResolverSelectionPlan()
	plan.SelectedOperations[0].OperationID = "invented_operation"

	_, err := resolver.Resolve(plan, []entities.WBRegistryOperation{
		validResolverRegistryOperation("operation_stocks"),
	})

	assertOperationSelectionRegistryResolutionError(t, err, "selected_operation_not_in_candidates")
}

func TestOperationSelectionRegistryResolverRejectsDuplicateSelectedOperations(t *testing.T) {
	resolver := NewOperationSelectionRegistryResolver()

	plan := validResolverSelectionPlan()
	plan.SelectedOperations = append(plan.SelectedOperations, entities.SelectedOperation{
		OperationID:   "operation_stocks",
		Purpose:       "Fetch stocks again.",
		DependsOn:     []string{},
		InputStrategy: entities.OperationInputStrategyBusinessEntities,
	})

	_, err := resolver.Resolve(plan, []entities.WBRegistryOperation{
		validResolverRegistryOperation("operation_stocks"),
	})

	assertOperationSelectionRegistryResolutionError(t, err, "duplicate_selected_operation_id")
}

func TestOperationSelectionRegistryResolverRejectsDuplicateCandidateOperations(t *testing.T) {
	resolver := NewOperationSelectionRegistryResolver()

	_, err := resolver.Resolve(validResolverSelectionPlan(), []entities.WBRegistryOperation{
		validResolverRegistryOperation("operation_stocks"),
		validResolverRegistryOperation("operation_stocks"),
	})

	assertOperationSelectionRegistryResolutionError(t, err, "duplicate_candidate_operation_id")
}

func TestOperationSelectionRegistryResolverRejectsEmptySelectedOperationID(t *testing.T) {
	resolver := NewOperationSelectionRegistryResolver()

	plan := validResolverSelectionPlan()
	plan.SelectedOperations[0].OperationID = ""

	_, err := resolver.Resolve(plan, []entities.WBRegistryOperation{
		validResolverRegistryOperation("operation_stocks"),
	})

	assertOperationSelectionRegistryResolutionError(t, err, "empty_selected_operation_id")
}

func validResolverSelectionPlan() entities.OperationSelectionPlan {
	return entities.OperationSelectionPlan{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		Status:        entities.OperationSelectionStatusReadyForComposition,
		SelectedOperations: []entities.SelectedOperation{
			{
				OperationID:   "operation_stocks",
				Purpose:       "Fetch stocks.",
				DependsOn:     []string{},
				InputStrategy: entities.OperationInputStrategyBusinessEntities,
			},
		},
		MissingInputs:      []entities.MissingBusinessInput{},
		RejectedCandidates: []entities.RejectedOperationCandidate{},
		Warnings:           []entities.PlanWarning{},
	}
}

func validResolverRegistryOperation(operationID string) entities.WBRegistryOperation {
	readonly := true

	return entities.WBRegistryOperation{
		Marketplace:              "wildberries",
		SourceFile:               "products.yaml",
		OperationID:              operationID,
		Method:                   "POST",
		ServerURL:                "https://marketplace-api.wildberries.ru",
		PathTemplate:             "/api/v3/stocks/{warehouseId}",
		Tags:                     []string{"Остатки"},
		Category:                 "marketplace",
		Summary:                  "Остатки товаров",
		Description:              "Возвращает остатки товаров.",
		XReadonlyMethod:          &readonly,
		XCategory:                "marketplace",
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

func assertOperationSelectionRegistryResolutionError(
	t *testing.T,
	err error,
	expectedReason string,
) {
	t.Helper()

	var resolutionError OperationSelectionRegistryResolutionError
	if !errors.As(err, &resolutionError) {
		t.Fatalf("expected OperationSelectionRegistryResolutionError, got %T: %v", err, err)
	}

	if resolutionError.Code != OperationSelectionRegistryResolutionErrorCode {
		t.Fatalf("expected code %q, got %q", OperationSelectionRegistryResolutionErrorCode, resolutionError.Code)
	}

	if resolutionError.Reason != expectedReason {
		t.Fatalf("expected reason %q, got %q", expectedReason, resolutionError.Reason)
	}
}
