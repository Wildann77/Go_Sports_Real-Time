CREATE TABLE IF NOT EXISTS matches (
    id BIGSERIAL PRIMARY KEY,
    sport TEXT NOT NULL,
    home_team TEXT NOT NULL,
    away_team TEXT NOT NULL,
    home_score INTEGER NOT NULL DEFAULT 0 CHECK (home_score >= 0),
    away_score INTEGER NOT NULL DEFAULT 0 CHECK (away_score >= 0),
    status TEXT NOT NULL CHECK (status IN ('scheduled', 'live', 'finished')),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_matches_status ON matches(status);
CREATE INDEX idx_matches_start_time ON matches(start_time);
CREATE INDEX idx_matches_created_at ON matches(created_at);
