package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) InsertDailyDraftScore(ctx context.Context, params models.SubmitDailyDraftScoreParams) error {
	lineupJSON, err := json.Marshal(params.Lineup)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO daily_draft_score (id, device_id, league, play_date, score, optimal_score, lineup)
		VALUES ($1, $2, $3, $4::date, $5, $6, $7)
	`

	_, err = s.db.ExecContext(
		ctx,
		query,
		newDailyDraftScoreID(),
		params.DeviceID,
		params.League,
		params.Date,
		params.Score,
		params.OptimalScore,
		string(lineupJSON),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDailyDraftAlreadySubmitted
		}
		return err
	}

	return nil
}

func (s *service) GetDailyDraftLeaderboard(ctx context.Context, league, date, deviceID string, topN int) (models.DailyDraftLeaderboardResponse, error) {
	if topN <= 0 {
		topN = 10
	}

	countQuery := `
		SELECT COUNT(*)::int
		FROM daily_draft_score
		WHERE league = $1 AND play_date = $2::date
	`
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, league, date).Scan(&total); err != nil {
		return models.DailyDraftLeaderboardResponse{}, err
	}

	topQuery := `
		SELECT score, device_id
		FROM daily_draft_score
		WHERE league = $1 AND play_date = $2::date
		ORDER BY score DESC, created_at ASC
		LIMIT $3
	`
	rows, err := s.db.QueryContext(ctx, topQuery, league, date, topN)
	if err != nil {
		return models.DailyDraftLeaderboardResponse{}, err
	}
	defer rows.Close()

	topScores := make([]models.DailyDraftLeaderboardEntry, 0, topN)
	rank := 1
	for rows.Next() {
		var score float64
		var entryDeviceID string
		if err := rows.Scan(&score, &entryDeviceID); err != nil {
			return models.DailyDraftLeaderboardResponse{}, err
		}
		topScores = append(topScores, models.DailyDraftLeaderboardEntry{
			Rank:  rank,
			Score: score,
			IsYou: entryDeviceID == deviceID,
		})
		rank++
	}
	if err := rows.Err(); err != nil {
		return models.DailyDraftLeaderboardResponse{}, err
	}

	var yourRank *int
	var yourScore *float64

	if deviceID != "" {
		userQuery := `
			SELECT score
			FROM daily_draft_score
			WHERE league = $1 AND play_date = $2::date AND device_id = $3
		`
		var score float64
		err := s.db.QueryRowContext(ctx, userQuery, league, date, deviceID).Scan(&score)
		if err == nil {
			yourScore = &score
			rankQuery := `
				SELECT COUNT(*)::int + 1
				FROM daily_draft_score
				WHERE league = $1 AND play_date = $2::date AND score > $3
			`
			var rankValue int
			if err := s.db.QueryRowContext(ctx, rankQuery, league, date, score).Scan(&rankValue); err == nil {
				yourRank = &rankValue
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return models.DailyDraftLeaderboardResponse{}, err
		}
	}

	return models.DailyDraftLeaderboardResponse{
		League:       league,
		Date:         date,
		TotalPlayers: total,
		YourRank:     yourRank,
		YourScore:    yourScore,
		TopScores:    topScores,
	}, nil
}

var ErrDailyDraftAlreadySubmitted = errors.New("draft score already submitted for this date")

func newDailyDraftScoreID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("draft_%d", time.Now().UnixNano())
	}
	return "draft_" + hex.EncodeToString(buffer)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}
