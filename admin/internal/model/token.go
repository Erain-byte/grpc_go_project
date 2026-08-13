package model

import "time"

// SessionStatus represents the state of an administrator login session.
type SessionStatus int8

const (
	SessionStatusRevoked SessionStatus = 0
	SessionStatusActive  SessionStatus = 1
)

// AdminSessionModel stores an administrator login session.
// Access tokens are not persisted; only the refresh-token hash is stored.
type AdminSessionModel struct {
	BaseModel
	AdminID          uint          `gorm:"not null;index:idx_admin_session;comment:关联管理员" json:"admin_id"`
	SessionID        string        `gorm:"size:64;uniqueIndex;not null;comment:会话标识" json:"session_id"`
	RefreshTokenHash string        `gorm:"size:64;uniqueIndex;not null;comment:刷新令牌哈希" json:"-"`
	ExpiresAt        time.Time     `gorm:"not null;index;comment:会话过期时间" json:"expires_at"`
	RevokedAt        *time.Time    `gorm:"index;comment:会话撤销时间" json:"revoked_at,omitempty"`
	Status           SessionStatus `gorm:"not null;default:1;index;comment:1有效 0撤销" json:"status"`
	Device           string        `gorm:"size:255;comment:登录设备" json:"device"`
	LoginIP          string        `gorm:"size:45;comment:登录IP" json:"login_ip"`

	Admin AdminModel `gorm:"foreignKey:AdminID;references:ID" json:"-"`
}

func (AdminSessionModel) TableName() string {
	return "admin_sessions"
}

// IsValid reports whether the session can still be used at now.
func (s AdminSessionModel) IsValid(now time.Time) bool {
	return !s.IsRevoked() && now.Before(s.ExpiresAt)
}

// IsRevoked only checks the in-memory model and does not access the database.
func (s AdminSessionModel) IsRevoked() bool {
	return s.Status == SessionStatusRevoked || s.RevokedAt != nil
}
