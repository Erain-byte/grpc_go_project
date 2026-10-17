package model

import "time"

type UserStatus int16

const (
	UserStatusActive   UserStatus = 1
	UserStatusDisabled UserStatus = 2
)

type User struct {
	ID              int64      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	KeycloakSubject string     `gorm:"type:uuid;not null;index" json:"keycloak_subject"`
	Username        string     `gorm:"size:64;not null;index" json:"username"`
	Email           *string    `gorm:"size:255;index" json:"email,omitempty"`
	Nickname        string     `gorm:"size:128;not null;default:''" json:"nickname"`
	AvatarURL       string     `gorm:"size:512;not null;default:''" json:"avatar_url"`
	Phone           *string    `gorm:"size:32;index" json:"phone,omitempty"`
	Status          UserStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `gorm:"not null;index" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"not null" json:"updated_at"`
}

type UserIdentityIndex struct {
	KeycloakSubject string    `gorm:"type:uuid;primaryKey" json:"keycloak_subject"`
	UserID          int64     `gorm:"not null;uniqueIndex" json:"user_id"`
	TableIndex      int16     `gorm:"not null;index;check:table_index BETWEEN 0 AND 7" json:"table_index"`
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`
}

func (UserIdentityIndex) TableName() string {
	return "user_identity_index"
}
