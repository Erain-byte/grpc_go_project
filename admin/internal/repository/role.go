package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"strings"

	"gorm.io/gorm"
)

type RoleRepository interface {
	Create(ctx context.Context, role *model.RoleModel) error
	FindByID(ctx context.Context, id uint) (*model.RoleModel, error)
	FindByCode(ctx context.Context, code string) (*model.RoleModel, error)
	FindByAdminID(ctx context.Context, adminID uint) ([]*model.RoleModel, error)
	List(ctx context.Context, offset, limit int) ([]*model.RoleModel, int64, error)
	Update(ctx context.Context, role *model.RoleModel) error
	Delete(ctx context.Context, id uint) error
	ReplacePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error
}

type roleRepository struct{ svc *svc.ServiceContext }

var _ RoleRepository = (*roleRepository)(nil)

func NewRoleRepository(svcCtx *svc.ServiceContext) RoleRepository {
	return &roleRepository{svc: svcCtx}
}

func (r *roleRepository) Create(ctx context.Context, role *model.RoleModel) error {
	if role == nil {
		return apperorr.InvalidArgument("role is nil")
	}
	return r.svc.DB.Gorm().WithContext(ctx).Omit("Permissions", "Admins").Create(role).Error
}

func (r *roleRepository) FindByID(ctx context.Context, id uint) (*model.RoleModel, error) {
	if id == 0 {
		return nil, apperorr.InvalidArgument("role ID is required")
	}
	role := new(model.RoleModel)
	err := r.svc.DB.Gorm().WithContext(ctx).Preload("Permissions").First(role, id).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *roleRepository) FindByCode(ctx context.Context, code string) (*model.RoleModel, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, apperorr.InvalidArgument("role code is required")
	}
	role := new(model.RoleModel)
	err := r.svc.DB.Gorm().WithContext(ctx).Preload("Permissions").Where("code = ?", code).First(role).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}

// FindByAdminID returns all enabled roles assigned to an administrator.
func (r *roleRepository) FindByAdminID(ctx context.Context, adminID uint) ([]*model.RoleModel, error) {
	if adminID == 0 {
		return nil, apperorr.InvalidArgument("admin ID is required")
	}

	admin := model.AdminModel{BaseModel: model.BaseModel{ID: adminID}}
	roles := make([]*model.RoleModel, 0)
	err := r.svc.DB.Gorm().
		WithContext(ctx).
		Where("status = ?", 1).
		Model(&admin).
		Association("Roles").
		Find(&roles)
	if err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepository) List(ctx context.Context, offset, limit int) ([]*model.RoleModel, int64, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, apperorr.InvalidArgument("invalid pagination parameters")
	}
	db := r.svc.DB.Gorm().WithContext(ctx)
	var total int64
	if err := db.Model(&model.RoleModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	roles := make([]*model.RoleModel, 0, limit)
	err := db.Preload("Permissions").Order("id DESC").Offset(offset).Limit(limit).Find(&roles).Error
	return roles, total, err
}

func (r *roleRepository) Update(ctx context.Context, role *model.RoleModel) error {
	if role == nil || role.ID == 0 {
		return apperorr.InvalidArgument("role with a valid ID is required")
	}
	result := r.svc.DB.Gorm().WithContext(ctx).Model(&model.RoleModel{}).
		Where("id = ?", role.ID).Select("name", "code", "desc", "status").Updates(role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *roleRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return apperorr.InvalidArgument("role ID is required")
	}
	return r.svc.DB.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		role := model.RoleModel{BaseModel: model.BaseModel{ID: id}}
		if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
			return err
		}
		if err := tx.Model(&role).Association("Admins").Clear(); err != nil {
			return err
		}
		result := tx.Delete(&role)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *roleRepository) ReplacePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	if roleID == 0 {
		return apperorr.InvalidArgument("role ID is required")
	}
	role := model.RoleModel{BaseModel: model.BaseModel{ID: roleID}}
	permissions := make([]model.PermissionsModel, 0, len(permissionIDs))
	for _, id := range permissionIDs {
		if id == 0 {
			return apperorr.InvalidArgument("permission ID is required")
		}
		permissions = append(permissions, model.PermissionsModel{BaseModel: model.BaseModel{ID: id}})
	}
	return r.svc.DB.Gorm().WithContext(ctx).Model(&role).Association("Permissions").Replace(&permissions)
}
