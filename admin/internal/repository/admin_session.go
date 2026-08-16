package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type AdminSessionRepository struct {
	svcCtx *svc.ServiceContext
}

func NewAdminSessionRepository(svcCtx *svc.ServiceContext) *AdminSessionRepository {
	return &AdminSessionRepository{svcCtx: svcCtx}
}

func (r *AdminSessionRepository) Revoke(
	ctx context.Context,
	sessionID string,
	now time.Time,
) error {
	result := r.svcCtx.DB.Gorm().WithContext(ctx).
		Model(&model.AdminSessionModel{}).
		Where("session_id = ? AND status = ?", sessionID, model.SessionStatusActive).
		Updates(map[string]any{
			"status":     model.SessionStatusRevoked,
			"revoked_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *AdminSessionRepository) Create(ctx context.Context, session *model.AdminSessionModel) error {

	if session == nil {
		return errors.New("session is nil")
	}
	return r.svcCtx.DB.Gorm().WithContext(ctx).Create(session).Error
}
