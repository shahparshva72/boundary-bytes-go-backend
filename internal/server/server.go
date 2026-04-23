package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/handlers"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-League")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type Server struct {
	Router       *chi.Mux
	DB           database.Service
	SQLGenerator ai.SQLGenerator
}

func NewServer(db database.Service, sqlGenerator ai.SQLGenerator) *Server {
	r := chi.NewRouter()

	// Default chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.NotFound(handlers.NotFoundHandler)

	r.Get("/health", handlers.HealthCheck)
	r.Get("/db-health", handlers.DBHealthCheck(db))
	r.Get("/api/players/batters", handlers.GetBatters(db))
	r.Get("/api/players/bowlers", handlers.GetBowlers(db))
	r.Get("/api/stats/seasons", handlers.GetSeasons(db))
	r.Get("/api/stats/latest-match-date", handlers.GetLatestMatchDate(db))
	r.Get("/api/matches/list", handlers.GetMatchList(db))
	r.Get("/api/matches", handlers.GetMatches(db))
	r.Get("/api/stats/team-wins", handlers.GetTeamWins(db))
	r.Get("/api/stats/team-averages", handlers.GetTeamAverages(db))
	r.Get("/api/stats/runrate-trend", handlers.GetRunRateTrend(db))
	r.Get("/api/stats/team-runrate-progression", handlers.GetTeamRunRateProgression(db))
	r.Get("/api/stats/bowling-wicket-types", handlers.GetBowlingWicketTypes(db))
	r.Get("/api/stats/matchup", handlers.GetMatchup(db))
	r.Get("/api/stats/multi-matchup", handlers.GetMultiMatchup(db))
	r.Get("/api/stats/player-compare", handlers.GetPlayerCompare(db))
	r.Get("/api/stats/player-progression", handlers.GetPlayerProgression(db))
	r.Get("/api/stats/stat-explorer/options", handlers.GetStatExplorerOptions(db))
	r.Post("/api/stats/stat-explorer/run", handlers.RunStatExplorer(db))
	r.Get("/api/stats/advanced", handlers.GetAdvancedStats(db))
	r.Get("/api/stats/fall-of-wickets/{matchId}", handlers.GetFallOfWickets(db))
	r.Get("/api/stats/leading-wicket-takers", handlers.GetLeadingWicketTakers(db))
	r.Get("/api/stats/leading-run-scorers", handlers.GetLeadingRunScorers(db))
	r.Get("/api/news", handlers.GetNews)
	r.Get("/api/ai/feedback", handlers.GetAIFeedbackStats(db))
	r.Post("/api/ai/feedback", handlers.SubmitAIFeedback(db))
	r.Post("/api/text-to-sql", handlers.TextToSQL(db, sqlGenerator))

	return &Server{
		Router:       r,
		DB:           db,
		SQLGenerator: sqlGenerator,
	}
}

func (s *Server) Start(port string) error {
	return http.ListenAndServe(":"+port, s.Router)
}
