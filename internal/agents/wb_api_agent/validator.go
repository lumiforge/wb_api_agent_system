package wb_api_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Validates registry-backed executable plans and separates blocking contract errors from recoverable clarification needs.
func (p *PlanPostProcessor) validatePlan(
	ctx context.Context,
	request entities.BusinessRequest,
	plan *entities.ApiExecutionPlan,
) (*entities.ApiExecutionPlan, error) {
	if plan.Status == "needs_clarification" {
		if len(plan.ClarifyingQuestions) == 0 {
			return entities.NewBlockedPlan(request, "plan_returned_empty_clarification", []entities.PlanWarning{
				{
					Code:    "invalid_plan_contract",
					Message: "Plan returned status=needs_clarification without clarifying_questions.",
				},
			}), nil
		}

		return nil, nil
	}

	if plan.Status == "blocked" {
		if plan.BlockReason == "" {
			return entities.NewBlockedPlan(request, "plan_returned_empty_block_reason", []entities.PlanWarning{
				{
					Code:    "invalid_plan_contract",
					Message: "Plan returned status=blocked without block_reason.",
				},
			}), nil
		}

		return nil, nil
	}

	if plan.Status != "ready" {
		return entities.NewBlockedPlan(request, "plan_returned_unknown_status", []entities.PlanWarning{
			{
				Code:    "invalid_plan_contract",
				Message: "Plan returned unknown status: " + plan.Status,
			},
		}), nil
	}

	clarifyingQuestions := make([]string, 0)
	blockErrors := make([]string, 0)

	blockErrors = append(blockErrors, validateReadyPlanShape(request, *plan)...)

	for stepIndex := range plan.Steps {
		step := &plan.Steps[stepIndex]

		operation, err := p.registry.GetOperation(ctx, step.OperationID)
		if err != nil {
			return nil, err
		}

		if operation == nil {
			blockErrors = append(blockErrors, fmt.Sprintf("step %s uses unknown operation_id %s", step.StepID, step.OperationID))
			continue
		}

		blockErrors = append(blockErrors, validateStepShape(*step)...)
		blockErrors = append(blockErrors, validateRegistryIdentity(*step, *operation)...)
		blockErrors = append(blockErrors, validateStepPolicies(request, *step, *operation)...)

		pathParams := schemaParamNames(operation.PathParamsSchemaJSON)
		addPathTemplateParams(pathParams, operation.PathTemplate)

		queryParams := schemaParamNames(operation.QueryParamsSchemaJSON)

		blockErrors = append(blockErrors, validateRequiredParams(step.StepID, "path_params", step.Request.PathParams, pathParams)...)
		blockErrors = append(blockErrors, validateRequiredParams(step.StepID, "query_params", step.Request.QueryParams, queryParams)...)

		blockErrors = append(blockErrors, validateUnknownValueBindings(step.StepID, "path_params", step.Request.PathParams, pathParams)...)
		blockErrors = append(blockErrors, validateUnknownValueBindings(step.StepID, "query_params", step.Request.QueryParams, queryParams)...)

		pathBindingValidation := validateValueBindings(step.StepID, "path_params", step.Request.PathParams, plan.Inputs)
		blockErrors = append(blockErrors, pathBindingValidation.BlockErrors...)
		clarifyingQuestions = append(clarifyingQuestions, pathBindingValidation.ClarifyingQuestions...)

		queryBindingValidation := validateValueBindings(step.StepID, "query_params", step.Request.QueryParams, plan.Inputs)
		blockErrors = append(blockErrors, queryBindingValidation.BlockErrors...)
		clarifyingQuestions = append(clarifyingQuestions, queryBindingValidation.ClarifyingQuestions...)

		bodyFields := requestBodyFieldNames(operation.RequestBodySchemaJSON)
		blockErrors = append(blockErrors, validateUnknownRequestBodyFields(step.StepID, bodyFields, step.Request.Body)...)

		bodyBindingValidation := validateRequestBodyBindings(step.StepID, step.Request.Body, plan.Inputs)
		blockErrors = append(blockErrors, bodyBindingValidation.BlockErrors...)
		clarifyingQuestions = append(clarifyingQuestions, bodyBindingValidation.ClarifyingQuestions...)

		clarifyingQuestions = append(
			clarifyingQuestions,
			validateRequiredRequestBody(step.StepID, operation.RequestBodySchemaJSON, step.Request.Body, plan.Inputs)...,
		)

		if len(step.ResponseMapping.Outputs) == 0 || hasOnlyRawOutput(*step) {
			// WHY: Registry-backed response mapping is normal post-processing, not a client-visible warning.
			applyResponseMappingDefaults(step, *operation)
		}

		blockErrors = append(blockErrors, validateResponseMapping(step.StepID, step.ResponseMapping)...)
	}

	if len(clarifyingQuestions) > 0 {
		return entities.NewRegistryValidatedNeedsClarificationPlan(request, dedupeStrings(clarifyingQuestions), plan.Warnings), nil
	}

	// WHY: Final output fields can be derived from normalized step response mappings as normal post-processing.
	normalizeFinalOutput(plan)

	blockErrors = append(blockErrors, validateFinalOutput(*plan)...)

	if len(blockErrors) > 0 {
		return entities.NewBlockedPlan(request, "plan_failed_registry_validation", append(plan.Warnings, entities.PlanWarning{
			Code:    "registry_validation_errors",
			Message: strings.Join(blockErrors, "; "),
		})), nil
	}

	plan.Validation.RegistryChecked = true
	plan.Validation.OutputSchemaChecked = true
	plan.Validation.ReadonlyPolicyChecked = request.Constraints.ReadonlyOnly
	plan.Validation.SecretsPolicyChecked = true
	plan.Validation.JamPolicyChecked = request.Constraints.NoJamSubscription
	if plan.Validation.Errors == nil {
		plan.Validation.Errors = []string{}
	}

	return nil, nil
}

