package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGenerateContent_MapsToolsAndToolResponses(t *testing.T) {
	var captured chatCompletionRequest
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	m.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatal(err)
		}
		resp := `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"resolve_relative_period","arguments":"{\"period\":\"last_week\"}"}}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
	})}

	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText("sys", genai.RoleUser),
			Tools: []*genai.Tool{
				{FunctionDeclarations: []*genai.FunctionDeclaration{
					{Name: "resolve_relative_period", Description: "resolve", Parameters: &genai.Schema{Type: genai.TypeObject}},
				}},
			},
		},
		Contents: []*genai.Content{
			{Role: string(genai.RoleUser), Parts: []*genai.Part{{Text: "input"}}},
			{Role: string(genai.RoleUser), Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{Name: "get_current_datetime", ID: "call-0", Response: map[string]any{"output": map[string]any{"iso": "2026-05-03T10:00:00Z"}}}}}},
		},
	}

	var got *model.LLMResponse
	for r, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatal(err)
		}
		got = r
	}

	if len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "resolve_relative_period" {
		t.Fatalf("tools not mapped: %+v", captured.Tools)
	}
	if captured.ResponseFormat != nil {
		t.Fatalf("response_format must be nil when tools are present: %+v", captured.ResponseFormat)
	}
	foundToolMsg := false
	for _, msg := range captured.Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call-0" {
			foundToolMsg = true
		}
	}
	if !foundToolMsg {
		t.Fatalf("tool response message not mapped: %+v", captured.Messages)
	}

	if got == nil || got.Content == nil || len(got.Content.Parts) == 0 || got.Content.Parts[0].FunctionCall == nil {
		t.Fatalf("expected function call response, got %+v", got)
	}
	if got.Content.Parts[0].FunctionCall.Name != "resolve_relative_period" {
		t.Fatalf("unexpected function call name: %+v", got.Content.Parts[0].FunctionCall)
	}
	if got.Content.Parts[0].FunctionCall.Args["period"] != "last_week" {
		t.Fatalf("unexpected function call args: %+v", got.Content.Parts[0].FunctionCall.Args)
	}
}

func TestGenerateContent_MapsFunctionCallAndResponseHistory(t *testing.T) {
	var captured chatCompletionRequest
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	m.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatal(err)
		}
		resp := `{"choices":[{"message":{"role":"assistant","content":"{\"status\":\"ready_for_composition\"}"}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
	})}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("input", genai.RoleUser),
			{
				Role: string(genai.RoleModel),
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							ID:   "call-1",
							Name: "get_current_datetime",
							Args: map[string]any{"timezone": "Europe/Moscow"},
						},
					},
				},
			},
			{
				Role: string(genai.RoleUser),
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							ID:       "call-1",
							Name:     "get_current_datetime",
							Response: map[string]any{"current_date": "2026-05-03"},
						},
					},
				},
			},
		},
	}
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(captured.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(captured.Messages))
	}
	assistantMsg := captured.Messages[1]
	if assistantMsg.Role != "assistant" || len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("assistant tool_calls not mapped: %+v", assistantMsg)
	}
	if assistantMsg.ToolCalls[0].ID != "call-1" {
		t.Fatalf("unexpected tool call id: %+v", assistantMsg.ToolCalls[0])
	}
	if assistantMsg.ToolCalls[0].Function.Name != "get_current_datetime" {
		t.Fatalf("unexpected tool call name: %+v", assistantMsg.ToolCalls[0])
	}
	var toolCallArgs map[string]any
	if err := json.Unmarshal([]byte(assistantMsg.ToolCalls[0].Function.Arguments), &toolCallArgs); err != nil {
		t.Fatalf("tool call arguments must be valid json: %v", err)
	}
	if toolCallArgs["timezone"] != "Europe/Moscow" {
		t.Fatalf("unexpected tool call arguments: %+v", toolCallArgs)
	}
	toolMsg := captured.Messages[2]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call-1" {
		t.Fatalf("tool message not mapped: %+v", toolMsg)
	}
}

func TestGenerateContent_TextJSONCompatibilityWithoutTools(t *testing.T) {
	var captured chatCompletionRequest
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	m.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &captured)
		resp := `{"choices":[{"message":{"role":"assistant","content":"{\"status\":\"ready_for_composition\"}"}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
	})}

	req := &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("input", genai.RoleUser)}}
	var got *model.LLMResponse
	for r, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatal(err)
		}
		got = r
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "json_object" {
		t.Fatalf("expected json_object response format when tools absent: %+v", captured.ResponseFormat)
	}
	if got == nil || got.Content == nil || len(got.Content.Parts) == 0 || got.Content.Parts[0].Text == "" {
		t.Fatalf("expected text part response, got %+v", got)
	}
}

func TestGenerateContent_ErrorsOnMissingFunctionResponseID(t *testing.T) {
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: string(genai.RoleUser),
				Parts: []*genai.Part{
					{FunctionResponse: &genai.FunctionResponse{Name: "get_current_datetime", Response: map[string]any{"ok": true}}},
				},
			},
		},
	}
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err == nil || !strings.Contains(err.Error(), "function response id is required") {
			t.Fatalf("expected missing function response id error, got %v", err)
		}
	}
}

func TestGenerateContent_ErrorsOnMissingFunctionCallID(t *testing.T) {
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: string(genai.RoleModel),
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{Name: "get_current_datetime", Args: map[string]any{"timezone": "UTC"}}},
				},
			},
		},
	}
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err == nil || !strings.Contains(err.Error(), "function call id is required") {
			t.Fatalf("expected missing function call id error, got %v", err)
		}
	}
}

func TestGenerateContent_ErrorsOnFunctionCallWithNonModelRole(t *testing.T) {
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: string(genai.RoleUser),
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{ID: "call-1", Name: "get_current_datetime", Args: map[string]any{"timezone": "UTC"}}},
				},
			},
		},
	}
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err == nil || !strings.Contains(err.Error(), "function call content must have model role") {
			t.Fatalf("expected non-model function call role error, got %v", err)
		}
	}
}

func TestGenerateContent_ErrorsOnMissingFunctionCallName(t *testing.T) {
	m := NewOpenAICompatibleModel("gpt-4o-mini", "http://example.test", "k")
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: string(genai.RoleModel),
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{ID: "call-1", Name: "   ", Args: map[string]any{"timezone": "UTC"}}},
				},
			},
		},
	}
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err == nil || !strings.Contains(err.Error(), "function call name is required") {
			t.Fatalf("expected missing function call name error, got %v", err)
		}
	}
}
