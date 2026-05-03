package wb_api_agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"

	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/composer"
	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/orchestration"
	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/selector"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

// PURPOSE: Holds runtime dependencies required to construct the WB API planning agent.
type Config struct {
	Registry       wbregistry.Retriever
	SessionService adksession.Service
	Model          model.LLM
	Logger         *log.Logger

	ModelName string
}

// PURPOSE: Orchestrates WB API planning through retrieval, bounded operation selection, deterministic composition, and validation.
type Agent struct {
	registry                   wbregistry.Retriever
	operationSelector          planning.OperationSelector
	operationSelectionResolver *orchestration.OperationSelectionRegistryResolver
	apiPlanComposer            planning.ApiPlanComposer
	postProcessor              *orchestration.PlanPostProcessor
	logger                     *log.Logger
}

func New(cfg Config) (*Agent, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	operationSelector, err := selector.NewADKOperationSelector(selector.OperationSelectorConfig{
		SessionService: cfg.SessionService,
		Model:          cfg.Model,
		Logger:         logger,
		ModelName:      cfg.ModelName,
	})
	if err != nil {
		return nil, fmt.Errorf("create operation selector: %w", err)
	}

	return &Agent{
		registry:                   cfg.Registry,
		operationSelector:          operationSelector,
		operationSelectionResolver: orchestration.NewOperationSelectionRegistryResolver(),
		apiPlanComposer:            composer.NewRegistryApiPlanComposer(),
		postProcessor:              orchestration.NewPlanPostProcessor(cfg.Registry),
		logger:                     logger,
	}, nil
}

func (a *Agent) Plan(ctx context.Context, request entities.BusinessRequest) (*entities.ApiExecutionPlan, error) {
	request.NormalizeCorrelationIdentifiers()

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

	// WHY: Non-deterministic requests now use bounded ADK operation selection followed by deterministic registry-backed composition.
	a.logger.Printf("selector composer pipeline started request_id=%s correlation_id=%s session_id=%s run_id=%s tool_call_id=%s client_execution_id=%s",
		request.RequestID,
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.CorrelationID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.SessionID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.RunID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ToolCallID }),
		metadataValue(request.Metadata, func(m *entities.RequestMetadata) string { return m.ClientExecutionID }),
	)
	return a.planWithSelectorComposer(ctx, request, operations)
}

func (a *Agent) planWithSelectorComposer(
	ctx context.Context,
	request entities.BusinessRequest,
	candidates []entities.WBRegistryOperation,
) (*entities.ApiExecutionPlan, error) {
	selectionInput := entities.NewOperationSelectionInput(request, candidates)

	selectionPlan, err := a.operationSelector.SelectOperations(ctx, selectionInput)
	if err != nil {
		return entities.NewBlockedPlan(request, "operation_selector_failed", []entities.PlanWarning{
			{
				Code:    "operation_selector_error",
				Message: err.Error(),
			},
		}), nil
	}

	switch selectionPlan.Status {
	case entities.OperationSelectionStatusNeedsClarification:
		return entities.NewRegistryValidatedNeedsClarificationPlan(
			request,
			missingBusinessInputQuestions(selectionPlan.MissingInputs),
			selectionPlan.Warnings,
		), nil

	case entities.OperationSelectionStatusUnsupported:
		return entities.NewBlockedPlan(request, "operation_selection_unsupported", append(selectionPlan.Warnings, entities.PlanWarning{
			Code:    "operation_selection_unsupported",
			Message: selectionPlan.UserFacingSummary,
		})), nil

	case entities.OperationSelectionStatusBlocked:
		return entities.NewBlockedPlan(request, "operation_selection_blocked", append(selectionPlan.Warnings, entities.PlanWarning{
			Code:    "operation_selection_blocked",
			Message: selectionPlan.UserFacingSummary,
		})), nil

	case entities.OperationSelectionStatusReadyForComposition:
		// Continue below.

	default:
		return entities.NewBlockedPlan(request, "operation_selector_returned_unknown_status", append(selectionPlan.Warnings, entities.PlanWarning{
			Code:    "operation_selector_invalid_status",
			Message: string(selectionPlan.Status),
		})), nil
	}
	if request.Period == nil && selectionPlan.ResolvedInputs.Period != nil {
		// WHY: Tool-resolved temporal facts must enter deterministic composition through the existing BusinessRequest period contract.
		request.Period = selectionPlan.ResolvedInputs.Period
	}
	selectedRegistryOperations, err := a.operationSelectionResolver.Resolve(*selectionPlan, candidates)
	if err != nil {
		return entities.NewBlockedPlan(request, "operation_selection_registry_resolution_failed", append(selectionPlan.Warnings, entities.PlanWarning{
			Code:    "operation_selection_registry_resolution_error",
			Message: err.Error(),
		})), nil
	}

	compositionInput := entities.NewApiPlanCompositionInput(
		request,
		*selectionPlan,
		selectedRegistryOperations,
	)

	plan, err := a.apiPlanComposer.Compose(ctx, compositionInput)
	if err != nil {
		return entities.NewBlockedPlan(request, "api_plan_composition_failed", append(selectionPlan.Warnings, entities.PlanWarning{
			Code:    "api_plan_composition_error",
			Message: err.Error(),
		})), nil
	}

	validationPlan, err := a.postProcessor.Process(ctx, request, plan)
	if err != nil {
		return nil, err
	}

	if validationPlan != nil {
		return validationPlan, nil
	}

	return plan, nil
}

func missingBusinessInputQuestions(inputs []entities.MissingBusinessInput) []string {
	questions := make([]string, 0, len(inputs))

	for _, input := range inputs {
		if input.UserQuestion == "" {
			continue
		}

		questions = append(questions, input.UserQuestion)
	}

	return questions
}

func metadataValue(metadata *entities.RequestMetadata, selector func(*entities.RequestMetadata) string) string {
	if metadata == nil {
		return ""
	}

	return selector(metadata)
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

	if strings.TrimSpace(request.NaturalLanguageRequest) == "" {
		questions = append(questions, "Provide natural_language_request.")
	}

	return questions
}

func buildRegistrySearchQuery(request entities.BusinessRequest) string {
	parts := make([]string, 0, 2+len(request.Entities)*2)
	if strings.TrimSpace(request.Intent) != "" {
		parts = append(parts, request.Intent)
	}

	parts = append(parts, request.NaturalLanguageRequest)

	for key, value := range request.Entities {
		parts = append(parts, key, stringifyEntityValue(value))
	}

	return strings.Join(parts, " ")
}

func stringifyEntityValue(value any) string {
	return fmt.Sprint(value)
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