func validateReadyPlanShape(request entities.BusinessRequest, plan entities.ApiExecutionPlan) []string {
	errors := make([]string, 0)

	if plan.SchemaVersion != "1.0" {
		errors = append(errors, "schema_version must be 1.0")
	}
	if plan.RequestID == "" {
		errors = append(errors, "request_id is empty")
	}
	if plan.Marketplace != "wildberries" {
		errors = append(errors, "marketplace must be wildberries")
	}
	if plan.RiskLevel == "" {
		errors = append(errors, "risk_level is empty")
	}
	if plan.ExecutionMode == "" {
		errors = append(errors, "execution_mode is empty")
	}
	if plan.Inputs == nil {
		// WHY: Empty input maps are valid for defaulted operations; nil breaks deterministic binding validation.
		errors = append(errors, "inputs must be an object")
	}
	if len(plan.Steps) == 0 {
		errors = append(errors, "ready plan must contain at least one step")
	}
	if request.Constraints.MaxSteps > 0 && len(plan.Steps) > request.Constraints.MaxSteps {
		errors = append(errors, fmt.Sprintf("plan has %d steps, max_steps is %d", len(plan.Steps), request.Constraints.MaxSteps))
	}
	if plan.Transforms == nil {
		errors = append(errors, "transforms must be an array")
	}
	if plan.Warnings == nil {
		errors = append(errors, "warnings must be an array")
	}
	if plan.Validation.Errors == nil {
		errors = append(errors, "validation.errors must be an array")
	}

	return errors
}

