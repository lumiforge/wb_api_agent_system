package composer

import (
	"encoding/json"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

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
	var b strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteRune('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
func pathTemplateParamNames(pathTemplate string) []string {
	names := []string{}
	for {
		s := strings.Index(pathTemplate, "{")
		if s == -1 {
			break
		}
		e := strings.Index(pathTemplate[s:], "}")
		if e == -1 {
			break
		}
		n := strings.TrimSpace(pathTemplate[s+1 : s+e])
		if n != "" {
			names = append(names, n)
		}
		pathTemplate = pathTemplate[s+e+1:]
	}
	return names
}
func schemaParamNames(schemaJSON string) map[string]bool {
	res := map[string]bool{}
	var schema map[string]any
	if json.Unmarshal([]byte(schemaJSON), &schema) != nil {
		return res
	}
	for n, v := range schema {
		req := false
		if m, ok := v.(map[string]any); ok {
			req, _ = m["required"].(bool)
		}
		res[n] = req
	}
	return res
}
func requiredRequestBodyFields(schemaJSON string) []string {
	var root map[string]any
	if json.Unmarshal([]byte(schemaJSON), &root) != nil {
		return []string{}
	}
	c, ok := root["content"].(map[string]any)
	if !ok {
		return []string{}
	}
	jc, ok := c["application/json"].(map[string]any)
	if !ok {
		return []string{}
	}
	s, ok := jc["schema"].(map[string]any)
	if !ok {
		return []string{}
	}
	rr, ok := s["required"].([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(rr))
	for _, it := range rr {
		if n, ok := it.(string); ok && n != "" {
			out = append(out, n)
		}
	}
	return out
}

func nonNilStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}
func nonNilWarnings(value []entities.PlanWarning) []entities.PlanWarning {
	if value == nil {
		return []entities.PlanWarning{}
	}
	return value
}

func requestBodyRequired(schemaJSON string) bool {
	var root map[string]any
	if json.Unmarshal([]byte(schemaJSON), &root) != nil {
		return false
	}
	required, _ := root["required"].(bool)
	return required
}

func requestBodySchemaRefName(schemaJSON string) string {
	var root map[string]any
	if json.Unmarshal([]byte(schemaJSON), &root) != nil {
		return ""
	}

	content, ok := root["content"].(map[string]any)
	if !ok {
		return ""
	}

	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		return ""
	}

	schema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		return ""
	}

	ref, ok := schema["$ref"].(string)
	if !ok {
		return ""
	}

	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(ref, prefix))
}
