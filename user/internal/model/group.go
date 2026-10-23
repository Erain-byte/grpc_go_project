package model

import "time"

// UserGroupStatus 表示用户分组是否可用。
type UserGroupStatus int16

const (
	UserGroupStatusEnabled  UserGroupStatus = 1 // 分组启用。
	UserGroupStatusDisabled UserGroupStatus = 2 // 分组停用。
)

// UserGroup 保存用户分组的定义，例如普通用户组、VIP 组或运营分组。
type UserGroup struct {
	ID          int64           `gorm:"primaryKey;autoIncrement:false" json:"id"`                     // 雪花算法生成的分组 ID。
	Code        string          `gorm:"size:64;not null;uniqueIndex" json:"code"`                     // 稳定且唯一的业务编码。
	Name        string          `gorm:"size:128;not null;index" json:"name"`                          // 分组显示名称。
	Description string          `gorm:"size:512;not null;default:''" json:"description"`              // 分组用途说明。
	Status      UserGroupStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"` // 分组状态。
	CreatedAt   time.Time       `gorm:"not null" json:"created_at"`                                   // 创建时间。
	UpdatedAt   time.Time       `gorm:"not null" json:"updated_at"`                                   // 最后更新时间。
}

func (UserGroup) TableName() string {
	return "user_groups"
}

type UserGroupMember struct {
	GroupID    int64     `gorm:"primaryKey;autoIncrement:false;index" json:"group_id"`                                   // 用户分组 ID。
	UserID     int64     `gorm:"primaryKey;autoIncrement:false;index" json:"user_id"`                                    // 本地用户 ID；与 GroupID 组成联合主键。
	TableIndex int16     `gorm:"not null;index;check:table_index BETWEEN 0 AND 7" json:"table_index"`                    // 用户所在分表编号，避免再次计算或查询路由表。
	JoinedAt   time.Time `gorm:"not null;index" json:"joined_at"`                                                        // 用户加入分组的业务时间。
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`                                                             // 关系记录创建时间。
	Group      UserGroup `gorm:"foreignKey:GroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"` // GORM 分组关联，不输出到 JSON。
}

func (UserGroupMember) TableName() string {
	return "user_group_members"
}
