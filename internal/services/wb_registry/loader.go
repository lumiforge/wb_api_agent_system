package wb_registry

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"gopkg.in/yaml.v3"
)

type OperationStore interface {
	ReplaceAll(ctx context.Context, operations []entities.WBRegistryOperation) error
}

// PURPOSE: Loads Wildberries OpenAPI YAML files and indexes operations for planner retrieval.
type Loader struct {
	store OperationStore
}

type LoadResult struct {
	FilesLoaded           int
	OperationsLoaded      int
	GeneratedOperationIDs int
	ReadOperations        int
	WriteOperations       int
	UnknownRiskOperations int
	JamOnlyOperations     int
}

type openAPIDocument struct {
	Info     openAPIInfo                `yaml:"info"`
	Security []map[string][]string      `yaml:"security"`
	Paths    map[string]openAPIPathItem `yaml:"paths"`
}

type openAPIInfo struct {
	Title     string `yaml:"title"`
	Version   string `yaml:"version"`
	XFileName string `yaml:"x-file-name"`
}

type openAPIPathItem struct {
	Servers    []openAPIServer    `yaml:"servers"`
	Parameters []openAPIParameter `yaml:"parameters"`
	Get        *openAPIOperation  `yaml:"get"`
	Post       *openAPIOperation  `yaml:"post"`
	Put        *openAPIOperation  `yaml:"put"`
	Patch      *openAPIOperation  `yaml:"patch"`
	Delete     *openAPIOperation  `yaml:"delete"`
}

type openAPIServer struct {
	URL string `yaml:"url"`
}

type openAPIOperation struct {
	OperationID     string                `yaml:"operationId"`
	Servers         []openAPIServer       `yaml:"servers"`
	Security        []map[string][]string `yaml:"security"`
	XReadonlyMethod *bool                 `yaml:"x-readonly-method"`
	XCategory       string                `yaml:"x-category"`
	XTokenTypes     []string              `yaml:"x-token-types"`
	Tags            []string              `yaml:"tags"`
	Summary         string                `yaml:"summary"`
	Description     string                `yaml:"description"`
	Parameters      []openAPIParameter    `yaml:"parameters"`
	RequestBody     yaml.Node             `yaml:"requestBody"`
	Responses       yaml.Node             `yaml:"responses"`
}

type openAPIParameter struct {
	In          string    `yaml:"in"`
	Name        string    `yaml:"name"`
	Required    bool      `yaml:"required"`
	Description string    `yaml:"description"`
	Schema      yaml.Node `yaml:"schema"`
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func NewLoader(store OperationStore) *Loader {
	return &Loader{store: store}
}

func (l *Loader) LoadFromDir(ctx context.Context, dir string) (LoadResult, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return LoadResult{}, fmt.Errorf("glob wb api yaml files: %w", err)
	}

	var allOperations []entities.WBRegistryOperation
	result := LoadResult{
		FilesLoaded: len(files),
	}

	for _, file := range files {
		operations, generatedIDs, err := parseOpenAPIFile(file)
		if err != nil {
			return LoadResult{}, err
		}

		for _, operation := range operations {
			switch {
			case operation.XReadonlyMethod == nil:
				result.UnknownRiskOperations++
			case *operation.XReadonlyMethod:
				result.ReadOperations++
			default:
				result.WriteOperations++
			}

			if operation.RequiresJam {
				result.JamOnlyOperations++
			}
		}

		result.GeneratedOperationIDs += generatedIDs
		allOperations = append(allOperations, operations...)
	}

	if err := l.store.ReplaceAll(ctx, allOperations); err != nil {
		return LoadResult{}, err
	}

	result.OperationsLoaded = len(allOperations)

	return result, nil
}

