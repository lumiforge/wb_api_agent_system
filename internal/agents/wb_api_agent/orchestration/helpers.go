package orchestration

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

func valueBindingFromMap(value map[string]any) entities.ValueBinding {
	binding := entities.ValueBinding{Source: stringFromAny(value["source"]), Value: value["value"], InputName: stringFromAny(value["input_name"]), StepID: stringFromAny(value["step_id"]), OutputName: stringFromAny(value["output_name"]), Expression: stringFromAny(value["expression"]), SecretName: stringFromAny(value["secret_name"]), Required: boolFromAny(value["required"])}
	if binding.Source == "" {
		binding.Source = "static"
	}
	if _, ok := value["required"]; !ok {
		binding.Required = true
	}
	return binding
}
func stringFromAny(value any) string { s, _ := value.(string); return s }
func boolFromAny(value any) bool     { b, _ := value.(bool); return b }
