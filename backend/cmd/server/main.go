package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/handler"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/httpx"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/service"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

func main() {
	// One JSON format for every line, set first so config errors use it too.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// run returns rather than exits, so its deferred cleanup always runs.
	if err := run(logger); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func run(logger *slog.Logger) error {
	port := envOrDefault("PORT", "8080")
	fetchTimeoutSec := envPositiveIntOrDefault("FETCH_TIMEOUT_SECONDS", 30)
	rateLimitPerMin := envPositiveIntOrDefault("RATE_LIMIT_PER_MINUTE", 60)

	// The header the proxy in front of the API writes the client address
	// into, overwriting any value a client sends. Empty means no proxy: the
	// TCP peer is the client, as on a local run.
	clientIPHeader := os.Getenv("CLIENT_IP_HEADER")

	// NWS policy requires an identifying User-Agent with contact info.
	// Overridable so that forks don't misidentify as SentryAtlas.
	nwsUserAgent := envOrDefault("NWS_USER_AGENT", "SentryAtlas/1.0 (github.com/KOHANTIC/SentryAtlas)")

	// Comma-separated list of allowed CORS origins. The default is open:
	// this is a public read-only API.
	allowedOrigins := strings.Split(envOrDefault("ALLOWED_ORIGINS", "*"), ",")
	for i := range allowedOrigins {
		allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
	}

	httpClient := &http.Client{Timeout: time.Duration(fetchTimeoutSec) * time.Second}
	fetchTimeout := time.Duration(fetchTimeoutSec) * time.Second

	usgs := adapters.NewUSGSAdapter(httpClient)
	eonet := adapters.NewEONETAdapter(httpClient)
	noaa := adapters.NewNOAAAdapter(httpClient, nwsUserAgent)
	gdacs := adapters.NewGDACSAdapter(httpClient)
	adapterList := []adapters.Adapter{usgs, eonet, noaa, gdacs}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Each source on its own schedule: NWS alerts change by the minute,
	// USGS's month feed every few, EONET and GDACS more slowly. An NWS copy
	// an hour old may hold alerts since cancelled, so it is withdrawn.
	st := store.New(
		store.Source{Name: usgs.Source()},
		store.Source{Name: eonet.Source()},
		store.Source{Name: noaa.Source(), MaxAge: time.Hour},
		store.Source{Name: gdacs.Source()},
	)
	waitPollers := store.Run(ctx, st, []store.Poll{
		{Fetcher: usgs, Interval: 5 * time.Minute},
		{Fetcher: eonet, Interval: 10 * time.Minute},
		{Fetcher: noaa, Interval: 2 * time.Minute},
		{Fetcher: gdacs, Interval: 10 * time.Minute},
	}, store.Config{Timeout: fetchTimeout, RetryAfter: 30 * time.Second})
	defer waitPollers()

	eventsHandler := handler.NewEventsHandler(service.NewEventsService(adapterList, st))
	go eventsHandler.ReportCalls(ctx, time.Hour)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	if clientIPHeader != "" {
		r.Use(middleware.ClientIPFromHeader(clientIPHeader))
	} else {
		r.Use(middleware.ClientIPFromRemoteAddr)
	}
	r.Use(httpx.RequestLog(logger))
	r.Use(middleware.Recoverer)
	// Types listed explicitly: application/geo+json is not in the
	// middleware's defaults, and it is the bulkiest response we serve.
	// text/event-stream is deliberately absent — compressing SSE buffers
	// flushes and breaks progressive delivery.
	r.Use(middleware.Compress(5, "application/json", "application/geo+json"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", handler.Health(st))

	// Keyed on the address from CLIENT_IP_HEADER only: headers a client can
	// set itself, such as True-Client-IP or the start of X-Forwarded-For,
	// would let anyone reset their own limit. One limiter for v1 and v2, so a
	// client has one budget; /health stays exempt so platform probes are
	// never throttled.
	limit := httprate.LimitBy(rateLimitPerMin, time.Minute, httpx.RateLimitKey,
		httprate.WithLimitHandler(httpx.TooManyRequests))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(limit)
		r.Get("/events", eventsHandler.GetEvents)
	})

	v2 := handler.NewV2Handler(st)
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(limit)
		r.Get("/snapshot", v2.Snapshot)
		r.Get("/events/{id}", v2.Event)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	served := make(chan error, 1)
	go func() {
		// Empty client_ip_header behind a proxy would put every visitor in one
		// rate-limit bucket, so the startup line says which source is in use.
		slog.Info("server starting", "port", port, "client_ip_header", clientIPHeader)
		served <- srv.ListenAndServe()
	}()

	select {
	case err := <-served:
		stop()
		return err
	case <-ctx.Done():
	}

	// The signal also stopped the pollers; the deferred wait collects them.
	slog.Info("server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return errors.Join(errors.New("forced to shut down"), err)
	}
	return nil
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
