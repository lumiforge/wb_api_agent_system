package composer

import (
	"strconv"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Converts explicit structural BusinessRequest facts into deterministic executable bindings.
func pathInputsFromBusinessRequest(
	operation entities.WBRegistryOperation,
	request entities.BusinessRequest,
) map[string]entities.InputValue {
	inputs := map[string]entities.InputValue{}

	for _, pathParam := range requiredPathParams(operation) {
		inputName, inputValue, ok := pathParamBindingFromBusinessRequest(pathParam, request)
		if !ok {
			continue
		}

		inputs[inputName] = inputValue
	}

	return inputs
}

func pathParamBindingsFromBusinessRequest(
	operation entities.WBRegistryOperation,
	request entities.BusinessRequest,
) map[string]entities.ValueBinding {
	bindings := map[string]entities.ValueBinding{}

	for _, pathParam := range requiredPathParams(operation) {
		inputName, _, ok := pathParamBindingFromBusinessRequest(pathParam, request)
		if !ok {
			continue
		}

		bindings[pathParam] = entities.ValueBinding{
			Source:    "input",
			InputName: inputName,
			Required:  true,
		}
	}

	return bindings
}

func pathParamBindingFromBusinessRequest(
	pathParam string,
	request entities.BusinessRequest,
) (string, entities.InputValue, bool) {
	switch canonicalInputName(pathParam) {
	case "warehouse_id":
		warehouseID, ok := explicitIntegerEntity(request.Entities, "warehouse_id", "warehouseId", "warehouseID")
		if !ok {
			return "", entities.InputValue{}, false
		}

		return "warehouse_id", entities.InputValue{
			Type:        "integer",
			Required:    true,
			Value:       warehouseID,
			Description: "Seller warehouse ID.",
		}, true

	default:
		return "", entities.InputValue{}, false
	}
}

func requiredPathParams(operation entities.WBRegistryOperation) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, pathParam := range pathTemplateParamNames(operation.PathTemplate) {
		if pathParam == "" || seen[pathParam] {
			continue
		}

		seen[pathParam] = true
		result = append(result, pathParam)
	}

	for _, pathParam := range requiredSchemaFields(schemaParamNames(operation.PathParamsSchemaJSON)) {
		if pathParam == "" || seen[pathParam] {
			continue
		}

		seen[pathParam] = true
		result = append(result, pathParam)
	}

	return result
}

func queryInputsFromBusinessRequest(
	queryParamsSchemaJSON string,
	request entities.BusinessRequest,
) map[string]entities.InputValue {
	inputs := map[string]entities.InputValue{}

	for _, queryParam := range requiredSchemaFields(schemaParamNames(queryParamsSchemaJSON)) {
		inputName, inputValue, ok := queryParamBindingFromBusinessRequest(queryParam, request)
		if !ok {
			continue
		}

		inputs[inputName] = inputValue
	}

	return inputs
}

func queryParamBindingsFromBusinessRequest(
	queryParamsSchemaJSON string,
	request entities.BusinessRequest,
) map[string]entities.ValueBinding {
	bindings := map[string]entities.ValueBinding{}

	for _, queryParam := range requiredSchemaFields(schemaParamNames(queryParamsSchemaJSON)) {
		inputName, _, ok := queryParamBindingFromBusinessRequest(queryParam, request)
		if !ok {
			continue
		}

		bindings[queryParam] = entities.ValueBinding{
			Source:    "input",
			InputName: inputName,
			Required:  true,
		}
	}

	return bindings
}

func queryParamBindingFromBusinessRequest(
	queryParam string,
	request entities.BusinessRequest,
) (string, entities.InputValue, bool) {
	switch canonicalInputName(queryParam) {
	case "date_from":
		if request.Period == nil || strings.TrimSpace(request.Period.From) == "" {
			return "", entities.InputValue{}, false
		}

		return "date_from", entities.InputValue{
			Type:        "string",
			Required:    true,
			Value:       request.Period.From,
			Description: "Period start date.",
		}, true

	case "date_to":
		if request.Period == nil || strings.TrimSpace(request.Period.To) == "" {
			return "", entities.InputValue{}, false
		}

		return "date_to", entities.InputValue{
			Type:        "string",
			Required:    true,
			Value:       request.Period.To,
			Description: "Period end date.",
		}, true

	default:
		return "", entities.InputValue{}, false
	}
}

