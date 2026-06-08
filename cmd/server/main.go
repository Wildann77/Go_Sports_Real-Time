package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sports-dashboard/internal"
	"sports-dashboard/internal/core/config"
	"sports-dashboard/internal/core/database"
)

func main() {
	// Initialize config
	cfg := config.LoadConfig()

	// Initialize logger
	setupLogger(cfg.AppEnv)

	// Print custom pink and white ASCII banner with environment details

	const (
		reset = "\033[0m"
		gray  = "\033[38;5;240m"
		white = "\033[97m"
		pink  = "\033[38;5;198m"
		muted = "\033[38;5;250m"
	)

	fmt.Print(white + "  █▀▀█ ████ █  █ " + pink + "█▀▀█ █    █▀▀█ " + white + "█▀▀█ █▀▀  ████ " + reset + "\n")
	fmt.Print(white + "  █▀▀▄ █  █ ▀▄▄█ " + pink + "█▀▀▄ █    █▄▄█ " + white + "█  █ █    █  █ " + reset + "\n")
	fmt.Print(white + "  █▄▄█ █▄▄█  ▄▄█ " + pink + "█▄▄█ █▄▄█ █  █ " + white + "█  █ █▄▄  █▄▄█ " + reset + "\n")

	fmt.Print("  " + gray + "==========================================" + reset + "\n")
	fmt.Print("  " + pink + "● " + white + "Sports Real-Time Dashboard Server " + muted + "v1.0.0" + reset + "\n")
	fmt.Print("  " + gray + "==========================================" + reset + "\n")
	fmt.Printf("  "+muted+"[Port]        "+gray+": "+white+"%s"+reset+"\n", cfg.Port)
	fmt.Printf("  "+muted+"[Environment] "+gray+": "+white+"%s"+reset+"\n", cfg.AppEnv)
	fmt.Print("  " + gray + "==========================================" + reset + "\n")

	// Initialize database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("Failed to access sql.DB handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Setup Router
	router, cleanup := internal.SetupRouter(appCtx, cfg, db)
	defer cleanup()

	// Setup server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		slog.Info("Starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}
	appCancel()
	cleanup()
	slog.Info("Server exiting")
}

func setupLogger(env string) {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
