package models

import "time"

type RefreshSession struct {
	ID         int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int64      `json:"userId" gorm:"column:user_id;not null;index"`
	TokenHash  string     `json:"-" gorm:"column:token_hash;not null"`
	JTI        string     `json:"jti" gorm:"not null;uniqueIndex"`
	FamilyID   string     `json:"familyId" gorm:"column:family_id;not null;index"`
	ExpiresAt  time.Time  `json:"expiresAt" gorm:"column:expires_at;not null"`
	RevokedAt  *time.Time `json:"revokedAt" gorm:"column:revoked_at"`
	ReplacedBy *int64     `json:"replacedBy" gorm:"column:replaced_by"`
	UserAgent  string     `json:"userAgent" gorm:"column:user_agent"`
	IPAddress  string     `json:"ipAddress" gorm:"column:ip_address"`
	CreatedAt  time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (RefreshSession) TableName() string {
	return "refresh_sessions"
}
