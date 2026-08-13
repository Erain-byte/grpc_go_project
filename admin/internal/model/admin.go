package model

// AdminStatus 表示管理员账号状态。
type AdminStatus int8

const (
	AdminStatusDisabled AdminStatus = 0
	AdminStatusEnabled  AdminStatus = 1
)

// AdminModel 对应管理员表。
type AdminModel struct {
	BaseModel
	Username     string      `gorm:"size:32;uniqueIndex;not null;comment:登录账号" json:"username"`
	PasswordHash string      `gorm:"size:255;not null;comment:密码哈希" json:"-"`
	Nickname     string      `gorm:"size:32;comment:昵称" json:"nickname"`
	Avatar       string      `gorm:"size:255;comment:头像地址" json:"avatar"`
	Phone        *string     `gorm:"size:32;uniqueIndex;comment:手机号" json:"phone,omitempty"`
	Email        *string     `gorm:"size:255;uniqueIndex;comment:邮箱" json:"email,omitempty"`
	Status       AdminStatus `gorm:"not null;default:1;index;comment:1正常 0禁用" json:"status"`

	// 一个管理员可以绑定多个角色，也可以在多个设备上保持登录会话。
	Roles    []RoleModel         `gorm:"many2many:admin_roles;" json:"roles,omitempty"`
	Sessions []AdminSessionModel `gorm:"foreignKey:AdminID;constraint:OnDelete:CASCADE" json:"-"`
}

func (AdminModel) TableName() string {
	return "admins"
}

// 管理员账号是否启用
func (a AdminModel) IsEnabled() bool {
	return a.Status == AdminStatusEnabled
}
