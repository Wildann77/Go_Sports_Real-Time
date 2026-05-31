package models

import (
	"gorm.io/datatypes"
	"time"
)

type Match struct {
	ID        int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	Sport     string         `json:"sport" gorm:"not null"`
	HomeTeam  string         `json:"homeTeam" gorm:"not null"`
	AwayTeam  string         `json:"awayTeam" gorm:"not null"`
	HomeScore int            `json:"homeScore" gorm:"not null;default:0"`
	AwayScore int            `json:"awayScore" gorm:"not null;default:0"`
	Status    string         `json:"status" gorm:"not null"`
	StartTime time.Time      `json:"startTime" gorm:"not null"`
	EndTime   time.Time      `json:"endTime" gorm:"not null"`
	Metadata  datatypes.JSON `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Match) TableName() string {
	return "matches"
}
