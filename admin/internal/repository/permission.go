package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"strings"

	"gorm.io/gorm"
)

type PermissionRepository interface {
	Create(ctx context.Context, permission *model.PermissionsModel) error
	FindByID(ctx context.Context, id uint) (*model.PermissionsModel, error)
	FindByKey(ctx context.Context, key string) (*model.PermissionsModel, error)
	List(ctx context.Context, offset, limit int) ([]*model.PermissionsModel, int64, error)
	Update(ctx context.Context, permission *model.PermissionsModel) error
	Delete(ctx context.Context, id uint) error
}

type permissionRepository struct{ svc *svc.ServiceContext }

var _ PermissionRepository = (*permissionRepository)(nil)

func NewPermissionRepository(svcCtx *svc.ServiceContext) PermissionRepository {
	return &permissionRepository{svc: svcCtx}
}

func (r *permissionRepository) Create(ctx context.Context, permission *model.PermissionsModel) error {
	if permission == nil {
		return apperorr.InvalidArgument("permission is nil")
	}
	return r.svc.DB.Gorm().WithContext(ctx).Omit("Roles").Create(permission).Error
}

func (r *permissionRepository) FindByID(ctx context.Context, id uint) (*model.PermissionsModel, error) {
	if id == 0 {
		return nil, apperorr.InvalidArgument("permission ID is required")
	}
	permission := new(model.PermissionsModel)
	err := r.svc.DB.Gorm().WithContext(ctx).First(permission, id).Error
	if err != nil {
		return nil, err
	}
	return permission, nil
}

func (r *permissionRepository) FindByKey(ctx context.Context, key string) (*model.PermissionsModel, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, apperorr.InvalidArgument("permission key is required")
	}
	permission := new(model.PermissionsModel)
	err := r.svc.DB.Gorm().WithContext(ctx).Where("`key` = ?", key).First(permission).Error
	if err != nil {
		return nil, err
	}
	return permission, nil
}

func (r *permissionRepository) List(ctx context.Context, offset, limit int) ([]*model.PermissionsModel, int64, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, apperorr.InvalidArgument("invalid pagination parameters")
	}
	db := r.svc.DB.Gorm().WithContext(ctx)
	var total int64
	if err := db.Model(&model.PermissionsModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	permissions := make([]*model.PermissionsModel, 0, limit)
	err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&permissions).Error
	return permissions, total, err
}

func (r *permissionRepository) Update(ctx context.Context, permission *model.PermissionsModel) error {
	if permission == nil || permission.ID == 0 {
		return apperorr.InvalidArgument("permission with a valid ID is required")
	}
	result := r.svc.DB.Gorm().WithContext(ctx).Model(&model.PermissionsModel{}).
		Where("id = ?", permission.ID).Select("name", "key", "module", "desc").Updates(permission)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *permissionRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return apperorr.InvalidArgument("permission ID is required")
	}
	return r.svc.DB.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permission := model.PermissionsModel{BaseModel: model.BaseModel{ID: id}}
		if err := tx.Model(&permission).Association("Roles").Clear(); err != nil {
			return err
		}
		result := tx.Delete(&permission)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
