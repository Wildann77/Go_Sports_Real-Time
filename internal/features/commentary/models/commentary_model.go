package models

import (
	"gorm.io/datatypes"
	"time"
)

type Commentary struct {
	ID        int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	MatchID   int64          `json:"matchId" gorm:"not null;index"`
	Minute    int            `json:"minute"`
	EventType string         `json:"eventType" gorm:"not null"`
	Message   string         `json:"message" gorm:"not null"`
	Payload   datatypes.JSON `json:"payload" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime"`
}

func (Commentary) TableName() string {
	return "commentary"
}
