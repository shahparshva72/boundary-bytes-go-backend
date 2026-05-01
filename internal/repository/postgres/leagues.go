package postgres

import (
	"context"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) GetLeagueConfigStats(ctx context.Context) ([]models.LeagueConfigItem, error) {
	query := `
		WITH match_stats AS (
			SELECT
				league,
				COUNT(DISTINCT match_id)::int AS matches,
				STRING_AGG(DISTINCT season, ',' ORDER BY season) AS seasons
			FROM wpl_match
			GROUP BY league
		),
		team_stats AS (
			SELECT
				mi.league,
				COUNT(DISTINCT CASE
					WHEN t.team_name = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
					WHEN t.team_name = 'Delhi Daredevils' THEN 'Delhi Capitals'
					WHEN t.team_name = 'Kings XI Punjab' THEN 'Punjab Kings'
					WHEN t.team_name = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
					ELSE t.team_name
				END)::int AS teams
			FROM wpl_team t
			JOIN wpl_match_info mi ON mi.match_id = t.match_id
			GROUP BY mi.league
		),
		player_stats AS (
			SELECT
				mi.league,
				COUNT(DISTINCT p.player_name)::int AS players
			FROM wpl_player p
			JOIN wpl_match_info mi ON mi.match_id = p.match_id
			GROUP BY mi.league
		)
		SELECT
			m.league,
			COALESCE(t.teams, 0) AS teams,
			m.matches,
			COALESCE(p.players, 0) AS players,
			m.seasons
		FROM match_stats m
		LEFT JOIN team_stats t ON t.league = m.league
		LEFT JOIN player_stats p ON p.league = m.league
		ORDER BY m.league;
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []models.LeagueConfigItem
	for rows.Next() {
		var item models.LeagueConfigItem
		var seasons string
		if err := rows.Scan(
			&item.League,
			&item.Stats.Teams,
			&item.Stats.Matches,
			&item.Stats.Players,
			&seasons,
		); err != nil {
			return nil, err
		}
		if seasons != "" {
			item.Stats.Seasons = strings.Split(seasons, ",")
		}
		configs = append(configs, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return configs, nil
}
