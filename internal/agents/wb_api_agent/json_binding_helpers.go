package wb_api_agent

import (
	"strings"

	"google.golang.org/genai"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Keeps boundary JSON coercion helpers shared by selector, validator, and transitional plan normalization.
func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}

	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func valueBindingFromMap(value map[string]any) entities.ValueBinding {
	binding := entities.ValueBinding{
		Source:     stringFromAny(value["source"]),
		Value:      value["value"],
		InputName:  stringFromAny(value["input_name"]),
		StepID:     stringFromAny(value["step_id"]),
		OutputName: stringFromAny(value["output_name"]),
		Expression: stringFromAny(value["expression"]),
		SecretName: stringFromAny(value["secret_name"]),
		Required:   boolFromAny(value["required"]),
	}

	if binding.Source == "" {
		binding.Source = "static"
	}
	if _, ok := value["required"]; !ok {
		binding.Required = true
	}

	return binding
}

func stringFromAny(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}

	return typed
}

func boolFromAny(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}

	return typed
}
