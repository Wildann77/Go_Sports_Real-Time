package schemas

import "time"

type CreateCommentaryRequest struct {
	Minute    int    `json:"minute" binding:"min=0"`
	EventType string `json:"eventType" binding:"required,max=50"`
	Message   string `json:"message" binding:"required,max=1000,non_empty_trimmed"`
	Payload   any    `json:"payload" binding:"omitempty,json_object"`
}

type CommentaryResponse struct {
	ID        int64     `json:"id"`
	MatchID   int64     `json:"matchId"`
	Minute    int       `json:"minute"`
	EventType string    `json:"eventType"`
	Message   string    `json:"message"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

type ParsedPayload struct {
	HomeScore *int `json:"homeScore"`
	AwayScore *int `json:"awayScore"`
}
