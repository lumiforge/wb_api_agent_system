package wb_api_agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func normalizePlan(request entities.BusinessRequest, plan *entities.ApiExecutionPlan) {
	if plan == nil {
		return
	}

	normalizeInputs(request, plan)

	for stepIndex := range plan.Steps {
		step := &plan.Steps[stepIndex]

		step.Request.PathParams = normalizeValueBindingMap(step.Request.PathParams, plan.Inputs)
		step.Request.QueryParams = normalizeValueBindingMap(step.Request.QueryParams, plan.Inputs)
		step.Request.Body = normalizeRequestBody(step.Request.Body, plan.Inputs)
	}
}

func normalizeInputs(request entities.BusinessRequest, plan *entities.ApiExecutionPlan) {
	if plan.Inputs == nil {
		plan.Inputs = map[string]entities.InputValue{}
	}

	normalized := make(map[string]entities.InputValue, len(plan.Inputs))

	for name, input := range plan.Inputs {
		normalizedName := canonicalInputName(name)
		input.Value = unwrapInputValue(input.Value)

		if existing, exists := normalized[normalizedName]; exists {
			if isEmptyInputValue(existing.Value) && !isEmptyInputValue(input.Value) {
				normalized[normalizedName] = input
			}
			continue
		}

		normalized[normalizedName] = input
	}

	plan.Inputs = normalized

	ensureInputFromEntity(plan, request, "warehouse_id", "warehouse_id", "integer", "Seller warehouse ID.")
	ensureInputFromEntity(plan, request, "warehouse_id", "warehouseId", "integer", "Seller warehouse ID.")
	ensureInputFromEntity(plan, request, "chrt_ids", "chrt_ids", "array", "Product size IDs.")
	ensureInputFromEntity(plan, request, "chrt_ids", "chrtIds", "array", "Product size IDs.")

	if request.Period != nil {
		if _, ok := plan.Inputs["date_from"]; !ok && strings.TrimSpace(request.Period.From) != "" {
			plan.Inputs["date_from"] = entities.InputValue{
				Type:        "string",
				Required:    true,
				Value:       request.Period.From,
				Description: "Period start date.",
			}
		}

		if _, ok := plan.Inputs["date_to"]; !ok && strings.TrimSpace(request.Period.To) != "" {
			plan.Inputs["date_to"] = entities.InputValue{
				Type:        "string",
				Required:    false,
				Value:       request.Period.To,
				Description: "Period end date.",
			}
		}
	}
}

func unwrapInputValue(value any) any {
	switch typed := value.(type) {
	case entities.InputValue:
		return unwrapInputValue(typed.Value)
	case map[string]any:
		if nestedValue, ok := typed["value"]; ok {
			if _, hasType := typed["type"]; hasType {
				if _, hasRequired := typed["required"]; hasRequired {
					return unwrapInputValue(nestedValue)
				}
			}
		}
	}

	return value
}

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

func ensureInputFromEntity(
	plan *entities.ApiExecutionPlan,
	request entities.BusinessRequest,
	canonicalName string,
	entityName string,
	inputType string,
	description string,
) {
	if _, ok := plan.Inputs[canonicalName]; ok {
		return
	}

	value, ok := request.Entities[entityName]
	if !ok {
		return
	}

	plan.Inputs[canonicalName] = entities.InputValue{
		Type:        inputType,
		Required:    true,
		Value:       value,
		Description: description,
	}
}

func normalizeValueBindingMap(
	values map[string]entities.ValueBinding,
	inputs map[string]entities.InputValue,
) map[string]entities.ValueBinding {
	if values == nil {
		return map[string]entities.ValueBinding{}
	}

	result := make(map[string]entities.ValueBinding, len(values))

	for name, binding := range values {
		result[name] = normalizeValueBinding(name, binding, inputs)
	}

	return result
}

func normalizeValueBinding(
	fieldName string,
	binding entities.ValueBinding,
	inputs map[string]entities.InputValue,
) entities.ValueBinding {
	if binding.InputName != "" {
		binding.InputName = canonicalInputName(binding.InputName)
	}

	if binding.Source == "input" {
		return binding
	}

	inputName := canonicalInputName(fieldName)
	if _, ok := inputs[inputName]; ok {
		return entities.ValueBinding{
			Source:    "input",
			InputName: inputName,
			Required:  true,
		}
	}

	if binding.Value != nil {
		if matchedInputName := findInputByValue(inputs, binding.Value); matchedInputName != "" {
			return entities.ValueBinding{
				Source:    "input",
				InputName: matchedInputName,
				Required:  true,
			}
		}
	}

	if binding.Source == "" {
		binding.Source = "static"
	}

	return binding
}

func normalizeRequestBody(body any, inputs map[string]entities.InputValue) any {
	bodyMap, ok := body.(map[string]any)
	if !ok {
		return body
	}

	normalized := make(map[string]any, len(bodyMap))

	for fieldName, value := range bodyMap {
		inputName := canonicalInputName(fieldName)

		if _, ok := inputs[inputName]; ok {
			// WHY: Body literals should be resolved through declared inputs so executor has one source of truth.
			normalized[fieldName] = entities.ValueBinding{
				Source:    "input",
				InputName: inputName,
				Required:  true,
			}
			continue
		}

		if valueMap, ok := value.(map[string]any); ok {
			if source, ok := valueMap["source"].(string); ok && source == "input" {
				valueMap["input_name"] = canonicalInputName(stringFromAny(valueMap["input_name"]))
				normalized[fieldName] = valueMap
				continue
			}
		}

		if matchedInputName := findInputByValue(inputs, value); matchedInputName != "" {
			normalized[fieldName] = entities.ValueBinding{
				Source:    "input",
				InputName: matchedInputName,
				Required:  true,
			}
			continue
		}

		normalized[fieldName] = value
	}

	return normalized
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

func findInputByValue(inputs map[string]entities.InputValue, value any) string {
	for inputName, input := range inputs {
		if valuesEquivalent(input.Value, value) {
			return inputName
		}
	}

	return ""
}

func valuesEquivalent(left any, right any) bool {
	if fmt.Sprint(left) == fmt.Sprint(right) {
		return true
	}

	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)

	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}
