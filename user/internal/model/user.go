package model

import "time"

// UserStatus 表示本地用户账号状态。
type UserStatus int16

const (
	UserStatusActive   UserStatus = 1 // 正常，可以使用业务功能。
	UserStatusDisabled UserStatus = 2 // 已禁用，拒绝业务操作。
)

// User 保存用户的本地业务资料。
// 实际数据按照 ID 路由到 users_00 至 users_07，因此不定义固定 TableName。
type User struct {
	ID              int64      `gorm:"primaryKey;autoIncrement:false" json:"id"`                     // 雪花算法生成的本地用户 ID，同时用于分片路由。
	KeycloakSubject string     `gorm:"type:uuid;not null;index" json:"keycloak_subject"`             // Keycloak Token 中的 sub，关联统一认证身份。
	Username        string     `gorm:"size:64;not null;index" json:"username"`                       // 登录身份对应的用户名快照。
	Email           *string    `gorm:"size:255;index" json:"email,omitempty"`                        // 邮箱；Keycloak 未提供时允许为空。
	Nickname        string     `gorm:"size:128;not null;default:''" json:"nickname"`                 // 业务系统内显示的昵称。
	AvatarURL       string     `gorm:"size:512;not null;default:''" json:"avatar_url"`               // 用户头像地址。
	Phone           *string    `gorm:"size:32;index" json:"phone,omitempty"`                         // 手机号；未绑定时允许为空。
	Status          UserStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"` // 本地业务账号状态。
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`                                      // 最近一次成功登录时间。
	CreatedAt       time.Time  `gorm:"not null;index" json:"created_at"`                             // 本地用户首次初始化时间。
	UpdatedAt       time.Time  `gorm:"not null" json:"updated_at"`                                   // 本地资料最后更新时间。
}

// UserIdentityIndex 全局索引表。

type UserIdentityIndex struct {
	KeycloakSubject string    `gorm:"type:uuid;primaryKey" json:"keycloak_subject"`                        // Keycloak 用户唯一标识，也是用户初始化的幂等键。
	UserID          int64     `gorm:"not null;uniqueIndex" json:"user_id"`                                 // 对应的本地雪花用户 ID。
	TableIndex      int16     `gorm:"not null;index;check:table_index BETWEEN 0 AND 7" json:"table_index"` // 用户分表编号，范围为 0～7。
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`                                          // 身份映射建立时间。
}

func (UserIdentityIndex) TableName() string {
	return "user_identity_index"
}
