CREATE TABLE daily_draft_score (
    id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    league TEXT NOT NULL,
    play_date DATE NOT NULL,
    score DOUBLE PRECISION NOT NULL,
    optimal_score DOUBLE PRECISION NOT NULL,
    lineup TEXT NOT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT daily_draft_score_pkey PRIMARY KEY (id),
    CONSTRAINT daily_draft_score_device_league_date_key UNIQUE (device_id, league, play_date)
);

CREATE INDEX daily_draft_score_league_date_idx ON daily_draft_score (league, play_date);
CREATE INDEX daily_draft_score_league_date_score_idx ON daily_draft_score (league, play_date, score DESC);