func validateStepShape(step entities.ApiPlanStep) []string {
	errors := make([]string, 0)

	if step.StepID == "" {
		errors = append(errors, "step_id is empty")
	}
	if step.OperationID == "" {
		errors = append(errors, fmt.Sprintf("step %s operation_id is empty", step.StepID))
	}
	if step.SourceFile == "" {
		errors = append(errors, fmt.Sprintf("step %s source_file is empty", step.StepID))
	}
	if step.RiskLevel == "" {
		errors = append(errors, fmt.Sprintf("step %s risk_level is empty", step.StepID))
	}
	if step.Purpose == "" {
		errors = append(errors, fmt.Sprintf("step %s purpose is empty", step.StepID))
	}
	if step.DependsOn == nil {
		errors = append(errors, fmt.Sprintf("step %s depends_on must be an array", step.StepID))
	}
	if step.Request.ServerURL == "" {
		errors = append(errors, fmt.Sprintf("step %s request.server_url is empty", step.StepID))
	}
	if step.Request.Method == "" {
		errors = append(errors, fmt.Sprintf("step %s request.method is empty", step.StepID))
	}
	if step.Request.PathTemplate == "" {
		errors = append(errors, fmt.Sprintf("step %s request.path_template is empty", step.StepID))
	}
	if step.Request.PathParams == nil {
		errors = append(errors, fmt.Sprintf("step %s request.path_params must be an object", step.StepID))
	}
	if step.Request.QueryParams == nil {
		errors = append(errors, fmt.Sprintf("step %s request.query_params must be an object", step.StepID))
	}
	if step.Request.Headers == nil {
		errors = append(errors, fmt.Sprintf("step %s request.headers must be an object", step.StepID))
	}
	if step.Request.Body == nil {
		errors = append(errors, fmt.Sprintf("step %s request.body must not be null", step.StepID))
	}
	if step.Request.ContentType == "" {
		errors = append(errors, fmt.Sprintf("step %s request.content_type is empty", step.StepID))
	}
	if step.Request.Accept == "" {
		errors = append(errors, fmt.Sprintf("step %s request.accept is empty", step.StepID))
	}

	return errors
}

func validateRegistryIdentity(step entities.ApiPlanStep, operation entities.WBRegistryOperation) []string {
	errors := make([]string, 0)

	if step.SourceFile != operation.SourceFile {
		errors = append(errors, fmt.Sprintf("step %s source_file mismatch: got %s, registry has %s", step.StepID, step.SourceFile, operation.SourceFile))
	}
	if step.Request.Method != operation.Method {
		errors = append(errors, fmt.Sprintf("step %s method mismatch: got %s, registry has %s", step.StepID, step.Request.Method, operation.Method))
	}
	if step.Request.ServerURL != operation.ServerURL {
		errors = append(errors, fmt.Sprintf("step %s server_url mismatch: got %s, registry has %s", step.StepID, step.Request.ServerURL, operation.ServerURL))
	}
	if step.Request.PathTemplate != operation.PathTemplate {
		errors = append(errors, fmt.Sprintf("step %s path_template mismatch: got %s, registry has %s", step.StepID, step.Request.PathTemplate, operation.PathTemplate))
	}

	return errors
}

func validateStepPolicies(
	request entities.BusinessRequest,
	step entities.ApiPlanStep,
	operation entities.WBRegistryOperation,
) []string {
	errors := make([]string, 0)

	if operation.XReadonlyMethod == nil {
		errors = append(errors, fmt.Sprintf("step %s registry readonly flag is unknown", step.StepID))
	} else if step.Readonly != *operation.XReadonlyMethod {
		errors = append(errors, fmt.Sprintf("step %s readonly mismatch with registry", step.StepID))
	}

	if request.Constraints.ReadonlyOnly && !step.Readonly {
		errors = append(errors, fmt.Sprintf("step %s violates readonly_only policy", step.StepID))
	}

	if request.Constraints.NoJamSubscription && operation.RequiresJam {
		errors = append(errors, fmt.Sprintf("step %s violates no_jam_subscription policy", step.StepID))
	}

	auth, ok := step.Request.Headers["Authorization"]
	if !ok {
		errors = append(errors, fmt.Sprintf("step %s missing Authorization header binding", step.StepID))
	} else if auth.Source != "executor_secret" || auth.SecretName != "WB_AUTHORIZATION" || !auth.Required {
		errors = append(errors, fmt.Sprintf("step %s Authorization must use required executor_secret WB_AUTHORIZATION", step.StepID))
	}

	for headerName, header := range step.Request.Headers {
		if strings.EqualFold(headerName, "Authorization") {
			continue
		}

		if strings.Contains(strings.ToLower(header.Value), "bearer ") || strings.Contains(strings.ToLower(header.Value), "token") {
			errors = append(errors, fmt.Sprintf("step %s header %s appears to contain a secret literal", step.StepID, headerName))
		}
	}

	return errors
}

