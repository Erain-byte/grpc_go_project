package model

type PermissionsModel struct {
	BaseModel
	Name   string `gorm:"size:64;not null;comment:权限名称" json:"name"`        // 用户列表
	Key    string `gorm:"size:128;unique;not null;comment:权限标识" json:"key"` // system:user_list
	Module string `gorm:"size:64;comment:所属模块" json:"module"`               // system、order、goods
	Desc   string `gorm:"size:255;comment:权限描述" json:"desc"`
	// 多对多关联角色
	Roles []RoleModel `gorm:"many2many:role_permission;" json:"roles,omitempty"`
}

func (PermissionsModel) TableName() string {
	return "permissions"
}
