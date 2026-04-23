package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	DB   DBConfig
	AI   AIConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type AIConfig struct {
	GoogleAPIKey string
	GeminiModel  string
	Timeout      time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables directly")
	}

	return &Config{
		Port: getEnv("PORT", "8080"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "boundary_bytes"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		AI: AIConfig{
			GoogleAPIKey: getEnv("GOOGLE_GENERATIVE_AI_API_KEY", ""),
			GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
			Timeout:      time.Duration(getEnvInt("AI_TIMEOUT_SECONDS", 20)) * time.Second,
		},
	}
}

func (c *Config) DBConnectionURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name, c.DB.SSLMode)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