func validateRequiredParams(
	stepID string,
	fieldName string,
	actual map[string]entities.ValueBinding,
	allowed map[string]bool,
) []string {
	errors := make([]string, 0)

	for name, required := range allowed {
		if !required {
			continue
		}

		if _, ok := actual[name]; !ok {
			errors = append(errors, fmt.Sprintf("step %s missing required %s.%s", stepID, fieldName, name))
		}
	}

	return errors
}

func validateUnknownValueBindings(
	stepID string,
	fieldName string,
	actual map[string]entities.ValueBinding,
	allowed map[string]bool,
) []string {
	errors := make([]string, 0)

	for name := range actual {
		if _, ok := allowed[name]; ok {
			continue
		}

		errors = append(errors, fmt.Sprintf("step %s has %s not present in registry: %s", stepID, fieldName, name))
	}

	return errors
}

// PURPOSE: Carries validator outcomes without mixing invalid plan contracts with missing user-provided business data.
type bindingValidationResult struct {
	BlockErrors         []string
	ClarifyingQuestions []string
}

func validateValueBindings(
	stepID string,
	fieldName string,
	values map[string]entities.ValueBinding,
	inputs map[string]entities.InputValue,
) bindingValidationResult {
	result := newBindingValidationResult()

	for name, binding := range values {
		bindingResult := validateValueBinding(stepID, fieldName+"."+name, binding, inputs)
		result.BlockErrors = append(result.BlockErrors, bindingResult.BlockErrors...)
		result.ClarifyingQuestions = append(result.ClarifyingQuestions, bindingResult.ClarifyingQuestions...)
	}

	return result
}

func validateValueBinding(
	stepID string,
	name string,
	binding entities.ValueBinding,
	inputs map[string]entities.InputValue,
) bindingValidationResult {
	result := newBindingValidationResult()

	switch binding.Source {
	case "input":
		if binding.InputName == "" {
			result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s has source=input but empty input_name", stepID, name))
			return result
		}

		input, ok := inputs[binding.InputName]
		if !ok {
			if binding.Required {
				// WHY: Missing required source=input values are recoverable by asking the user for business data.
				result.ClarifyingQuestions = append(result.ClarifyingQuestions, missingInputQuestion(binding.InputName))
				return result
			}

			result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s references missing optional input %s", stepID, name, binding.InputName))
			return result
		}

		if binding.Required && isEmptyInputValue(input.Value) {
			// WHY: Empty required business inputs are recoverable and must not be treated as registry validation failures.
			result.ClarifyingQuestions = append(result.ClarifyingQuestions, missingInputQuestion(binding.InputName))
		}
	case "static":
		if binding.Required && isEmptyInputValue(binding.Value) {
			result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s has empty required static value", stepID, name))
		}
	case "step_output":
		if binding.StepID == "" || binding.OutputName == "" {
			result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s has invalid step_output binding", stepID, name))
		}
	case "executor_secret":
		if binding.SecretName == "" {
			result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s has empty secret_name", stepID, name))
		}
	default:
		result.BlockErrors = append(result.BlockErrors, fmt.Sprintf("step %s binding %s has unsupported source %s", stepID, name, binding.Source))
	}

	return result
}

