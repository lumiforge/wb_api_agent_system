package deterministic_planner

import (
	"context"
	"fmt"
	"strconv"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

const sellerWarehouseStocksOperationID = "generated_post_api_v3_stocks_warehouseid"

// PURPOSE: Builds a deterministic plan for seller warehouse stock lookup.
type SellerWarehouseStocksScenario struct {
	registry wbregistry.Retriever
}

func NewSellerWarehouseStocksScenario(registry wbregistry.Retriever) *SellerWarehouseStocksScenario {
	return &SellerWarehouseStocksScenario{
		registry: registry,
	}
}

func (s *SellerWarehouseStocksScenario) TryPlan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, bool, error) {
	if !s.matches(request) {
		return nil, false, nil
	}

	warehouseID, ok := integerEntity(request.Entities, "warehouse_id", "warehouseId")
	if !ok {
		return entities.NewNeedsClarificationPlan(request, []string{
			"Provide entities.warehouse_id.",
		}), true, nil
	}

	chrtIDs, ok := integerArrayEntity(request.Entities, "chrt_ids", "chrtIds")
	if !ok || len(chrtIDs) == 0 {
		return entities.NewNeedsClarificationPlan(request, []string{
			"Provide entities.chrt_ids as a non-empty array of product size IDs.",
		}), true, nil
	}

	operation, err := s.registry.GetOperation(ctx, sellerWarehouseStocksOperationID)
	if err != nil {
		return nil, true, err
	}

	if operation == nil {
		return entities.NewBlockedPlan(request, "required_registry_operation_not_found", []entities.PlanWarning{
			{
				Code:    "missing_operation",
				Message: "Registry operation generated_post_api_v3_stocks_warehouseid was not found.",
			},
		}), true, nil
	}

	if operation.XReadonlyMethod == nil || !*operation.XReadonlyMethod {
		return entities.NewBlockedPlan(request, "required_operation_is_not_readonly", []entities.PlanWarning{
			{
				Code:    "readonly_policy_block",
				Message: "Required registry operation is not readonly.",
			},
		}), true, nil
	}

	if request.Constraints.NoJamSubscription && operation.RequiresJam {
		return entities.NewBlockedPlan(request, "jam_operation_blocked_by_no_jam_subscription", []entities.PlanWarning{
			{
				Code:    "jam_policy_block",
				Message: "Required registry operation requires Jam subscription.",
			},
		}), true, nil
	}

	// WHY: Known readonly stock lookup can be planned deterministically without LLM selection.
	return entities.NewSellerWarehouseStocksPlan(request, *operation, warehouseID, chrtIDs), true, nil
}

func (s *SellerWarehouseStocksScenario) matches(request entities.BusinessRequest) bool {
	// WHY: Mixed business intents like inventory+sales must fall through to registry search and LLM fallback instead of being captured by the simple stocks scenario.
	return request.Intent == "get_seller_warehouse_stocks"
}

func integerEntity(entities map[string]any, names ...string) (int, bool) {
	for _, name := range names {
		value, ok := entities[name]
		if !ok {
			continue
		}

		parsed, ok := integerFromAny(value)
		if ok {
			return parsed, true
		}
	}

	return 0, false
}

func integerArrayEntity(entities map[string]any, names ...string) ([]int, bool) {
	for _, name := range names {
		value, ok := entities[name]
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case []int:
			return typed, true
		case []float64:
			result := make([]int, 0, len(typed))
			for _, item := range typed {
				result = append(result, int(item))
			}
			return result, true
		case []any:
			result := make([]int, 0, len(typed))
			for _, item := range typed {
				parsed, ok := integerFromAny(item)
				if !ok {
					return nil, false
				}

				result = append(result, parsed)
			}
			return result, true
		default:
			return nil, false
		}
	}

	return nil, false
}

func integerFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	default:
		parsed, err := strconv.Atoi(fmt.Sprint(typed))
		return parsed, err == nil
	}
}
