package wb_api_agent

import "github.com/lumiforge/wb_api_agent_system/internal/domain/entities"

// PURPOSE: Defines the registry-grounded input prepared for ADK planner/formatter execution.
type PlannerInput struct {
	BusinessRequest    entities.BusinessRequest      `json:"business_request"`
	RegistryCandidates []RegistryCandidateForPlanner `json:"registry_candidates"`
	Prompts            PlannerPrompts                `json:"prompts"`
	Policies           PlannerPolicies               `json:"policies"`
	OutputContract     string                        `json:"output_contract"`
	Metadata           *entities.RequestMetadata     `json:"metadata,omitempty"`
}

type PlannerPrompts struct {
	System  string `json:"system"`
	Plan    string `json:"plan"`
	Explore string `json:"explore"`
	General string `json:"general"`
}

type PlannerPolicies struct {
	ReadonlyOnly      bool `json:"readonly_only"`
	NoJamSubscription bool `json:"no_jam_subscription"`
	NoSecrets         bool `json:"no_secrets"`
	NoHTTPExecution   bool `json:"no_http_execution"`
	RegistryOnly      bool `json:"registry_only"`
}

type RegistryCandidateForPlanner struct {
	OperationID              string   `json:"operation_id"`
	SourceFile               string   `json:"source_file"`
	Method                   string   `json:"method"`
	ServerURL                string   `json:"server_url"`
	PathTemplate             string   `json:"path_template"`
	Tags                     []string `json:"tags"`
	Category                 string   `json:"category"`
	Summary                  string   `json:"summary"`
	Description              string   `json:"description"`
	Readonly                 *bool    `json:"readonly"`
	RiskLevel                string   `json:"risk_level"`
	XCategory                string   `json:"x_category"`
	XTokenTypes              []string `json:"x_token_types"`
	PathParamsSchemaJSON     string   `json:"path_params_schema_json"`
	QueryParamsSchemaJSON    string   `json:"query_params_schema_json"`
	HeadersSchemaJSON        string   `json:"headers_schema_json"`
	RequestBodySchemaJSON    string   `json:"request_body_schema_json"`
	ResponseSchemaJSON       string   `json:"response_schema_json"`
	RateLimitNotes           string   `json:"rate_limit_notes"`
	SubscriptionRequirements string   `json:"subscription_requirements"`
	RequiresJam              bool     `json:"requires_jam"`
}
