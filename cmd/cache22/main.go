// Command cache22 runs the Cache22 one-time secret sharing server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nekonaru/cache22/internal/config"
	"github.com/nekonaru/cache22/internal/server"
	"github.com/nekonaru/cache22/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cache22:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(flag.CommandLine, os.Args[1:])
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	st := store.NewMemory(cfg.MaxTotalBytes)
	srv := server.New(cfg, st, logger, time.Now)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Expired-drop sweeper; interval fixed per PRD.
	sweeperDone := make(chan struct{})
	go func() {
		defer close(sweeperDone)
		srv.Sweeper(ctx.Done(), time.Minute)
	}()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr, "base_url", cfg.BaseURL)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	<-sweeperDone
	return nil
}