func newBindingValidationResult() bindingValidationResult {
	return bindingValidationResult{
		BlockErrors:         []string{},
		ClarifyingQuestions: []string{},
	}
}

func validateUnknownRequestBodyFields(stepID string, allowed map[string]bool, body any) []string {
	if len(allowed) == 0 {
		return []string{}
	}

	bodyMap, ok := body.(map[string]any)
	if !ok {
		return []string{}
	}

	errors := make([]string, 0)

	for name := range bodyMap {
		if _, ok := allowed[name]; ok {
			continue
		}

		errors = append(errors, fmt.Sprintf("step %s request body field is not present in registry: %s", stepID, name))
	}

	return errors
}

func validateRequestBodyBindings(
	stepID string,
	body any,
	inputs map[string]entities.InputValue,
) bindingValidationResult {
	result := newBindingValidationResult()

	bodyMap, ok := body.(map[string]any)
	if !ok {
		return result
	}

	for name, value := range bodyMap {
		binding, ok := bodyValueBinding(value)
		if !ok {
			continue
		}

		bindingResult := validateValueBinding(stepID, "body."+name, binding, inputs)
		result.BlockErrors = append(result.BlockErrors, bindingResult.BlockErrors...)
		result.ClarifyingQuestions = append(result.ClarifyingQuestions, bindingResult.ClarifyingQuestions...)
	}

	return result
}

func validateRequiredRequestBody(
	stepID string,
	requestBodySchemaJSON string,
	body any,
	inputs map[string]entities.InputValue,
) []string {
	requiredFields := requiredRequestBodyFields(requestBodySchemaJSON)
	if len(requiredFields) == 0 {
		return []string{}
	}

	bodyMap, ok := body.(map[string]any)
	if !ok {
		return []string{missingRequestBodyFieldsQuestion(requiredFields)}
	}

	questions := make([]string, 0)

	for _, field := range requiredFields {
		value, ok := bodyMap[field]
		if !ok || isEmptyBodyValue(value, inputs) {
			// WHY: User-facing clarification must not expose internal normalized entity names.
			questions = append(questions, missingRequestBodyFieldQuestion(field))
		}
	}

	return questions
}

func validateResponseMapping(stepID string, mapping entities.ResponseMapping) []string {
	errors := make([]string, 0)

	if len(mapping.Outputs) == 0 {
		errors = append(errors, fmt.Sprintf("step %s response_mapping.outputs is empty", stepID))
	}

	for outputName, output := range mapping.Outputs {
		if outputName == "" {
			errors = append(errors, fmt.Sprintf("step %s response_mapping output name is empty", stepID))
		}
		if output.Type == "" {
			errors = append(errors, fmt.Sprintf("step %s response_mapping output %s type is empty", stepID, outputName))
		}
		if output.Path == "" {
			errors = append(errors, fmt.Sprintf("step %s response_mapping output %s path is empty", stepID, outputName))
		}
	}

	return errors
}

func validateFinalOutput(plan entities.ApiExecutionPlan) []string {
	errors := make([]string, 0)

	if plan.FinalOutput.Type == "" {
		errors = append(errors, "final_output.type is empty")
	}
	if plan.FinalOutput.Description == "" {
		errors = append(errors, "final_output.description is empty")
	}
	if len(plan.FinalOutput.Fields) == 0 {
		errors = append(errors, "final_output.fields is empty")
	}

	availableOutputs := make(map[string]bool)
	for _, step := range plan.Steps {
		for outputName := range step.ResponseMapping.Outputs {
			availableOutputs["steps."+step.StepID+".outputs."+outputName] = true
		}
	}

	for fieldName, rawRef := range plan.FinalOutput.Fields {
		ref, ok := rawRef.(string)
		if !ok {
			continue
		}

		if strings.HasPrefix(ref, "steps.") && !availableOutputs[ref] {
			errors = append(errors, fmt.Sprintf("final_output.fields.%s references missing step output %s", fieldName, ref))
		}
	}

	return errors
}

