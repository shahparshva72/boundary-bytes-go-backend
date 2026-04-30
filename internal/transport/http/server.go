package httpserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/repository/postgres"
	advancedstatsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/advancedstats"
	aifeedbackservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/aifeedback"
	healthservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/health"
	leaguesservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/leagues"
	matchesservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/matches"
	playersservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/players"
	statsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/stats"
	statsexplorerservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/statsexplorer"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/texttosql"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/transport/http/handlers"
)

type Dependencies struct {
	DB           postgres.Service
	SQLGenerator ai.SQLGenerator
}

type Server struct {
	httpServer    *http.Server
	Router        *chi.Mux
	DB            postgres.Service
	SQLGenerator  ai.SQLGenerator
}

func New(deps Dependencies) *Server {
	r := chi.NewRouter()
	advancedStatsService := advancedstatsservice.New(deps.DB)
	aiFeedbackService := aifeedbackservice.New(deps.DB)
	healthService := healthservice.New(deps.DB)
	leaguesService := leaguesservice.New(deps.DB)
	matchesService := matchesservice.New(deps.DB)
	playersService := playersservice.New(deps.DB)
	statsService := statsservice.New(deps.DB)
	statsExplorerService := statsexplorerservice.New(deps.DB)
	textToSQLService := texttosql.New(deps.DB, deps.SQLGenerator)

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.NotFound(handlers.NotFoundHandler)

	r.Get("/health", handlers.HealthCheck)
	r.Get("/db-health", handlers.DBHealthCheck(healthService))
	r.Get("/api/players/batters", handlers.GetBatters(playersService))
	r.Get("/api/players/bowlers", handlers.GetBowlers(playersService))
	r.Get("/api/leagues/config", handlers.GetLeagueConfigs(leaguesService))
	r.Get("/api/stats/seasons", handlers.GetSeasons(matchesService))
	r.Get("/api/stats/latest-match-date", handlers.GetLatestMatchDate(matchesService))
	r.Get("/api/matches/list", handlers.GetMatchList(matchesService))
	r.Get("/api/matches", handlers.GetMatches(matchesService))
	r.Get("/api/stats/team-wins", handlers.GetTeamWins(matchesService))
	r.Get("/api/stats/team-averages", handlers.GetTeamAverages(matchesService))
	r.Get("/api/stats/runrate-trend", handlers.GetRunRateTrend(statsService))
	r.Get("/api/stats/team-runrate-progression", handlers.GetTeamRunRateProgression(statsService))
	r.Get("/api/stats/bowling-wicket-types", handlers.GetBowlingWicketTypes(advancedStatsService))
	r.Get("/api/stats/matchup", handlers.GetMatchup(statsService))
	r.Get("/api/stats/multi-matchup", handlers.GetMultiMatchup(advancedStatsService))
	r.Get("/api/stats/player-compare", handlers.GetPlayerCompare(statsService))
	r.Get("/api/stats/player-progression", handlers.GetPlayerProgression(advancedStatsService))
	r.Get("/api/stats/stat-explorer/options", handlers.GetStatExplorerOptions(statsExplorerService))
	r.Post("/api/stats/stat-explorer/run", handlers.RunStatExplorer(statsExplorerService))
	r.Get("/api/stats/advanced", handlers.GetAdvancedStats(advancedStatsService))
	r.Get("/api/stats/fall-of-wickets/{matchId}", handlers.GetFallOfWickets(advancedStatsService))
	r.Get("/api/stats/leading-wicket-takers", handlers.GetLeadingWicketTakers(statsService))
	r.Get("/api/stats/leading-run-scorers", handlers.GetLeadingRunScorers(statsService))
	r.Get("/api/news", handlers.GetNews)
	r.Get("/api/ai/feedback", handlers.GetAIFeedbackStats(aiFeedbackService))
	r.Post("/api/ai/feedback", handlers.SubmitAIFeedback(aiFeedbackService))
	r.Post("/api/text-to-sql", handlers.TextToSQL(textToSQLService))

	return &Server{
		Router:       r,
		DB:           deps.DB,
		SQLGenerator: deps.SQLGenerator,
	}
}

func (s *Server) Start(port string) error {
	s.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: s.Router,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-League")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
