package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/handlers"
)

type Server struct {
	Router *chi.Mux
	DB     database.Service
}

func NewServer(db database.Service) *Server {
	r := chi.NewRouter()

	// Default chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.NotFound(handlers.NotFoundHandler)

	r.Get("/health", handlers.HealthCheck)
	r.Get("/db-health", handlers.DBHealthCheck(db))

	return &Server{
		Router: r,
		DB:     db,
	}
}

func (s *Server) Start(port string) error {
	return http.ListenAndServe(":"+port, s.Router)
}