func parseOpenAPIFile(filePath string) ([]entities.WBRegistryOperation, int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("read openapi yaml %s: %w", filePath, err)
	}

	var document openAPIDocument
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, 0, fmt.Errorf("parse openapi yaml %s: %w", filePath, err)
	}

	sourceFile := filepath.Base(filePath)
	if document.Info.XFileName != "" {
		sourceFile = document.Info.XFileName + ".yaml"
	}

	operations := make([]entities.WBRegistryOperation, 0)
	generatedIDs := 0

	for pathTemplate, pathItem := range document.Paths {
		for _, candidate := range []struct {
			method    string
			operation *openAPIOperation
		}{
			{method: "GET", operation: pathItem.Get},
			{method: "POST", operation: pathItem.Post},
			{method: "PUT", operation: pathItem.Put},
			{method: "PATCH", operation: pathItem.Patch},
			{method: "DELETE", operation: pathItem.Delete},
		} {
			if candidate.operation == nil {
				continue
			}

			operation, generated := buildRegistryOperation(
				sourceFile,
				pathTemplate,
				candidate.method,
				pathItem.Servers,
				pathItem.Parameters,
				document.Security,
				candidate.operation,
			)

			if generated {
				generatedIDs++
			}

			operations = append(operations, operation)
		}
	}

	return operations, generatedIDs, nil
}

func buildRegistryOperation(
	sourceFile string,
	pathTemplate string,
	method string,
	pathServers []openAPIServer,
	pathParameters []openAPIParameter,
	documentSecurity []map[string][]string,
	operation *openAPIOperation,
) (entities.WBRegistryOperation, bool) {
	serverURL := firstServerURL(operation.Servers)
	if serverURL == "" {
		serverURL = firstServerURL(pathServers)
	}

	operationID, generated := normalizeOperationID(operation.OperationID, method, pathTemplate)
	parameters := append([]openAPIParameter{}, pathParameters...)
	parameters = append(parameters, operation.Parameters...)

	return entities.WBRegistryOperation{
		Marketplace:           "wildberries",
		SourceFile:            sourceFile,
		OperationID:           operationID,
		Method:                method,
		ServerURL:             serverURL,
		PathTemplate:          pathTemplate,
		Tags:                  nonNilStringSlice(operation.Tags),
		Category:              firstString(operation.Tags),
		Summary:               operation.Summary,
		Description:           operation.Description,
		XReadonlyMethod:       operation.XReadonlyMethod,
		XCategory:             operation.XCategory,
		XTokenTypes:           nonNilStringSlice(operation.XTokenTypes),
		PathParamsSchemaJSON:  mustObjectJSON(parameterSchema(parameters, "path", false)),
		QueryParamsSchemaJSON: mustObjectJSON(parameterSchema(parameters, "query", false)),
		HeadersSchemaJSON:     mustObjectJSON(parameterSchema(parameters, "header", requiresAuthorization(documentSecurity, operation.Security))),
		RequestBodySchemaJSON: mustObjectJSON(nodeToAny(operation.RequestBody)),
		ResponseSchemaJSON:    mustObjectJSON(nodeToAny(operation.Responses)),

		RateLimitNotes:           extractRateLimitNotes(operation.Description),
		SubscriptionRequirements: extractSubscriptionRequirements(operation.Description),
		RequiresJam:              requiresJam(operationID, operation.Summary, operation.Description, operation.Tags),
	}, generated
}

func normalizeOperationID(operationID string, method string, pathTemplate string) (string, bool) {
	if strings.TrimSpace(operationID) != "" {
		return strings.TrimSpace(operationID), false
	}

	return "generated_" + strings.ToLower(method) + "_" + sanitizePathForID(pathTemplate), true
}

func sanitizePathForID(pathTemplate string) string {
	var builder strings.Builder

	for _, r := range strings.Trim(pathTemplate, "/") {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
		default:
			builder.WriteRune('_')
		}
	}

	result := strings.Trim(builder.String(), "_")
	result = regexp.MustCompile(`_+`).ReplaceAllString(result, "_")
	if result == "" {
		return "root"
	}

	return result
}

func parameterSchema(parameters []openAPIParameter, location string, includeAuthorization bool) map[string]any {
	result := make(map[string]any)

	if includeAuthorization {
		result["Authorization"] = map[string]any{
			"type":        "apiKey",
			"in":          "header",
			"required":    true,
			"description": "WB API authorization header supplied by executor_secret.",
		}
	}

	for _, parameter := range parameters {
		if parameter.In != location || parameter.Name == "" {
			continue
		}

		result[parameter.Name] = map[string]any{
			"required":    parameter.Required,
			"description": parameter.Description,

			"schema": nodeToAny(parameter.Schema),
		}
	}

	return result
}

