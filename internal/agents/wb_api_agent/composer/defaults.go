package composer

import (
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
)

// PURPOSE: Provides deterministic defaults used while composing registry-backed executable plans.
func executionMode(request entities.BusinessRequest) string {
	if strings.TrimSpace(request.Constraints.ExecutionMode) != "" {
		return request.Constraints.ExecutionMode
	}

	return "automatic"
}

func naturalLanguageSummary(input entities.ApiPlanCompositionInput) string {
	if strings.TrimSpace(input.SelectionPlan.UserFacingSummary) != "" {
		return input.SelectionPlan.UserFacingSummary
	}

	if strings.TrimSpace(input.BusinessRequest.NaturalLanguageRequest) != "" {
		return input.BusinessRequest.NaturalLanguageRequest
	}

	return "Planned Wildberries API operation."
}

func defaultRateLimitPolicy(operationID string) entities.RateLimitPolicy {
	return entities.RateLimitPolicy{
		Enabled:       true,
		Bucket:        operationID,
		MaxRequests:   1,
		PeriodSeconds: 60,
		MinIntervalMS: 1000,
	}
}

func stableStepID(operationID string) string {
	normalized := strings.TrimSpace(operationID)
	if normalized == "" {
		return "api_step"
	}

	normalized = strings.ToLower(normalized)
	var builder strings.Builder
	lastWasUnderscore := false

	for _, r := range normalized {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAllowed {
			builder.WriteRune(r)
			lastWasUnderscore = false
			continue
		}

		if !lastWasUnderscore {
			builder.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	stepID := strings.Trim(builder.String(), "_")
	if stepID == "" {
		return "api_step"
	}

	return stepID
}

func defaultRetryPolicy() entities.RetryPolicy {
	return entities.RetryPolicy{
		Enabled:       true,
		MaxAttempts:   3,
		RetryOnStatus: []int{429, 500, 502, 503, 504},
		Backoff: entities.BackoffPolicy{
			Type:           "exponential",
			InitialDelayMS: 1000,
			MaxDelayMS:     20000,
		},
	}
}