func bodyValueBinding(value any) (entities.ValueBinding, bool) {
	switch typed := value.(type) {
	case entities.ValueBinding:
		return typed, true
	case map[string]any:
		if _, ok := typed["source"].(string); !ok {
			return entities.ValueBinding{}, false
		}

		return valueBindingFromMap(typed), true
	default:
		return entities.ValueBinding{}, false
	}
}

func isEmptyBodyValue(value any, inputs map[string]entities.InputValue) bool {
	binding, ok := bodyValueBinding(value)
	if ok {
		switch binding.Source {
		case "input":
			input, exists := inputs[binding.InputName]
			if !exists {
				return true
			}

			return isEmptyInputValue(input.Value)
		case "static":
			return isEmptyInputValue(binding.Value)
		default:
			return false
		}
	}

	return isEmptyPlanValue(value)
}

func schemaParamNames(schemaJSON string) map[string]bool {
	result := make(map[string]bool)

	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return result
	}

	for name, rawValue := range schema {
		required := false

		if valueMap, ok := rawValue.(map[string]any); ok {
			required, _ = valueMap["required"].(bool)
		}

		result[name] = required
	}

	return result
}

func requestBodyFieldNames(schemaJSON string) map[string]bool {
	result := make(map[string]bool)

	schema := requestBodySchema(schemaJSON)
	if schema == nil {
		return result
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return result
	}

	required := requiredNames(schema)

	for name := range properties {
		result[name] = required[name]
	}

	return result
}

func requiredRequestBodyFields(schemaJSON string) []string {
	schema := requestBodySchema(schemaJSON)
	if schema == nil {
		return []string{}
	}

	requiredRaw, ok := schema["required"].([]any)
	if !ok {
		return []string{}
	}

	required := make([]string, 0, len(requiredRaw))
	for _, item := range requiredRaw {
		name, ok := item.(string)
		if ok && name != "" {
			required = append(required, name)
		}
	}

	return required
}

func requestBodySchema(schemaJSON string) map[string]any {
	var root map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &root); err != nil {
		return nil
	}

	content, ok := root["content"].(map[string]any)
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

	return schema
}

func requiredNames(schema map[string]any) map[string]bool {
	result := make(map[string]bool)

	rawRequired, ok := schema["required"].([]any)
	if !ok {
		return result
	}

	for _, item := range rawRequired {
		name, ok := item.(string)
		if ok && name != "" {
			result[name] = true
		}
	}

	return result
}

func isEmptyPlanValue(value any) bool {
	if value == nil {
		return true
	}

	switch typed := value.(type) {
	case entities.ValueBinding:
		if typed.Source != "" {
			return false
		}

		return typed.Value == nil
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []int:
		return len(typed) == 0
	case map[string]any:
		if source, ok := typed["source"].(string); ok && source != "" {
			return false
		}

		return len(typed) == 0
	default:
		return false
	}
}

func addPathTemplateParams(params map[string]bool, pathTemplate string) {
	for _, name := range pathTemplateParamNames(pathTemplate) {
		if _, ok := params[name]; !ok {
			params[name] = true
		}
	}
}

func pathTemplateParamNames(pathTemplate string) []string {
	names := make([]string, 0)

	for {
		start := strings.Index(pathTemplate, "{")
		if start == -1 {
			break
		}

		end := strings.Index(pathTemplate[start:], "}")
		if end == -1 {
			break
		}

		name := strings.TrimSpace(pathTemplate[start+1 : start+end])
		if name != "" {
			names = append(names, name)
		}

		pathTemplate = pathTemplate[start+end+1:]
	}

	return names
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}

		seen[value] = true
		result = append(result, value)
	}

	return result
}