func requiresAuthorization(documentSecurity []map[string][]string, operationSecurity []map[string][]string) bool {
	if len(operationSecurity) > 0 {
		return securityContainsHeaderAPIKey(operationSecurity)
	}

	return securityContainsHeaderAPIKey(documentSecurity)
}

func securityContainsHeaderAPIKey(security []map[string][]string) bool {
	for _, requirement := range security {
		if _, ok := requirement["HeaderApiKey"]; ok {
			return true
		}
	}

	return false
}

func nodeToAny(node yaml.Node) any {
	if node.IsZero() {
		return nil
	}

	var value any
	if err := node.Decode(&value); err != nil {
		return nil
	}

	return normalizeYAMLValue(value)
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			result[key] = normalizeYAMLValue(value)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			result[fmt.Sprint(key)] = normalizeYAMLValue(value)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, value := range typed {
			result = append(result, normalizeYAMLValue(value))
		}
		return result
	default:
		return typed
	}
}

func firstServerURL(servers []openAPIServer) string {
	if len(servers) == 0 {
		return ""
	}

	return servers[0].URL
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

func mustObjectJSON(value any) string {
	if value == nil {
		return "{}"
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}

	if string(data) == "null" {
		return "{}"
	}

	return string(data)
}

func extractRateLimitNotes(description string) string {
	return cleanHTMLBlock(extractDescriptionBlock(description, "description_limit"))
}

func extractSubscriptionRequirements(description string) string {
	token := cleanHTMLBlock(extractDescriptionBlock(description, "description_token"))
	auth := cleanHTMLBlock(extractDescriptionBlock(description, "description_auth"))

	switch {
	case token != "" && auth != "":
		return auth + "\n" + token
	case token != "":
		return token
	default:
		return auth
	}
}

func extractDescriptionBlock(description string, marker string) string {
	start := strings.Index(description, `<div class="`+marker)
	if start == -1 {
		start = strings.Index(description, marker)
	}
	if start == -1 {
		return ""
	}

	end := strings.Index(description[start:], "</div>")
	if end == -1 {
		return strings.TrimSpace(description[start:])
	}

	return strings.TrimSpace(description[start : start+end+len("</div>")])
}

func cleanHTMLBlock(value string) string {
	value = html.UnescapeString(value)
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "\r", "\n")

	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

func requiresJam(operationID string, summary string, description string, tags []string) bool {
	operationUpper := strings.ToUpper(operationID)

	if isKnownNonJamOperation(operationUpper) {
		return false
	}

	if isKnownJamOnlyOperation(operationUpper) {
		return true
	}

	combined := strings.ToLower(operationID + " " + summary + " " + description + " " + strings.Join(tags, " "))

	if strings.Contains(combined, "получить информацию о подписке джем") {
		return false
	}

	if strings.Contains(combined, "доступны только с подпиской") && strings.Contains(combined, "джем") {
		return true
	}

	if strings.Contains(combined, "доступен только с подпиской") && strings.Contains(combined, "джем") {
		return true
	}

	if strings.Contains(combined, "только с подпиской «джем»") {
		return true
	}

	if strings.Contains(combined, "только с подпиской джем") {
		return true
	}

	return false
}

func isKnownJamOnlyOperation(operationID string) bool {
	known := map[string]bool{
		"DETAIL_HISTORY_REPORT":                 true,
		"GROUPED_HISTORY_REPORT":                true,
		"SEARCH_QUERIES_PREMIUM_REPORT_GROUP":   true,
		"SEARCH_QUERIES_PREMIUM_REPORT_PRODUCT": true,
		"SEARCH_QUERIES_PREMIUM_REPORT_TEXT":    true,
	}

	return known[operationID]
}

func isKnownNonJamOperation(operationID string) bool {
	known := map[string]bool{
		"STOCK_HISTORY_REPORT_CSV": true,
		"STOCK_HISTORY_DAILY_CSV":  true,
	}

	return known[operationID]
}
