package a2a

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/llm"
)

type Config struct {
	PublicBaseURL string
}

// PURPOSE: Exposes the WB API planner through minimal A2A-compatible HTTP routes.
type Handler struct {
	cfg     Config
	planner llm.Planner
}

func NewHandler(cfg Config, planner llm.Planner) *Handler {
	return &Handler{
		cfg:     cfg,
		planner: planner,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/a2a", h.handleRPC)
	mux.HandleFunc("/.well-known/agent.json", h.handleAgentCard)
	mux.HandleFunc("/.well-known/agent-card.json", h.handleAgentCard)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.agentCard())
}

func (h *Handler) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	var rpcRequest entities.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcRequest); err != nil {
		writeJSONRPCError(w, nil, http.StatusBadRequest, -32700, "parse error")
		return
	}

	if rpcRequest.JSONRPC != "2.0" {
		writeJSONRPCError(w, rpcRequest.ID, http.StatusBadRequest, -32600, "invalid jsonrpc version")
		return
	}

	switch rpcRequest.Method {
	case "message/send":
		h.handleMessageSend(w, r, rpcRequest)
	default:
		writeJSONRPCError(w, rpcRequest.ID, http.StatusOK, -32601, "method not found")
	}
}

func (h *Handler) handleMessageSend(w http.ResponseWriter, r *http.Request, rpcRequest entities.JSONRPCRequest) {
	businessRequest := parseBusinessRequest(rpcRequest.Params)

	plan, err := h.planner.Plan(r.Context(), businessRequest)
	if err != nil {
		writeJSONRPCError(w, rpcRequest.ID, http.StatusOK, -32603, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entities.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcRequest.ID,
		Result:  plan,
	})
}

func (h *Handler) agentCard() entities.AgentCard {
	return entities.AgentCard{
		Name:        "WB API Agent System",
		Description: "Plans Wildberries API calls and returns machine-executable ApiExecutionPlan objects without executing HTTP requests or handling secrets.",
		URL:         strings.TrimRight(h.cfg.PublicBaseURL, "/") + "/a2a",
		Version:     "0.1.0",
		Capabilities: map[string]bool{
			"streaming":              false,
			"pushNotifications":      false,
			"stateTransitionHistory": false,
		},
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"application/json"},
		Skills: []entities.AgentSkill{
			{
				ID:          "wildberries_api_planning",
				Name:        "Wildberries API planning",
				Description: "Builds ApiExecutionPlan objects for Wildberries API operations from business requests.",
				Tags:        []string{"wildberries", "api-planning", "readonly", "a2a"},
			},
		},
	}
}

func parseBusinessRequest(raw json.RawMessage) entities.BusinessRequest {
	var request entities.BusinessRequest
	if len(raw) == 0 {
		return request
	}

	// WHY: Local tests can send the business request directly as JSON-RPC params before full A2A message parsing exists.
	_ = json.Unmarshal(raw, &request)

	return request
}

func writeJSONRPCError(w http.ResponseWriter, id any, statusCode int, code int, message string) {
	writeJSON(w, statusCode, entities.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &entities.JSONRPCError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
