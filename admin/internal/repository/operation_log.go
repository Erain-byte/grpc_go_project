package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"time"
)

type OperationLogFilter struct {
	AdminID  uint
	Module   string
	OperType string
	Status   *int8
	StartAt  *time.Time
	EndAt    *time.Time
}

type OperationLogRepository interface {
	Create(ctx context.Context, log *model.OperationLogModel) error
	FindByID(ctx context.Context, id uint) (*model.OperationLogModel, error)
	List(ctx context.Context, filter OperationLogFilter, offset, limit int) ([]*model.OperationLogModel, int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type operationLogRepository struct{ svc *svc.ServiceContext }

var _ OperationLogRepository = (*operationLogRepository)(nil)

func NewOperationLogRepository(svcCtx *svc.ServiceContext) OperationLogRepository {
	return &operationLogRepository{svc: svcCtx}
}

func (r *operationLogRepository) Create(ctx context.Context, log *model.OperationLogModel) error {
	if log == nil {
		return apperorr.InvalidArgument("operation log is nil")
	}
	return r.svc.DB.Gorm().WithContext(ctx).Omit("Admin").Create(log).Error
}

func (r *operationLogRepository) FindByID(ctx context.Context, id uint) (*model.OperationLogModel, error) {
	if id == 0 {
		return nil, apperorr.InvalidArgument("operation log ID is required")
	}
	log := new(model.OperationLogModel)
	err := r.svc.DB.Gorm().WithContext(ctx).Preload("Admin").First(log, id).Error
	if err != nil {
		return nil, err
	}
	return log, nil
}

func (r *operationLogRepository) List(ctx context.Context, filter OperationLogFilter, offset, limit int) ([]*model.OperationLogModel, int64, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, apperorr.InvalidArgument("invalid pagination parameters")
	}
	db := r.svc.DB.Gorm().WithContext(ctx).Model(&model.OperationLogModel{})
	if filter.AdminID != 0 {
		db = db.Where("admin_id = ?", filter.AdminID)
	}
	if filter.Module != "" {
		db = db.Where("module = ?", filter.Module)
	}
	if filter.OperType != "" {
		db = db.Where("oper_type = ?", filter.OperType)
	}
	if filter.Status != nil {
		db = db.Where("status = ?", *filter.Status)
	}
	if filter.StartAt != nil {
		db = db.Where("created_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		db = db.Where("created_at <= ?", *filter.EndAt)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]*model.OperationLogModel, 0, limit)
	err := db.Preload("Admin").Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func (r *operationLogRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.svc.DB.Gorm().WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&model.OperationLogModel{})
	return result.RowsAffected, result.Error
}
