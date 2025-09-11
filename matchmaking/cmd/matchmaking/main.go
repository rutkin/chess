package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/example/chess/matchmaking/internal/api"
	"github.com/example/chess/matchmaking/internal/config"
	"github.com/example/chess/matchmaking/internal/matcher"
	"github.com/example/chess/matchmaking/internal/storage"
)

func main() {
	cfg := config.Load()
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	rdb := storage.MustRedis(cfg)
	store := storage.NewStore(rdb, logger)

	m := matcher.NewMatcher(store, logger, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// metrics endpoint - wired via prom.go helper
	r.Get("/metrics", func(w http.ResponseWriter, req *http.Request) { promHTTPHandler().ServeHTTP(w, req) })

	api.RegisterRoutes(r, store, logger, cfg)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		logger.Info("http server starting", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	_ = srv.Shutdown(shutdownCtx)
}

// promHTTPHandler isolated to avoid importing in other files
func promHTTPHandler() http.Handler { return promHandler() }
