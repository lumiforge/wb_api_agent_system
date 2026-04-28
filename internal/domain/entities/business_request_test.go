package entities

import (
	"encoding/json"
	"testing"
)

func TestBusinessRequestJSONParsingWithMetadata(t *testing.T) {
	raw := []byte(`{
		"request_id":"req_1",
		"marketplace":"wildberries",
		"natural_language_request":"Get stocks",
		"metadata":{
			"correlation_id":"corr_1",
			"session_id":"sess_1",
			"run_id":"run_1",
			"tool_call_id":"call_1",
			"client_execution_id":"exec_1",
			"user_id":"user_1",
			"source":"sp_agent"
		}
	}`)

	var request BusinessRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}

	if request.Metadata == nil {
		t.Fatal("expected metadata")
	}
	if request.Metadata.CorrelationID != "corr_1" {
		t.Fatalf("expected corr_1, got %q", request.Metadata.CorrelationID)
	}
}

func TestBusinessRequestMetadataOptional(t *testing.T) {
	raw := []byte(`{
		"request_id":"req_1",
		"marketplace":"wildberries",
		"natural_language_request":"Get stocks"
	}`)

	var request BusinessRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}

	if request.Metadata != nil {
		t.Fatalf("expected nil metadata, got %#v", request.Metadata)
	}
}

func TestNormalizeCorrelationFallbacks(t *testing.T) {
	requestWithRequestID := BusinessRequest{RequestID: "req_1"}
	requestWithRequestID.NormalizeCorrelationIdentifiers()
	if requestWithRequestID.Metadata == nil || requestWithRequestID.Metadata.CorrelationID != "req_1" {
		t.Fatalf("expected metadata.correlation_id=req_1, got %#v", requestWithRequestID.Metadata)
	}

	requestWithCorrelationID := BusinessRequest{Metadata: &RequestMetadata{CorrelationID: "corr_1"}}
	requestWithCorrelationID.NormalizeCorrelationIdentifiers()
	if requestWithCorrelationID.RequestID != "corr_1" {
		t.Fatalf("expected request_id corr_1, got %q", requestWithCorrelationID.RequestID)
	}

	requestBothEmpty := BusinessRequest{}
	requestBothEmpty.NormalizeCorrelationIdentifiers()
	if requestBothEmpty.RequestID != "" {
		t.Fatalf("expected empty request_id, got %q", requestBothEmpty.RequestID)
	}
	if requestBothEmpty.Metadata != nil {
		t.Fatalf("expected nil metadata, got %#v", requestBothEmpty.Metadata)
	}
}
