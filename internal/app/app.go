package app

import (
	"context"
	"fmt"
	"log"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/config"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ratelimit"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/repository/postgres"
	httpserver "github.com/shahparshva72/boundary-bytes-go-backend/internal/transport/http"
)

type App struct {
	config *config.Config
	db     postgres.Service
	server *httpserver.Server

	stopBackground context.CancelFunc
}

func New() (*App, error) {
	cfg := config.Load()

	db, err := postgres.New(cfg.DBConnectionURL())
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	sqlGenerator := ai.NewGeminiSQLService(ai.GeminiSQLConfig{
		APIKey:  cfg.AI.GoogleAPIKey,
		Model:   cfg.AI.GeminiModel,
		Timeout: cfg.AI.Timeout,
	})

	var dailyLimiter *ratelimit.DailyLimiter
	if cfg.RateLimit.Enabled() {
		redisClient, err := ratelimit.NewUpstashClient(cfg.RateLimit.UpstashURL, cfg.RateLimit.UpstashToken)
		if err != nil {
			return nil, fmt.Errorf("initialize upstash redis: %w", err)
		}

		dailyLimiter, err = ratelimit.NewDailyLimiter(redisClient, cfg.RateLimit.IPHashSecret, cfg.RateLimit.DailyLimit)
		if err != nil {
			return nil, fmt.Errorf("initialize daily rate limiter: %w", err)
		}
	} else {
		log.Println("Warning: text-to-sql rate limiting disabled (missing UPSTASH_REDIS_REST_URL, UPSTASH_REDIS_REST_TOKEN, or RATE_LIMIT_IP_HASH_SECRET)")
	}

	backgroundCtx, stopBackground := context.WithCancel(context.Background())

	var jevClassifier *ai.JevClient
	if cfg.TypeSafe.Enabled() {
		jevClassifier = ai.NewJevClient(ai.JevConfig{
			APIKey:  cfg.TypeSafe.APIKey,
			BaseURL: cfg.TypeSafe.BaseURL,
			Timeout: cfg.TypeSafe.Timeout,
		})
		go jevClassifier.KeepWarm(backgroundCtx)
		log.Println("TypeSafe AI (Jev System One) classifier enabled for fast cricket stats routing and guardrails")
	} else {
		log.Println("Notice: TypeSafe AI (Jev) disabled (missing TYPESAFE_API_KEY), queries will default to Gemini")
	}

	server := httpserver.New(httpserver.Dependencies{
		DB:           db,
		SQLGenerator: sqlGenerator,
		Classifier:   jevClassifier,
		RateLimiter:  dailyLimiter,
	})

	return &App{
		config:         cfg,
		db:             db,
		server:         server,
		stopBackground: stopBackground,
	}, nil
}

func (a *App) Port() string {
	return a.config.Port
}

func (a *App) Run() error {
	return a.server.Start(a.config.Port)
}

func (a *App) Shutdown(ctx context.Context) error {
	var errs []error
	if a.stopBackground != nil {
		a.stopBackground()
	}
	if err := a.server.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
	}
	if err := a.closeDB(); err != nil {
		errs = append(errs, fmt.Errorf("close database: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

func (a *App) closeDB() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}
