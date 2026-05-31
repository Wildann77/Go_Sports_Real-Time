package schemas

import "time"

type CreateMatchRequest struct {
	Sport     string    `json:"sport" binding:"required,max=50,safe_slug"`
	HomeTeam  string    `json:"homeTeam" binding:"required,max=100,non_empty_trimmed"`
	AwayTeam  string    `json:"awayTeam" binding:"required,max=100,non_empty_trimmed"`
	StartTime time.Time `json:"startTime" binding:"required"`
	EndTime   time.Time `json:"endTime" binding:"required"`
	HomeScore int       `json:"homeScore" binding:"min=0"`
	AwayScore int       `json:"awayScore" binding:"min=0"`
	Metadata  any       `json:"metadata" binding:"omitempty,json_object"`
}

type MatchResponse struct {
	ID        int64     `json:"id"`
	Sport     string    `json:"sport"`
	HomeTeam  string    `json:"homeTeam"`
	AwayTeam  string    `json:"awayTeam"`
	HomeScore int       `json:"homeScore"`
	AwayScore int       `json:"awayScore"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Metadata  any       `json:"metadata"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
