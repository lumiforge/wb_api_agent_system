package wb_api_agent

import (
	"context"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Owns WB API planning behavior implemented through ADK agents.
type Agent struct {
	registry wbregistry.Retriever
}

func New(registry wbregistry.Retriever) *Agent {
	return &Agent{
		registry: registry,
	}
}

func (a *Agent) Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error) {
	questions := requiredQuestions(request)
	if len(questions) > 0 {
		// WHY: Boundary requests must be converted into ApiExecutionPlan instead of HTTP errors.
		return entities.NewNeedsClarificationPlan(request, questions), nil
	}

	operations, err := a.registry.SearchOperations(ctx, wbregistry.SearchQuery{
		Query:        buildRegistrySearchQuery(request),
		Limit:        effectiveMaxSteps(request.Constraints.MaxSteps),
		ReadonlyOnly: request.Constraints.ReadonlyOnly,
		ExcludeJam:   request.Constraints.NoJamSubscription,
	})
	if err != nil {
		return nil, err
	}

	if len(operations) == 0 {
		return entities.NewBlockedPlan(
			request,
			"no_registry_operations_match_request_constraints",
			[]entities.PlanWarning{
				{
					Code:    "no_matching_operations",
					Message: "No WB API registry operations match the request and constraints.",
				},
			},
		), nil
	}

	return entities.NewNeedsClarificationPlan(request, []string{
		"Planner found candidate WB API operations, but final step composition is not implemented yet.",
	}), nil
}

func requiredQuestions(request entities.BusinessRequest) []string {
	questions := make([]string, 0)

	if strings.TrimSpace(request.RequestID) == "" {
		questions = append(questions, "Provide request_id.")
	}

	if strings.TrimSpace(request.Marketplace) == "" {
		questions = append(questions, "Provide marketplace.")
	}

	if request.Marketplace != "" && request.Marketplace != "wildberries" {
		questions = append(questions, "Only marketplace=wildberries is supported.")
	}

	if strings.TrimSpace(request.Intent) == "" {
		questions = append(questions, "Provide intent.")
	}

	if strings.TrimSpace(request.NaturalLanguageRequest) == "" {
		questions = append(questions, "Provide natural_language_request.")
	}

	return questions
}

func buildRegistrySearchQuery(request entities.BusinessRequest) string {
	parts := []string{
		request.Intent,
		request.NaturalLanguageRequest,
	}

	for key, value := range request.Entities {
		parts = append(parts, key, stringifyEntityValue(value))
	}

	return strings.Join(parts, " ")
}

func stringifyEntityValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return ""
	case int:
		return ""
	default:
		return ""
	}
}

func effectiveMaxSteps(maxSteps int) int {
	if maxSteps <= 0 {
		return 10
	}

	if maxSteps > 20 {
		return 20
	}

	return maxSteps
}
