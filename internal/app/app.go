package app

import (
	"fmt"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/config"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/repository/postgres"
	httpserver "github.com/shahparshva72/boundary-bytes-go-backend/internal/transport/http"
)

type App struct {
	config *config.Config
	db     postgres.Service
	server *httpserver.Server
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

	server := httpserver.New(httpserver.Dependencies{
		DB:           db,
		SQLGenerator: sqlGenerator,
	})

	return &App{
		config: cfg,
		db:     db,
		server: server,
	}, nil
}

func (a *App) Port() string {
	return a.config.Port
}

func (a *App) Run() error {
	return a.server.Start(a.config.Port)
}

func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}
