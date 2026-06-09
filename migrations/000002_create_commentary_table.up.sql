CREATE TABLE IF NOT EXISTS commentary (
    id BIGSERIAL PRIMARY KEY,
    match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    minute INTEGER CHECK (minute >= 0),
    event_type TEXT NOT NULL,
    message TEXT NOT NULL,
    payload JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_commentary_match_id ON commentary(match_id);
CREATE INDEX idx_commentary_created_at ON commentary(created_at);
CREATE INDEX idx_commentary_event_type ON commentary(event_type);
