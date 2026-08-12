package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/cache"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/handler"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/service"
)

func main() {
	port := envOrDefault("PORT", "8080")
	cacheTTLMin := envPositiveIntOrDefault("CACHE_TTL_MINUTES", 5)
	fetchTimeoutSec := envPositiveIntOrDefault("FETCH_TIMEOUT_SECONDS", 30)

	httpClient := &http.Client{Timeout: time.Duration(fetchTimeoutSec) * time.Second}

	adapterList := []adapters.Adapter{
		adapters.NewUSGSAdapter(httpClient),
		adapters.NewEONETAdapter(httpClient),
		adapters.NewNOAAAdapter(httpClient, "SentryAtlas/1.0 (github.com/KOHANTIC/SentryAtlas)"),
		adapters.NewGDACSAdapter(httpClient),
	}

	eventsCache := cache.New[[]models.Event](time.Duration(cacheTTLMin) * time.Minute)
	defer eventsCache.Close()

	eventsSvc := service.NewEventsService(
		adapterList,
		eventsCache,
		time.Duration(fetchTimeoutSec)*time.Second,
	)

	eventsHandler := handler.NewEventsHandler(eventsSvc)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/events", eventsHandler.GetEvents)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("server shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envPositiveIntOrDefault reads an integer env var that must be positive.
// A set-but-invalid value is a fatal misconfiguration: silently falling back
// would hide the operator's mistake, and zero/negative values break the cache
// ticker and fetch timeouts downstream.
func envPositiveIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil || i <= 0 {
		slog.Error("invalid environment variable: must be a positive integer",
			"key", key,
			"value", v,
		)
		os.Exit(1)
	}
	return i
}
