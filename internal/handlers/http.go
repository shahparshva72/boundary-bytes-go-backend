package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const defaultLeague = "WPL"

var validLeagues = []string{"WPL", "IPL", "BBL", "WBBL", "SA20"}

var validLeagueSet = map[string]struct{}{
	"WPL":  {},
	"IPL":  {},
	"BBL":  {},
	"WBBL": {},
	"SA20": {},
}

func resolveLeague(w http.ResponseWriter, r *http.Request) (string, bool) {
	league := strings.TrimSpace(r.URL.Query().Get("league"))
	if league == "" {
		return defaultLeague, true
	}

	if _, ok := validLeagueSet[league]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid league: %s. Valid leagues are: %s", league, strings.Join(validLeagues, ", ")))
		return "", false
	}

	return league, true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
