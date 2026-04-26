package entities

// PURPOSE: Represents one indexed Wildberries OpenAPI operation available for planning.
type WBRegistryOperation struct {
	Marketplace              string   `json:"marketplace"`
	SourceFile               string   `json:"source_file"`
	OperationID              string   `json:"operation_id"`
	Method                   string   `json:"method"`
	ServerURL                string   `json:"server_url"`
	PathTemplate             string   `json:"path_template"`
	Tags                     []string `json:"tags"`
	Category                 string   `json:"category"`
	Summary                  string   `json:"summary"`
	Description              string   `json:"description"`
	XReadonlyMethod          *bool    `json:"x_readonly_method"`
	XCategory                string   `json:"x_category"`
	XTokenTypes              []string `json:"x_token_types"`
	PathParamsSchemaJSON     string   `json:"path_params_schema_json"`
	QueryParamsSchemaJSON    string   `json:"query_params_schema_json"`
	HeadersSchemaJSON        string   `json:"headers_schema_json"`
	RequestBodySchemaJSON    string   `json:"request_body_schema_json"`
	ResponseSchemaJSON       string   `json:"response_schema_json"`
	RateLimitNotes           string   `json:"rate_limit_notes"`
	SubscriptionRequirements string   `json:"subscription_requirements"`
	MaxPeriodDays            *int     `json:"max_period_days"`
	MaxLookbackDays          *int     `json:"max_lookback_days"`
	RequiresJam              bool     `json:"requires_jam"`
}
