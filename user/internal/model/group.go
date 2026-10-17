package model

import "time"

type UserGroupStatus int16

const (
	UserGroupStatusEnabled  UserGroupStatus = 1
	UserGroupStatusDisabled UserGroupStatus = 2
)

type UserGroup struct {
	ID          int64           `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Code        string          `gorm:"size:64;not null;uniqueIndex" json:"code"`
	Name        string          `gorm:"size:128;not null;index" json:"name"`
	Description string          `gorm:"size:512;not null;default:''" json:"description"`
	Status      UserGroupStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"`
	CreatedAt   time.Time       `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"not null" json:"updated_at"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}

type UserGroupMember struct {
	GroupID    int64     `gorm:"primaryKey;autoIncrement:false;index" json:"group_id"`
	UserID     int64     `gorm:"primaryKey;autoIncrement:false;index" json:"user_id"`
	TableIndex int16     `gorm:"not null;index;check:table_index BETWEEN 0 AND 7" json:"table_index"`
	JoinedAt   time.Time `gorm:"not null;index" json:"joined_at"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	Group      UserGroup `gorm:"foreignKey:GroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (UserGroupMember) TableName() string {
	return "user_group_members"
}
