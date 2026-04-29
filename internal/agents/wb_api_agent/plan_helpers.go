package wb_api_agent

import (
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func isEmptyInputValue(value any) bool {
	if value == nil {
		return true
	}

	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []int:
		return len(typed) == 0
	default:
		return false
	}
}

func canonicalInputName(name string) string {
	switch name {
	case "warehouseId", "warehouseID", "warehouse_id":
		return "warehouse_id"
	case "chrtIds", "chrtIDs", "chrt_ids":
		return "chrt_ids"
	case "dateFrom", "date_from":
		return "date_from"
	case "dateTo", "date_to":
		return "date_to"
	default:
		return camelToSnake(name)
	}
}

func camelToSnake(value string) string {
	var builder strings.Builder

	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			builder.WriteRune('_')
		}

		builder.WriteRune(r)
	}

	return strings.ToLower(builder.String())
}

// PURPOSE: Normalizes composed plan output slices/maps that must be non-nil for stable JSON contracts.
func nonNilWarnings(value []entities.PlanWarning) []entities.PlanWarning {
	if value == nil {
		return []entities.PlanWarning{}
	}

	return value
}

func nonNilStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}

	return value
}
