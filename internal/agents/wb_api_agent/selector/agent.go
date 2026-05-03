package selector

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

	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent/tools"
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

	selectorTools, err := tools.NewOperationSelectorTools()
	if err != nil {
		return nil, fmt.Errorf("create operation selector tools: %w", err)
	}

	selectorAgent, err := llmagent.New(llmagent.Config{
		Name:        "wb_operation_selector_agent",
		Description: "Selects Wildberries registry operations and missing business inputs.",
		Model:       cfg.Model,
		// WHY: The selector contract contains JSON braces that must not be treated as ADK state placeholders.
		InstructionProvider: func(ctx adkagent.ReadonlyContext) (string, error) {
			return instruction, nil
		},
		// WHY: Relative temporal expressions require runtime facts and deterministic date arithmetic outside model memory.
		Tools:           selectorTools,
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

func cleanModelJSON(responseText string) string {
	cleaned := strings.TrimSpace(responseText)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part.Text == "" {
			continue
		}
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "\n")
}
