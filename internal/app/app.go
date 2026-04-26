package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	adksession "google.golang.org/adk/session"

	adkllm "github.com/lumiforge/wb_api_agent_system/internal/adapters/adk/llm"
	adksessionadapter "github.com/lumiforge/wb_api_agent_system/internal/adapters/adk/session"
	sqliteadapter "github.com/lumiforge/wb_api_agent_system/internal/adapters/sqlite"
	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent"
	"github.com/lumiforge/wb_api_agent_system/internal/config"
	"github.com/lumiforge/wb_api_agent_system/internal/services/a2a"
	"github.com/lumiforge/wb_api_agent_system/internal/services/deterministic_planner"
	"github.com/lumiforge/wb_api_agent_system/internal/services/wb_registry"
)

// PURPOSE: Wires infrastructure, agents, services, and the HTTP server into one runnable application.
type Application struct {
	cfg            *config.Config
	logger         *log.Logger
	httpServer     *http.Server
	sessionService adksession.Service
}

func New(cfg *config.Config, logger *log.Logger) (*Application, error) {
	sessionService, err := adksessionadapter.NewSQLiteSessionService(
		cfg.SQLitePath,
		cfg.DatabaseAutoMigrate,
	)
	if err != nil {
		return nil, err
	}

	registryDB, err := sqliteadapter.NewDB(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}

	if cfg.DatabaseAutoMigrate {
		// WHY: The registry loader writes app-owned operation metadata into SQLite at startup.
		if err := sqliteadapter.ApplyMigrationFile(context.Background(), registryDB, "scheme/up.sql"); err != nil {
			return nil, err
		}
	}

	registryStore := sqliteadapter.NewWBRegistryStore(registryDB)
	registryLoader := wb_registry.NewLoader(registryStore)

	loadResult, err := registryLoader.LoadFromDir(context.Background(), cfg.WBRegistryPath)
	if err != nil {
		return nil, err
	}

	logger.Printf(
		"WB OpenAPI registry loaded: files=%d operations=%d generated_operation_ids=%d read=%d write=%d unknown=%d jam_only=%d path=%s",
		loadResult.FilesLoaded,
		loadResult.OperationsLoaded,
		loadResult.GeneratedOperationIDs,
		loadResult.ReadOperations,
		loadResult.WriteOperations,
		loadResult.UnknownRiskOperations,
		loadResult.JamOnlyOperations,
		cfg.WBRegistryPath,
	)

	systemPrompt, err := os.ReadFile(cfg.SystemPromptPath)
	if err != nil {
		return nil, fmt.Errorf("read system prompt: %w", err)
	}

	planPrompt, err := os.ReadFile(cfg.PlanPromptPath)
	if err != nil {
		return nil, fmt.Errorf("read plan prompt: %w", err)
	}

	explorePrompt, err := os.ReadFile(cfg.ExplorePromptPath)
	if err != nil {
		return nil, fmt.Errorf("read explore prompt: %w", err)
	}

	generalPrompt, err := os.ReadFile(cfg.GeneralPromptPath)
	if err != nil {
		return nil, fmt.Errorf("read general prompt: %w", err)
	}

	deterministicPlanner := deterministic_planner.New(registryStore)

	llmModel := adkllm.NewOpenAICompatibleModel(
		cfg.ModelName,
		cfg.OpenAIBaseURL,
		cfg.OpenAIAPIKey,
	)

	plannerAgent, err := wb_api_agent.New(wb_api_agent.Config{
		Registry:             registryStore,
		DeterministicPlanner: deterministicPlanner,
		SessionService:       sessionService,
		Model:                llmModel,
		Logger:               logger,
		SystemPrompt:         string(systemPrompt),
		PlanPrompt:           string(planPrompt),
		ExplorePrompt:        string(explorePrompt),
		GeneralPrompt:        string(generalPrompt),
		ModelName:            cfg.ModelName,
		DebugLogPlannerInput: cfg.DebugLogPlannerInput,
	})
	if err != nil {
		return nil, err
	}

	a2aHandler := a2a.NewHandler(a2a.Config{
		PublicBaseURL: cfg.PublicBaseURL,
		Logger:        logger,
	}, plannerAgent, registryStore)

	mux := http.NewServeMux()

	// WHY: Application layer owns HTTP route registration; the A2A service only exposes handlers.
	mux.HandleFunc("/healthz", a2aHandler.HandleHealth)
	mux.HandleFunc("/a2a", a2aHandler.HandleRPC)
	mux.HandleFunc("/debug/registry/stats", a2aHandler.HandleRegistryStats)
	mux.HandleFunc("/debug/registry/search", a2aHandler.HandleRegistrySearch)
	mux.HandleFunc("/.well-known/agent.json", a2aHandler.HandleAgentCard)
	mux.HandleFunc("/.well-known/agent-card.json", a2aHandler.HandleAgentCard)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Application{
		cfg:            cfg,
		logger:         logger,
		httpServer:     httpServer,
		sessionService: sessionService,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.logger.Printf("WB API Agent System listening on %s", a.cfg.HTTPAddr)

		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// WHY: Shutdown gives in-flight local test requests a short window to finish cleanly.
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		return nil
	case err := <-errCh:
		return err
	}
}
