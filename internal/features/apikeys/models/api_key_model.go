package models

import "time"

type APIKey struct {
	ID          int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      int64      `json:"userId" gorm:"column:user_id;not null;index"`
	Name        string     `json:"name" gorm:"not null"`
	KeyPrefix   string     `json:"keyPrefix" gorm:"column:key_prefix;not null"`
	KeyHash     string     `json:"-" gorm:"column:key_hash;not null;uniqueIndex"`
	KeyLastFour string     `json:"keyLastFour" gorm:"column:key_last_four;not null"`
	Scopes      []string   `json:"scopes" gorm:"type:jsonb;serializer:json;not null"`
	LastUsedAt  *time.Time `json:"lastUsedAt" gorm:"column:last_used_at"`
	ExpiresAt   *time.Time `json:"expiresAt" gorm:"column:expires_at"`
	RevokedAt   *time.Time `json:"revokedAt" gorm:"column:revoked_at"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (APIKey) TableName() string {
	return "api_keys"
}
