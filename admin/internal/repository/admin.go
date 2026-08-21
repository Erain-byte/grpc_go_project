package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"strings"

	"gorm.io/gorm"
)

type AdminRepository interface {
	// CreateAdmin creates a new admin
	Create(ctx context.Context, admin *model.AdminModel) error
	FindByID(ctx context.Context, id uint) (*model.AdminModel, error)
	FindByUsername(ctx context.Context, username string) (*model.AdminModel, error)
	List(ctx context.Context, offset, limit int) ([]*model.AdminModel, int64, error)
	Update(ctx context.Context, admin *model.AdminModel) error
	Delete(ctx context.Context, id uint) error
	ReplaceRoles(ctx context.Context, adminID uint, roleIDs []uint) error
}

type adminRepository struct {
	svc *svc.ServiceContext
}

// 编译期检查 *adminRepository 是否实现了 AdminRepository 的全部方法。
var _ AdminRepository = (*adminRepository)(nil)

func NewAdminRepository(svc *svc.ServiceContext) AdminRepository {
	return &adminRepository{svc: svc}
}

func (r *adminRepository) Create(ctx context.Context, admin *model.AdminModel) error {
	if admin == nil {
		return apperorr.InvalidArgument("admin is nil")
	}
	return r.svc.DB.Gorm().
		WithContext(ctx).
		Omit("Roles", "Sessions").
		Create(admin).
		Error
}

func (r *adminRepository) FindByID(ctx context.Context, id uint) (*model.AdminModel, error) {
	if id == 0 {
		return nil, apperorr.InvalidArgument("admin ID is required")
	}

	admin := new(model.AdminModel)
	err := r.svc.DB.Gorm().
		WithContext(ctx).
		Preload("Roles").
		First(admin, id).
		Error
	if err != nil {
		return nil, err
	}
	return admin, nil
}

func (r *adminRepository) FindByUsername(ctx context.Context, username string) (*model.AdminModel, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, apperorr.InvalidArgument("admin username is required")
	}

	admin := new(model.AdminModel)
	err := r.svc.DB.Gorm().
		WithContext(ctx).
		Preload("Roles").
		Where("username = ?", username).
		First(admin).
		Error
	if err != nil {
		return nil, err
	}
	return admin, nil
}

func (r *adminRepository) List(ctx context.Context, offset, limit int) ([]*model.AdminModel, int64, error) {
	if offset < 0 {
		return nil, 0, apperorr.InvalidArgument("offset cannot be negative")
	}
	if limit < 1 || limit > 100 {
		return nil, 0, apperorr.InvalidArgument("limit must be between 1 and 100")
	}

	db := r.svc.DB.Gorm().WithContext(ctx)
	var total int64
	if err := db.Model(&model.AdminModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	admins := make([]*model.AdminModel, 0, limit)
	err := db.
		Preload("Roles").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&admins).
		Error
	if err != nil {
		return nil, 0, err
	}
	return admins, total, nil
}

func (r *adminRepository) Update(ctx context.Context, admin *model.AdminModel) error {
	if admin == nil || admin.ID == 0 {
		return apperorr.InvalidArgument("admin with a valid ID is required")
	}

	result := r.svc.DB.Gorm().
		WithContext(ctx).
		Model(&model.AdminModel{}).
		Where("id = ?", admin.ID).
		Select(
			"username",
			"password_hash",
			"nickname",
			"avatar",
			"phone",
			"email",
			"status",
		).
		Updates(admin)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *adminRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return apperorr.InvalidArgument("admin ID is required")
	}

	result := r.svc.DB.Gorm().
		WithContext(ctx).
		Delete(&model.AdminModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *adminRepository) ReplaceRoles(ctx context.Context, adminID uint, roleIDs []uint) error {
	if adminID == 0 {
		return apperorr.InvalidArgument("admin ID is required")
	}
	admin := model.AdminModel{BaseModel: model.BaseModel{ID: adminID}}
	roles := make([]model.RoleModel, 0, len(roleIDs))
	for _, id := range roleIDs {
		if id == 0 {
			return apperorr.InvalidArgument("role ID is required")
		}
		roles = append(roles, model.RoleModel{BaseModel: model.BaseModel{ID: id}})
	}
	return r.svc.DB.Gorm().WithContext(ctx).Model(&admin).Association("Roles").Replace(&roles)
}
