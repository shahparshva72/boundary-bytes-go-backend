package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
)

// defaultLeague is used when a request names no league, leagues or gender.
// It keeps today's frontend working until it sends a scope on every request.
const defaultLeague = "WPL"

// resolveLeague returns the request's league scope: a comma-separated list of
// visible league codes (see leagues.Registry.ResolveScope). Repository queries
// filter with m.league = ANY(string_to_array(scope, ',')), so a single-league
// scope such as "IPL" behaves exactly as before.
func resolveLeague(w http.ResponseWriter, r *http.Request) (string, bool) {
	query := r.URL.Query()
	scope, err := leagues.Default().ResolveScope(query.Get("league"), query.Get("leagues"), query.Get("gender"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	if scope == "" {
		return defaultLeague, true
	}

	return scope, true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
