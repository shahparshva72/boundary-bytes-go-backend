package main

import (
	"fmt"
	"log"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/config"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.New(cfg.DBConnectionURL())
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	sqlGenerator := ai.NewGeminiSQLService(ai.GeminiSQLConfig{
		APIKey:  cfg.AI.GoogleAPIKey,
		Model:   cfg.AI.GeminiModel,
		Timeout: cfg.AI.Timeout,
	})

	srv := server.NewServer(db, sqlGenerator)

	fmt.Printf("Server starting on :%s\n", cfg.Port)

	if err := srv.Start(cfg.Port); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
