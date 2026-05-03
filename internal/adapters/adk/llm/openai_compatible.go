package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/authctx"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// PURPOSE: Adapts an OpenAI-compatible chat completion endpoint to ADK model.LLM.
type OpenAICompatibleModel struct {
	modelName string
	baseURL   string
	apiKey    string
	client    *http.Client
}

type chatCompletionRequest struct {
	Model          string                     `json:"model"`
	Messages       []chatCompletionMessage    `json:"messages"`
	Temperature    float64                    `json:"temperature"`
	ResponseFormat *chatCompletionResponseFmt `json:"response_format,omitempty"`
	Tools          []chatCompletionTool       `json:"tools,omitempty"`
}

type chatCompletionResponseFmt struct {
	Type string `json:"type"`
}

type chatCompletionMessage struct {
	Role       string                   `json:"role"`
	Content    string                   `json:"content,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	ToolCalls  []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionTool struct {
	Type     string                     `json:"type"`
	Function chatCompletionToolFunction `json:"function"`
}

type chatCompletionToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatCompletionToolCall struct {
	ID       string                         `json:"id,omitempty"`
	Type     string                         `json:"type"`
	Function chatCompletionToolFunctionCall `json:"function"`
}

type chatCompletionToolFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
}

func NewOpenAICompatibleModel(modelName string, baseURL string, apiKey string) *OpenAICompatibleModel {
	return &OpenAICompatibleModel{
		modelName: strings.TrimSpace(modelName),
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:    strings.TrimSpace(apiKey),
		client:    http.DefaultClient,
	}
}

func (m *OpenAICompatibleModel) Name() string {
	return m.modelName
}

func (m *OpenAICompatibleModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		responseContent, err := m.generate(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}

		yield(&model.LLMResponse{
			Content: responseContent,
		}, nil)
	}
}

func (m *OpenAICompatibleModel) generate(ctx context.Context, req *model.LLMRequest) (*genai.Content, error) {
	if m == nil {
		return nil, fmt.Errorf("openai compatible model is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("llm request is nil")
	}
	if m.baseURL == "" {
		return nil, fmt.Errorf("model proxy base url is required")
	}

	messages := make([]chatCompletionMessage, 0, len(req.Contents)+1)

	if req.Config != nil && req.Config.SystemInstruction != nil {
		systemText := contentText(req.Config.SystemInstruction)
		if systemText != "" {
			messages = append(messages, chatCompletionMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	for _, content := range req.Contents {
		if content == nil {
			continue
		}

		role := "user"
		if content.Role == string(genai.RoleModel) {
			role = "assistant"
		}

		var (
			hasFunctionResponse bool
			functionCalls       []chatCompletionToolCall
		)
		for _, part := range content.Parts {
			if part == nil {
				continue
			}
			if part.FunctionResponse != nil {
				hasFunctionResponse = true
				msg, err := functionResponseMessage(part.FunctionResponse)
				if err != nil {
					return nil, err
				}
				messages = append(messages, msg)
				continue
			}
			if part.FunctionCall != nil {
				if content.Role != string(genai.RoleModel) {
					return nil, fmt.Errorf("function call content must have model role")
				}
				fc, err := functionCallToToolCall(part.FunctionCall)
				if err != nil {
					return nil, err
				}
				functionCalls = append(functionCalls, fc)
			}
		}
		if hasFunctionResponse {
			continue
		}
		text := contentText(content)
		if len(functionCalls) > 0 {
			messages = append(messages, chatCompletionMessage{
				Role:      role,
				Content:   text,
				ToolCalls: functionCalls,
			})
			continue
		}
		if text == "" {
			continue
		}

		messages = append(messages, chatCompletionMessage{
			Role:    role,
			Content: text,
		})
	}

	requestModel := strings.TrimSpace(req.Model)
	if requestModel == "" {
		requestModel = m.modelName
	}
	if requestModel == "" {
		return nil, fmt.Errorf("model name is required")
	}
	tools := mapTools(req)
	responseFormat := &chatCompletionResponseFmt{Type: "json_object"}
	if len(tools) > 0 {
		responseFormat = nil
	}

	body := chatCompletionRequest{
		Model:          requestModel,
		Messages:       messages,
		Temperature:    0,
		ResponseFormat: responseFormat,
		Tools:          tools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create chat completion request: %w", err)
	}

	authHeader, err := m.authorizationHeader(ctx)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send chat completion request: %w", err)
	}
	defer httpResp.Body.Close()

	responsePayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read chat completion response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat completion failed: status=%d body=%s", httpResp.StatusCode, string(responsePayload))
	}

	var response chatCompletionResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, fmt.Errorf("parse chat completion response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("chat completion response has no choices")
	}
	return responseMessageToContent(response.Choices[0].Message), nil
}

func responseMessageToContent(message chatCompletionMessage) *genai.Content {
	content := &genai.Content{Role: string(genai.RoleModel)}
	for _, toolCall := range message.ToolCalls {
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   strings.TrimSpace(toolCall.ID),
				Name: strings.TrimSpace(toolCall.Function.Name),
				Args: parseToolCallArgs(toolCall.Function.Arguments),
			},
		})
	}
	if text := strings.TrimSpace(message.Content); text != "" {
		content.Parts = append(content.Parts, genai.NewPartFromText(text))
	}
	return content
}

func parseToolCallArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return map[string]any{"raw_arguments": raw}
	}
	return args
}

func mapTools(req *model.LLMRequest) []chatCompletionTool {
	if req == nil || req.Config == nil {
		return nil
	}
	tools := make([]chatCompletionTool, 0)
	for _, t := range req.Config.Tools {
		if t == nil {
			continue
		}
		for _, decl := range t.FunctionDeclarations {
			if decl == nil || strings.TrimSpace(decl.Name) == "" {
				continue
			}
			tools = append(tools, chatCompletionTool{
				Type: "function",
				Function: chatCompletionToolFunction{
					Name:        decl.Name,
					Description: decl.Description,
					Parameters:  decl.Parameters,
				},
			})
		}
	}
	return tools
}

func functionResponseMessage(fr *genai.FunctionResponse) (chatCompletionMessage, error) {
	payload := map[string]any{}
	if fr != nil && fr.Response != nil {
		payload = fr.Response
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		return chatCompletionMessage{}, fmt.Errorf("marshal function response payload: %w", err)
	}
	toolCallID := ""
	if fr != nil {
		toolCallID = strings.TrimSpace(fr.ID)
	}
	if toolCallID == "" {
		return chatCompletionMessage{}, fmt.Errorf("function response id is required for tool message")
	}
	return chatCompletionMessage{
		Role:       "tool",
		Content:    string(contentBytes),
		ToolCallID: toolCallID,
	}, nil
}

func functionCallToToolCall(fc *genai.FunctionCall) (chatCompletionToolCall, error) {
	if fc == nil {
		return chatCompletionToolCall{}, fmt.Errorf("function call is nil")
	}
	toolCallID := strings.TrimSpace(fc.ID)
	if toolCallID == "" {
		return chatCompletionToolCall{}, fmt.Errorf("function call id is required in request history")
	}
	name := strings.TrimSpace(fc.Name)
	if name == "" {
		return chatCompletionToolCall{}, fmt.Errorf("function call name is required")
	}
	args := fc.Args
	if args == nil {
		args = map[string]any{}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return chatCompletionToolCall{}, fmt.Errorf("marshal function call args: %w", err)
	}
	return chatCompletionToolCall{
		ID:   toolCallID,
		Type: "function",
		Function: chatCompletionToolFunctionCall{
			Name:      name,
			Arguments: string(argsJSON),
		},
	}, nil
}

func (m *OpenAICompatibleModel) authorizationHeader(ctx context.Context) (string, error) {
	userJWT, err := authctx.UserJWT(ctx)
	if err == nil && strings.TrimSpace(userJWT) != "" {
		return "Bearer " + strings.TrimSpace(userJWT), nil
	}

	if strings.TrimSpace(m.apiKey) != "" {
		return "Bearer " + strings.TrimSpace(m.apiKey), nil
	}

	return "", err
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}

	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		if strings.TrimSpace(part.Text) != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}
