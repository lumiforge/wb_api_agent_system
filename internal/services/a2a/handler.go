package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/entities"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/llm"
	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
)

type Config struct {
	PublicBaseURL string
	Logger        *log.Logger
}

const (
	a2aMaxRequestBytes = 1 << 20
	a2aRequestTimeout  = 120 * time.Second
)

// PURPOSE: Exposes the WB API planner through minimal A2A-compatible HTTP routes.
type Handler struct {
	cfg      Config
	planner  llm.Planner
	registry wbregistry.Retriever
	logger   *log.Logger
}

func NewHandler(cfg Config, planner llm.Planner, registry wbregistry.Retriever) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	return &Handler{
		cfg:      cfg,
		planner:  planner,
		registry: registry,
		logger:   logger,
	}
}
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) HandleRegistryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	stats, err := h.registry.Stats(r.Context())
	if err != nil {
		writeJSONRPCError(w, nil, http.StatusInternalServerError, -32603, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) HandleRegistrySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	operations, err := h.registry.SearchOperations(r.Context(), wbregistry.SearchQuery{
		Query:        r.URL.Query().Get("q"),
		Limit:        limit,
		ReadonlyOnly: r.URL.Query().Get("readonly_only") == "true",
		ExcludeJam:   r.URL.Query().Get("exclude_jam") == "true",
	})
	if err != nil {
		writeJSONRPCError(w, nil, http.StatusInternalServerError, -32603, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"operations": operations,
	})
}

func (h *Handler) HandleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32601, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.agentCard())
}

func (h *Handler) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONRPCError(w, nil, http.StatusMethodNotAllowed, -32600, "method must be POST")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), a2aRequestTimeout)
	defer cancel()

	r = r.WithContext(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, a2aMaxRequestBytes)

	rpcRequest, err := decodeJSONRPCRequest(r)
	if err != nil {
		writeJSONRPCDecodeError(w, err)
		return
	}

	if rpcRequest.JSONRPC != "2.0" {
		writeJSONRPCError(w, rpcRequest.ID, http.StatusBadRequest, -32600, "invalid jsonrpc version")
		return
	}

	if rpcRequest.Method == "" {
		writeJSONRPCError(w, rpcRequest.ID, http.StatusBadRequest, -32600, "method is required")
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
	startedAt := time.Now()

	businessRequest, err := parseBusinessRequest(rpcRequest.Params)
	if err != nil {
		h.logA2AResult(rpcRequest.ID, "", "", "invalid_params", time.Since(startedAt), err)
		writeJSONRPCError(w, rpcRequest.ID, http.StatusBadRequest, -32602, err.Error())
		return
	}

	plan, err := h.planner.Plan(r.Context(), businessRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			h.logA2AResult(rpcRequest.ID, businessRequest.RequestID, businessRequest.Intent, "timeout", time.Since(startedAt), err)
			writeJSONRPCError(w, rpcRequest.ID, http.StatusOK, -32000, "request timeout")
			return
		}

		h.logA2AResult(rpcRequest.ID, businessRequest.RequestID, businessRequest.Intent, "internal_error", time.Since(startedAt), err)
		writeJSONRPCError(w, rpcRequest.ID, http.StatusOK, -32603, "internal error")
		return
	}

	status := ""
	if plan != nil {
		status = plan.Status
	}

	h.logA2AResult(rpcRequest.ID, businessRequest.RequestID, businessRequest.Intent, status, time.Since(startedAt), nil)

	writeJSON(w, http.StatusOK, entities.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcRequest.ID,
		Result:  plan,
	})
}
func (h *Handler) logA2AResult(
	jsonrpcID any,
	businessRequestID string,
	intent string,
	status string,
	duration time.Duration,
	err error,
) {
	if err != nil {
		h.logger.Printf(
			"a2a message/send finished jsonrpc_id=%v request_id=%s intent=%s status=%s duration_ms=%d error=%q",
			jsonrpcID,
			businessRequestID,
			intent,
			status,
			duration.Milliseconds(),
			err.Error(),
		)
		return
	}

	h.logger.Printf(
		"a2a message/send finished jsonrpc_id=%v request_id=%s intent=%s status=%s duration_ms=%d",
		jsonrpcID,
		businessRequestID,
		intent,
		status,
		duration.Milliseconds(),
	)
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

func parseBusinessRequest(raw json.RawMessage) (entities.BusinessRequest, error) {
	var request entities.BusinessRequest
	if len(raw) == 0 {
		return request, fmt.Errorf("params are required")
	}

	// WHY: Boundary params must be valid BusinessRequest JSON before planner execution starts.
	if err := json.Unmarshal(raw, &request); err != nil {
		return request, fmt.Errorf("invalid params: %w", err)
	}

	return request, nil
}
func decodeJSONRPCRequest(r *http.Request) (entities.JSONRPCRequest, error) {
	var request entities.JSONRPCRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		return request, err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return request, fmt.Errorf("request body must contain exactly one JSON object")
	}

	return request, nil
}

func writeJSONRPCDecodeError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "http: request body too large") {
		writeJSONRPCError(w, nil, http.StatusRequestEntityTooLarge, -32600, "request body too large")
		return
	}

	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		writeJSONRPCError(w, nil, http.StatusBadRequest, -32700, "parse error")
		return
	}

	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		writeJSONRPCError(w, nil, http.StatusBadRequest, -32600, "invalid request")
		return
	}

	if strings.Contains(err.Error(), "unknown field") {
		writeJSONRPCError(w, nil, http.StatusBadRequest, -32600, "invalid request")
		return
	}

	writeJSONRPCError(w, nil, http.StatusBadRequest, -32600, "invalid request")
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
