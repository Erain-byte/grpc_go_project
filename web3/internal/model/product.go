package model

import "time"

// ClientEventType 与 Proto 的匿名事件枚举保持一致。
type ClientEventType int16

// ClientEvent 只记录产品漏斗需要的最小信息。
// InstallationHash 是随机安装标识的 SHA-256 摘要，不保存原始安装标识或钱包地址。
type ClientEvent struct {
	ID               string          `gorm:"type:uuid;primaryKey" json:"id"`
	InstallationHash string          `gorm:"type:char(64);not null;index:idx_client_events_installation_time,priority:1" json:"installation_hash"`
	EventType        ClientEventType `gorm:"not null;index:idx_client_events_type_time,priority:1;check:event_type BETWEEN 1 AND 11" json:"event_type"`
	AppVersion       string          `gorm:"size:32;not null;default:''" json:"app_version"`
	Platform         int16           `gorm:"not null;check:platform BETWEEN 1 AND 3" json:"platform"`
	Network          string          `gorm:"size:32;not null;default:''" json:"network"`
	ResultType       string          `gorm:"size:32;not null;default:''" json:"result_type"`
	DurationBucket   int16           `gorm:"not null;default:0;check:duration_bucket BETWEEN 0 AND 5" json:"duration_bucket"`
	OccurredAt       time.Time       `gorm:"not null;index:idx_client_events_installation_time,priority:2;index:idx_client_events_type_time,priority:2" json:"occurred_at"`
	CreatedAt        time.Time       `gorm:"not null" json:"created_at"`
}

func (ClientEvent) TableName() string { return "client_events" }

type FeedbackStatus int16

const (
	FeedbackStatusPending    FeedbackStatus = 1
	FeedbackStatusProcessing FeedbackStatus = 2
	FeedbackStatusResolved   FeedbackStatus = 3
	FeedbackStatusClosed     FeedbackStatus = 4
)

// FeedbackItem 保存用户主动提交的问题描述，不关联登录账号和钱包地址。
type FeedbackItem struct {
	ID               string         `gorm:"type:uuid;primaryKey" json:"id"`
	InstallationHash string         `gorm:"type:char(64);not null;uniqueIndex:uk_feedback_installation_issue,priority:1" json:"installation_hash"`
	IssueID          string         `gorm:"size:64;not null;uniqueIndex:uk_feedback_installation_issue,priority:2" json:"issue_id"`
	AppVersion       string         `gorm:"size:32;not null;default:''" json:"app_version"`
	Platform         int16          `gorm:"not null;check:platform BETWEEN 1 AND 3" json:"platform"`
	Description      string         `gorm:"type:text;not null" json:"description"`
	Status           FeedbackStatus `gorm:"not null;default:1;index:idx_feedback_status_created,priority:1;check:status BETWEEN 1 AND 4" json:"status"`
	CreatedAt        time.Time      `gorm:"not null;index:idx_feedback_status_created,priority:2" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"not null" json:"updated_at"`
}

func (FeedbackItem) TableName() string { return "feedback_items" }
