package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	adksession "google.golang.org/adk/session"

	adkllm "github.com/lumiforge/wb_api_agent_system/internal/adapters/adk/llm"
	adksessionadapter "github.com/lumiforge/wb_api_agent_system/internal/adapters/adk/session"
	sqliteadapter "github.com/lumiforge/wb_api_agent_system/internal/adapters/sqlite"
	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent"
	"github.com/lumiforge/wb_api_agent_system/internal/config"
	"github.com/lumiforge/wb_api_agent_system/internal/services/a2a"
	"github.com/lumiforge/wb_api_agent_system/internal/services/wb_registry"
	"github.com/lumiforge/wb_api_agent_system/internal/services/wb_registry_retrieval"
)

// PURPOSE: Wires infrastructure, agents, services, and the HTTP server into one runnable application.
type Application struct {
	cfg                    *config.Config
	logger                 *log.Logger
	httpServer             *http.Server
	sessionService         adksession.Service
	registryEmbeddingStore *sqliteadapter.WBRegistryEmbeddingStore
}

func New(cfg *config.Config, logger *log.Logger) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if logger == nil {
		logger = log.Default()
	}

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

	embeddingsDB, err := sqliteadapter.NewDB(cfg.EmbeddingsSQLitePath)
	if err != nil {
		return nil, err
	}

	if cfg.DatabaseAutoMigrate {
		// WHY: Registry embeddings are app-owned and must live outside ADK session storage.
		if err := sqliteadapter.AutoMigrateWBRegistryEmbeddingStore(embeddingsDB); err != nil {
			return nil, err
		}
	}

	registryStore := sqliteadapter.NewWBRegistryStore(registryDB)
	registryLoader := wb_registry.NewLoader(registryStore)
	registryEmbeddingStore := sqliteadapter.NewWBRegistryEmbeddingStore(embeddingsDB)

	embeddingClient := adkllm.NewOpenAICompatibleEmbeddingClient(
		cfg.ModelProxyBaseURL,
		"",
	)

	embeddingStatusService, err := wb_registry_retrieval.NewEmbeddingIndexStatusService(wb_registry_retrieval.EmbeddingIndexStatusServiceConfig{
		SourceStore:    registryStore,
		EmbeddingStore: registryEmbeddingStore,
		Model:          cfg.EmbeddingModel,
		Dimensions:     cfg.EmbeddingDimensions,
	})
	if err != nil {
		return nil, fmt.Errorf("create registry embedding status service: %w", err)
	}

	logger.Printf(
		"WB registry embedding store ready: path=%s model=%s dimensions=%d",
		cfg.EmbeddingsSQLitePath,
		cfg.EmbeddingModel,
		cfg.EmbeddingDimensions,
	)

	if cfg.EmbeddingIndexRebuildOnStartup {
		embeddingIndexer, err := wb_registry_retrieval.NewEmbeddingIndexer(wb_registry_retrieval.EmbeddingIndexerConfig{
			SourceStore:     registryStore,
			EmbeddingStore:  registryEmbeddingStore,
			EmbeddingClient: embeddingClient,
			Model:           cfg.EmbeddingModel,
			Dimensions:      cfg.EmbeddingDimensions,
			BatchSize:       64,
		})
		if err != nil {
			return nil, fmt.Errorf("create registry embedding indexer: %w", err)
		}

		// WHY: Embedding rebuild is explicit opt-in because it performs external model calls at startup.
		indexResult, err := embeddingIndexer.Rebuild(context.Background())
		if err != nil {
			return nil, fmt.Errorf("rebuild registry embedding index: %w", err)
		}

		logger.Printf(
			"WB registry embedding index rebuilt: scanned=%d created=%d skipped=%d model=%s dimensions=%d",
			indexResult.OperationsScanned,
			indexResult.EmbeddingsCreated,
			indexResult.EmbeddingsSkipped,
			cfg.EmbeddingModel,
			cfg.EmbeddingDimensions,
		)
	}

	loadResult, err := registryLoader.LoadFromDir(context.Background(), cfg.WBRegistryPath)
	if err != nil {
		return nil, err
	}

	var semanticRetriever wb_registry_retrieval.SemanticCandidateRetriever
	if cfg.SemanticRetrievalEnabled {
		semanticOperationRetriever, err := wb_registry_retrieval.NewSemanticOperationRetriever(wb_registry_retrieval.SemanticOperationRetrieverConfig{
			SourceStore:     registryStore,
			EmbeddingStore:  registryEmbeddingStore,
			EmbeddingClient: embeddingClient,
			Model:           cfg.EmbeddingModel,
			Dimensions:      cfg.EmbeddingDimensions,
		})
		if err != nil {
			return nil, fmt.Errorf("create semantic operation retriever: %w", err)
		}

		semanticRetriever = semanticOperationRetriever
	}

	registryRetriever, err := wb_registry_retrieval.New(wb_registry_retrieval.ServiceConfig{
		Store:                    registryStore,
		SemanticRetriever:        semanticRetriever,
		SemanticExpansionEnabled: cfg.SemanticRetrievalEnabled,
		SemanticExpansionLimit:   cfg.SemanticRetrievalLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("create registry retriever: %w", err)
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

	llmModel := adkllm.NewOpenAICompatibleModel(
		cfg.ModelName,
		cfg.ModelProxyBaseURL,
		"",
	)

	plannerAgent, err := wb_api_agent.New(wb_api_agent.Config{
		Registry:       registryRetriever,
		SessionService: sessionService,
		Model:          llmModel,
		Logger:         logger,
		ModelName:      cfg.ModelName,
	})
	if err != nil {
		return nil, err
	}

	a2aHandler := a2a.NewHandler(a2a.Config{
		PublicBaseURL:                cfg.PublicBaseURL,
		Logger:                       logger,
		EmbeddingIndexStatusProvider: embeddingStatusService,
	}, plannerAgent, registryRetriever)

	mux := http.NewServeMux()

	// WHY: Application layer owns HTTP route registration; the A2A service only exposes handlers.
	mux.HandleFunc("/healthz", a2aHandler.HandleHealth)
	mux.HandleFunc("/a2a", a2aHandler.HandleRPC)
	mux.HandleFunc("/debug/registry/stats", a2aHandler.HandleRegistryStats)
	mux.HandleFunc("/debug/registry/search", a2aHandler.HandleRegistrySearch)
	mux.HandleFunc("/debug/registry/embeddings/status", a2aHandler.HandleRegistryEmbeddingsStatus)
	mux.HandleFunc("/.well-known/agent.json", a2aHandler.HandleAgentCard)
	mux.HandleFunc("/.well-known/agent-card.json", a2aHandler.HandleAgentCard)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Application{
		cfg:                    cfg,
		logger:                 logger,
		httpServer:             httpServer,
		sessionService:         sessionService,
		registryEmbeddingStore: registryEmbeddingStore,
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
