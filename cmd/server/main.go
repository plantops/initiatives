package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"initiative-app/internal/app"
)

func main() {
	logger := log.New(os.Stdout, "initiative: ", log.LstdFlags|log.LUTC)

	addr := env("APP_ADDR", "127.0.0.1:8080")
	dataDir := env("APP_DATA_DIR", "data")
	dbPath := env("APP_DB", filepath.Join(dataDir, "initiative.db"))
	uploadDir := env("APP_UPLOAD_DIR", filepath.Join(dataDir, "uploads"))
	cookieSecure := strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true") || os.Getenv("COOKIE_SECURE") == "1"

	application, err := app.New(dbPath, uploadDir, cookieSecure, logger)
	if err != nil {
		logger.Fatal(err)
	}
	defer application.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Printf("listening on http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
