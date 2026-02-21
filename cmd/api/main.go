package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/apascualco/gotway/internal/infrastructure/http"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	cfg, err := config.Load(version, commit, buildDate)
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	setupLogger(cfg)

	s, err := http.NewServer(cfg)
	if err != nil {
		slog.Error("failed to create server", slog.Any("error", err))
		os.Exit(1)
	}

	go func() {
		slog.Info("starting server",
			slog.Int("port", cfg.Port),
			slog.String("env", cfg.Env),
			slog.String("version", version),
			slog.String("commit", commit),
			slog.String("build_date", buildDate),
		)
		if err := s.Run(); err != nil {
			slog.Error("server error", slog.Any("error", err))
		}
	}()

	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGINT, syscall.SIGTERM)
	<-q

	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server forced to shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	slog.InfoContext(ctx, "server exited")
}

func setupLogger(cfg *config.Config) {
	var h slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() == slog.KindDuration {
				a.Value = slog.StringValue(a.Value.Duration().String())
			}
			return a
		},
	}

	if cfg.Env == "production" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(h))
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
