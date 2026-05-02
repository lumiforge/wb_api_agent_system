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
}

type chatCompletionResponseFmt struct {
	Type string `json:"type"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
		responseText, err := m.generate(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}

		yield(&model.LLMResponse{
			Content: genai.NewContentFromText(responseText, genai.RoleModel),
		}, nil)
	}
}

func (m *OpenAICompatibleModel) generate(ctx context.Context, req *model.LLMRequest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("openai compatible model is nil")
	}
	if req == nil {
		return "", fmt.Errorf("llm request is nil")
	}
	if m.baseURL == "" {
		return "", fmt.Errorf("model proxy base url is required")
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

		text := contentText(content)
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
		return "", fmt.Errorf("model name is required")
	}

	body := chatCompletionRequest{
		Model:       requestModel,
		Messages:    messages,
		Temperature: 0,
		ResponseFormat: &chatCompletionResponseFmt{
			Type: "json_object",
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create chat completion request: %w", err)
	}

	authHeader, err := m.authorizationHeader(ctx)
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send chat completion request: %w", err)
	}
	defer httpResp.Body.Close()

	responsePayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", fmt.Errorf("chat completion failed: status=%d body=%s", httpResp.StatusCode, string(responsePayload))
	}

	var response chatCompletionResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return "", fmt.Errorf("parse chat completion response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("chat completion response has no choices")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
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
