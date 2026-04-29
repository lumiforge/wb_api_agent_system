package wb_api_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/planning"
)

var _ planning.OperationSelector = (*ADKOperationSelector)(nil)

// PURPOSE: Executes probabilistic operation selection through ADK while preserving deterministic input/output contracts.
type ADKOperationSelector struct {
	logger    *log.Logger
	adkRunner *runner.Runner
	validator *planning.OperationSelectionValidator
}

type OperationSelectorConfig struct {
	SessionService adksession.Service
	Model          model.LLM
	Logger         *log.Logger
	ModelName      string
}

func NewADKOperationSelector(cfg OperationSelectorConfig) (*ADKOperationSelector, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	instruction := buildOperationSelectorInstruction()

	selectorAgent, err := llmagent.New(llmagent.Config{
		Name:        "wb_operation_selector_agent",
		Description: "Selects Wildberries registry operations and missing business inputs.",
		Model:       cfg.Model,
		// WHY: The selector contract contains JSON braces that must not be treated as ADK state placeholders.
		InstructionProvider: func(ctx adkagent.ReadonlyContext) (string, error) {
			return instruction, nil
		},
		IncludeContents: llmagent.IncludeContentsNone,
	})
	if err != nil {
		return nil, fmt.Errorf("create wb operation selector agent: %w", err)
	}

	adkRunner, err := runner.New(runner.Config{
		AppName:           "wb_api_agent_system",
		Agent:             selectorAgent,
		SessionService:    cfg.SessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create operation selector runner: %w", err)
	}

	return &ADKOperationSelector{
		logger:    logger,
		adkRunner: adkRunner,
		validator: planning.NewOperationSelectionValidator(),
	}, nil
}

func (s *ADKOperationSelector) SelectOperations(
	ctx context.Context,
	input entities.OperationSelectionInput,
) (*entities.OperationSelectionPlan, error) {
	if err := input.ValidateShape(); err != nil {
		return nil, fmt.Errorf("invalid operation selection input: %w", err)
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal operation selection input: %w", err)
	}

	responseText, err := s.runADK(ctx, input, string(inputJSON))
	if err != nil {
		return nil, fmt.Errorf("run operation selector: %w", err)
	}

	plan, err := parseOperationSelectionPlan(responseText)
	if err != nil {
		return nil, err
	}

	if err := s.validator.Validate(input, *plan); err != nil {
		return nil, err
	}

	return plan, nil
}

func (s *ADKOperationSelector) runADK(
	ctx context.Context,
	input entities.OperationSelectionInput,
	inputJSON string,
) (string, error) {
	userID := "a2a"
	if input.Metadata != nil && strings.TrimSpace(input.Metadata.UserID) != "" {
		userID = input.Metadata.UserID
	}

	sessionID := input.RequestID
	if sessionID == "" {
		sessionID = "unknown"
	}

	message := genai.NewContentFromText(inputJSON, genai.RoleUser)

	var finalText string
	for event, err := range s.adkRunner.Run(ctx, userID, sessionID, message, adkagent.RunConfig{
		StreamingMode: adkagent.StreamingModeNone,
	}) {
		if err != nil {
			return "", err
		}

		if event == nil || event.Content == nil {
			continue
		}

		text := contentText(event.Content)
		if text == "" {
			continue
		}

		finalText = text
		if event.IsFinalResponse() {
			break
		}
	}

	if finalText == "" {
		return "", fmt.Errorf("operation selector returned empty response")
	}

	s.logger.Printf(
		"operation selector completed request_id=%s selected_response_bytes=%d",
		input.RequestID,
		len(finalText),
	)

	return finalText, nil
}

func parseOperationSelectionPlan(responseText string) (*entities.OperationSelectionPlan, error) {
	cleaned := cleanModelJSON(responseText)

	var plan entities.OperationSelectionPlan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return nil, fmt.Errorf("parse OperationSelectionPlan JSON: %w; response=%s", err, responseText)
	}

	if err := plan.ValidateShape(); err != nil {
		return nil, err
	}

	return &plan, nil
}

func buildOperationSelectorInstruction() string {
	return strings.Join([]string{
		"You are the Wildberries API operation selector.",
		"You receive exactly one OperationSelectionInput JSON object.",
		"You must return exactly one OperationSelectionPlan JSON object.",
		"Do not return markdown.",
		"Do not return explanations.",
		"Do not execute HTTP requests.",
		"Do not request, infer, expose, or return secrets.",
		"Use only operation_id values present in registry_candidates.",
		"Do not invent method, server_url, path_template, request params, headers, bodies, pagination, retry policy, or response mappings.",
		"Your responsibility is only operation selection and missing business input identification.",
		"Executable ApiExecutionPlan composition is forbidden in this layer.",
		"Aliases and semantic interpretation may help you choose among registry_candidates, but they are not source of truth.",
		"registry_candidates are the only source of truth for available operations.",
		"business_request is the source of truth for user-provided business facts.",
		"If required business data is missing, return status=\"needs_clarification\" and fill missing_inputs.",
		"If the request cannot be represented by any registry candidate, return status=\"unsupported\".",
		"If policy forbids the request, return status=\"blocked\".",
		"If selected operations are sufficient for deterministic composition, return status=\"ready_for_composition\".",
		"Never put internal field names into missing_inputs[].user_question.",
		"Internal field names may appear only in missing_inputs[].internal_fields.",
		"",
		"OperationSelectionPlan JSON shape:",
		`{
  "schema_version": "1.0",
  "request_id": "same as input.request_id",
  "marketplace": "wildberries",
  "status": "ready_for_composition | needs_clarification | unsupported | blocked",
  "user_facing_summary": "short user-facing summary when useful",
  "selected_operations": [
    {
      "operation_id": "must exactly match one registry_candidates[].operation_id",
      "purpose": "why this operation is needed",
      "depends_on": [],
      "input_strategy": "no_user_input | static_defaults | business_entities | step_output"
    }
  ],
  "missing_inputs": [
    {
      "code": "stable_business_input_code",
      "user_question": "question visible to the user without internal field names",
      "accepts": [],
      "internal_fields": []
    }
  ],
  "rejected_candidates": [
    {
      "operation_id": "must exactly match one registry_candidates[].operation_id",
      "reason": "why candidate was not selected"
    }
  ],
  "warnings": []
}`,
	}, "\n")
}
