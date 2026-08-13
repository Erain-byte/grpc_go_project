package model

type RoleModel struct {
	BaseModel
	Name   string `gorm:"size:64;unique;not null;comment:角色名称" json:"name"` // 超级管理员、运营
	Code   string `gorm:"size:64;unique;not null;comment:角色编码" json:"code"` // super_admin, operator
	Desc   string `gorm:"size:255;comment:角色描述" json:"desc"`
	Status int8   `gorm:"default:1;comment:状态 1启用 0禁用" json:"status"`

	// 多对多：角色关联权限
	Permissions []PermissionsModel `gorm:"many2many:role_permission;" json:"permissions,omitempty"`
	// 多对多：角色关联管理员
	Admins []AdminModel `gorm:"many2many:admin_role;" json:"admins,omitempty"`
}

func (RoleModel) TableName() string {
	return "role"
}
