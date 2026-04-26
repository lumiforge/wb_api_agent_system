package config

import (
	"log"
	"os"
	"strconv"
)

// PURPOSE: Holds runtime settings loaded from environment variables at process startup.
type Config struct {
	OpenAIAPIKey        string
	OpenAIBaseURL       string
	WBRegistryPath      string
	ModelName           string
	SystemPromptPath    string
	PlanPromptPath      string
	ExplorePromptPath   string
	GeneralPromptPath   string
	SQLitePath          string
	DatabaseAutoMigrate bool
	HTTPAddr            string
	PublicBaseURL       string

	CompactionEnabled            bool
	CompactionTokenThreshold     int
	CompactionRetainRecentEvents int
	CompactionMaxToolResultChars int
}

func Load() *Config {
	return &Config{
		OpenAIAPIKey:        getEnv("HYDRA_AI_API_KEY", ""),
		OpenAIBaseURL:       getEnv("HYDRA_AI_BASE_URL", ""),
		ModelName:           getEnv("SP_AGENT_MODEL", "gpt-4o-mini"),
		SystemPromptPath:    getEnv("SP_AGENT_SYSTEM_PROMPT_PATH", "internal/agents/wb_api_agent/prompts/system.md"),
		PlanPromptPath:      getEnv("SP_AGENT_PLAN_PROMPT_PATH", "internal/agents/wb_api_agent/prompts/plan.md"),
		ExplorePromptPath:   getEnv("SP_AGENT_EXPLORE_PROMPT_PATH", "internal/agents/wb_api_agent/prompts/explore.md"),
		GeneralPromptPath:   getEnv("SP_AGENT_GENERAL_PROMPT_PATH", "internal/agents/wb_api_agent/prompts/general.md"),
		WBRegistryPath:      getEnv("SP_AGENT_WB_REGISTRY_PATH", "docs/wb-api"),
		SQLitePath:          getEnv("SP_AGENT_SQLITE_PATH", "wb_api_agent_system.db"),
		DatabaseAutoMigrate: getEnvBool("SP_AGENT_DATABASE_AUTO_MIGRATE", true),
		HTTPAddr:            getEnv("SP_AGENT_HTTP_ADDR", ":8080"),
		PublicBaseURL:       getEnv("SP_AGENT_PUBLIC_BASE_URL", "http://localhost:8080"),

		CompactionEnabled:            getEnvBool("SP_AGENT_COMPACTION_ENABLED", true),
		CompactionTokenThreshold:     getEnvInt("SP_AGENT_COMPACTION_TOKEN_THRESHOLD", 60000, 1000, 2000000),
		CompactionRetainRecentEvents: getEnvInt("SP_AGENT_COMPACTION_RETAIN_RECENT_EVENTS", 8, 1, 100),
		CompactionMaxToolResultChars: getEnvInt("SP_AGENT_COMPACTION_MAX_TOOL_RESULT_CHARS", 4000, 100, 100000),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if fallback == "" {
		log.Fatalf("FATAL: Environment variable %s is not set.", key)
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("WARN: %s=%q is not a bool, using default %t", key, value, fallback)
		return fallback
	}

	return parsed
}

func getEnvInt(key string, fallback, min, max int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("WARN: %s=%q is not an integer, using default %d", key, value, fallback)
		return fallback
	}

	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}

	return parsed
}
