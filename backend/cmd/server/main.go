package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/database"
	"flowmonitor/backend/internal/routes"
	"flowmonitor/backend/internal/seed"
	"flowmonitor/backend/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	svc := service.New(db, cfg)

	if cfg.SeedDemo {
		if err := seed.Demo(db, cfg, svc); err != nil {
			log.Printf("seed: %v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background watcher: materializes stuck alerts and resolves cleared ones.
	watcher := service.NewWatcher(db, cfg)
	go watcher.Start(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes.New(svc, cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("flowmonitor API on http://localhost:%s (db: %s, ingest auth: %t, auto-register: %t)",
			cfg.Port, cfg.DBPath, cfg.IngestAPIKey != "", cfg.AutoRegister)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("stopped")
}
