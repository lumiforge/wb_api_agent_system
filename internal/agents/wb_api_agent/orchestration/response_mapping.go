package orchestration

import (
	"encoding/json"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

func applyResponseMappingDefaults(step *entities.ApiPlanStep, operation entities.WBRegistryOperation) bool {
	if len(step.ResponseMapping.Outputs) > 0 {
		return false
	}

	outputs := knownResponseOutputs(operation)
	if len(outputs) == 0 {
		outputs = inferResponseOutputs(operation.ResponseSchemaJSON)
	}

	if len(outputs) == 0 {
		outputs = map[string]entities.MappedOutput{
			"raw": {
				Type: "object",
				Path: "$",
			},
		}
	}

	step.ResponseMapping.Outputs = outputs

	return true
}

func knownResponseOutputs(operation entities.WBRegistryOperation) map[string]entities.MappedOutput {
	switch operation.OperationID {
	case "generated_post_api_v3_stocks_warehouseid":
		return map[string]entities.MappedOutput{
			"stocks": {
				Type: "rows",
				Path: "$.stocks",
			},
		}
	case "generated_get_api_v1_supplier_sales":
		return map[string]entities.MappedOutput{
			"sales": {
				Type: "rows",
				Path: "$",
			},
		}
	case "generated_get_api_v1_supplier_orders":
		return map[string]entities.MappedOutput{
			"orders": {
				Type: "rows",
				Path: "$",
			},
		}
	}

	return nil
}

func inferResponseOutputs(responseSchemaJSON string) map[string]entities.MappedOutput {
	var root map[string]any
	if err := json.Unmarshal([]byte(responseSchemaJSON), &root); err != nil {
		return nil
	}

	okResponse, ok := root["200"].(map[string]any)
	if !ok {
		return nil
	}

	content, ok := okResponse["content"].(map[string]any)
	if !ok {
		return nil
	}

	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil
	}

	schema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		return nil
	}

	if schema["type"] == "array" {
		return map[string]entities.MappedOutput{
			"rows": {
				Type: "rows",
				Path: "$",
			},
		}
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	outputs := make(map[string]entities.MappedOutput)

	for name, rawProperty := range properties {
		property, ok := rawProperty.(map[string]any)
		if !ok {
			continue
		}

		outputType := "object"
		if property["type"] == "array" || property["items"] != nil {
			outputType = "rows"
		}

		outputs[name] = entities.MappedOutput{
			Type: outputType,
			Path: "$." + name,
		}
	}

	if len(outputs) == 0 {
		return nil
	}

	return outputs
}

func normalizeFinalOutput(plan *entities.ApiExecutionPlan) bool {
	if plan.FinalOutput.Type == "" {
		plan.FinalOutput.Type = "object"
	}

	if plan.FinalOutput.Description == "" {
		plan.FinalOutput.Description = "Planned API outputs."
	}

	if len(plan.FinalOutput.Fields) > 0 {
		return false
	}

	plan.FinalOutput.Fields = buildFinalOutputFields(plan.Steps)

	return true
}

func buildFinalOutputFields(steps []entities.ApiPlanStep) map[string]any {
	fields := make(map[string]any)

	for _, step := range steps {
		if step.StepID == "" {
			continue
		}

		for outputName := range step.ResponseMapping.Outputs {
			fieldName := outputName
			if _, exists := fields[fieldName]; exists {
				fieldName = step.StepID + "_" + outputName
			}

			fields[fieldName] = "steps." + step.StepID + ".outputs." + outputName
		}
	}

	return fields
}

func hasOnlyRawOutput(step entities.ApiPlanStep) bool {
	if len(step.ResponseMapping.Outputs) != 1 {
		return false
	}

	_, ok := step.ResponseMapping.Outputs["raw"]

	return ok
}
