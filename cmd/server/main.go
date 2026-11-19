package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/database"
	"github.com/SabhyaAggarwal/gritloom/internal/server"
)

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be a boolean", name)
	}
	return parsed
}

func main() {
	production := os.Getenv("GRITLOOM_ENV") == "production"
	dataDirectory := os.Getenv("GRITLOOM_DATA_DIR")
	if dataDirectory == "" {
		dataDirectory = "data"
	}
	webRoot := os.Getenv("GRITLOOM_WEB_ROOT")
	if webRoot == "" {
		webRoot = filepath.Join("web", "dist")
	}
	if _, err := os.Stat(webRoot); err != nil {
		webRoot = ""
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(ctx, filepath.Join(dataDirectory, "gritloom.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	application, err := server.New(db, server.Config{
		SecureCookies: envBool("GRITLOOM_SECURE_COOKIES", production),
		Development:   !production,
		TrustProxy:    envBool("GRITLOOM_TRUST_PROXY", false),
		WebRoot:       webRoot,
	})
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()
	log.Printf("GritLoom listening on http://localhost:%s", port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
