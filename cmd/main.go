package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lumiforge/wb_api_agent_system/internal/app"
	"github.com/lumiforge/wb_api_agent_system/internal/config"
)

func main() {
	cfg := config.Load()

	application, err := app.New(cfg, log.Default())
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
