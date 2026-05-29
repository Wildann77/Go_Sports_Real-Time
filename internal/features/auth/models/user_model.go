package models

import "time"

type User struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Email        string    `json:"email" gorm:"not null;uniqueIndex"`
	Name         string    `json:"name" gorm:"not null"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;not null"`
	TokenVersion int       `json:"-" gorm:"column:token_version;not null;default:0"`
	CreatedAt    time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}