func bodyInputsFromBusinessRequest(
	requestBodySchemaJSON string,
	request entities.BusinessRequest,
) map[string]entities.InputValue {
	inputs := map[string]entities.InputValue{}

	for name, binding := range requestBodyDefaultBindingsFromRegistrySchema(requestBodySchemaJSON) {
		// WHY: Registry schema defaults are explicit structural facts and may be used without LLM inference.
		inputs[name] = entities.InputValue{
			Type:        inputValueTypeFromStaticValue(binding.Value),
			Required:    binding.Required,
			Value:       binding.Value,
			Description: "Registry schema default.",
		}
	}

	for _, bodyField := range requiredRequestBodyFields(requestBodySchemaJSON) {
		inputName, inputValue, ok := bodyFieldBindingFromBusinessRequest(bodyField, request)
		if !ok {
			continue
		}

		inputs[inputName] = inputValue
	}

	return inputs
}

func bodyBindingsFromBusinessRequest(
	requestBodySchemaJSON string,
	request entities.BusinessRequest,
) map[string]any {
	body := map[string]any{}

	for name, binding := range requestBodyDefaultBindingsFromRegistrySchema(requestBodySchemaJSON) {
		// WHY: Registry schema defaults are explicit structural facts and may be used without LLM inference.
		body[name] = binding
	}

	for _, bodyField := range requiredRequestBodyFields(requestBodySchemaJSON) {
		inputName, _, ok := bodyFieldBindingFromBusinessRequest(bodyField, request)
		if !ok {
			continue
		}

		body[bodyField] = entities.ValueBinding{
			Source:    "input",
			InputName: inputName,
			Required:  true,
		}
	}

	return body
}

func requestBodyDefaultBindingsFromRegistrySchema(requestBodySchemaJSON string) map[string]entities.ValueBinding {
	switch requestBodySchemaRefName(requestBodySchemaJSON) {
	case "InventoryRequest":
		return map[string]entities.ValueBinding{
			"limit": {
				Source:   "static",
				Value:    250000,
				Required: true,
			},
			"offset": {
				Source:   "static",
				Value:    0,
				Required: true,
			},
		}

	default:
		return map[string]entities.ValueBinding{}
	}
}

func inputValueTypeFromStaticValue(value any) string {
	switch value.(type) {
	case int, int64, float64:
		return "integer"
	case string:
		return "string"
	case bool:
		return "boolean"
	case []any, []int, []int64, []float64, []string:
		return "array"
	default:
		return "object"
	}
}

func bodyFieldBindingFromBusinessRequest(
	bodyField string,
	request entities.BusinessRequest,
) (string, entities.InputValue, bool) {
	switch canonicalInputName(bodyField) {
	case "chrt_ids":
		chrtIDs, ok := explicitIntegerArrayEntity(request.Entities, "chrt_ids", "chrtIds", "chrtIDs")
		if !ok || len(chrtIDs) == 0 {
			return "", entities.InputValue{}, false
		}

		return "chrt_ids", entities.InputValue{
			Type:        "array",
			Required:    true,
			Value:       chrtIDs,
			Description: "Product size IDs.",
		}, true

	default:
		return "", entities.InputValue{}, false
	}
}

func mergeInputValues(
	target map[string]entities.InputValue,
	source map[string]entities.InputValue,
) {
	for key, value := range source {
		// WHY: Explicit path/entity bindings are already canonical; later sources must not silently overwrite them.
		if _, exists := target[key]; exists {
			continue
		}

		target[key] = value
	}
}

func requiredSchemaFields(fields map[string]bool) []string {
	result := make([]string, 0)

	for fieldName, required := range fields {
		if required {
			result = append(result, fieldName)
		}
	}

	return result
}

func explicitIntegerEntity(entitiesMap map[string]any, names ...string) (int, bool) {
	for _, name := range names {
		value, ok := entitiesMap[name]
		if !ok {
			continue
		}

		parsed, ok := explicitIntegerValue(value)
		if ok {
			return parsed, true
		}
	}

	return 0, false
}

func explicitIntegerValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}

		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func explicitIntegerArrayEntity(entitiesMap map[string]any, names ...string) ([]int, bool) {
	for _, name := range names {
		value, ok := entitiesMap[name]
		if !ok {
			continue
		}

		parsed, ok := explicitIntegerArrayValue(value)
		if ok {
			return parsed, true
		}
	}

	return nil, false
}

func explicitIntegerArrayValue(value any) ([]int, bool) {
	switch typed := value.(type) {
	case []int:
		return typed, true
	case []int64:
		result := make([]int, 0, len(typed))
		for _, item := range typed {
			result = append(result, int(item))
		}

		return result, true
	case []float64:
		result := make([]int, 0, len(typed))
		for _, item := range typed {
			if item != float64(int(item)) {
				return nil, false
			}

			result = append(result, int(item))
		}

		return result, true
	case []any:
		result := make([]int, 0, len(typed))
		for _, item := range typed {
			parsed, ok := explicitIntegerValue(item)
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
