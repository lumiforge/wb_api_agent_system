package entities

import "encoding/json"

// PURPOSE: Defines the minimal JSON-RPC envelope used by the A2A-compatible HTTP handler.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type AgentCard struct {
	Name               string                `json:"name"`
	Description        string                `json:"description"`
	URL                string                `json:"url"`
	Version            string                `json:"version"`
	Capabilities       map[string]bool       `json:"capabilities"`
	DefaultInputModes  []string              `json:"defaultInputModes"`
	DefaultOutputModes []string              `json:"defaultOutputModes"`
	Skills             []AgentSkill          `json:"skills"`
	SecuritySchemes    map[string]any        `json:"securitySchemes,omitempty"`
	Security           []map[string][]string `json:"security,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
