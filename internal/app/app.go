package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lumiforge/wb_api_agent_system/internal/agents/wb_api_agent"
	"github.com/lumiforge/wb_api_agent_system/internal/config"
	"github.com/lumiforge/wb_api_agent_system/internal/services/a2a"
	adksession "google.golang.org/adk/session"
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

	plannerAgent := wb_api_agent.New()

	a2aHandler := a2a.NewHandler(a2a.Config{
		PublicBaseURL: cfg.PublicBaseURL,
	}, plannerAgent)

	mux := http.NewServeMux()
	a2aHandler.RegisterRoutes(mux)

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
