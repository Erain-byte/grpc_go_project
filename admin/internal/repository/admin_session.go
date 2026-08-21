package repository

import (
	"admin/internal/model"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SessionRepository interface {
	Create(ctx context.Context, session *model.AdminSessionModel) error
	FindBySessionID(ctx context.Context, sessionID string) (*model.AdminSessionModel, error)
	FindByRefreshTokenHash(ctx context.Context, tokenHash string) (*model.AdminSessionModel, error)
	Revoke(ctx context.Context, sessionID string, now time.Time) error
	RevokeByAdminAndSession(ctx context.Context, adminID uint, sessionID string, now time.Time) error
	RevokeAllByAdminID(ctx context.Context, adminID uint, now time.Time) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

type adminSessionRepository struct {
	svcCtx *svc.ServiceContext
}

var _ SessionRepository = (*adminSessionRepository)(nil)

func NewAdminSessionRepository(svcCtx *svc.ServiceContext) SessionRepository {
	return &adminSessionRepository{svcCtx: svcCtx}
}

func (r *adminSessionRepository) Revoke(
	ctx context.Context,
	sessionID string,
	now time.Time,
) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return apperorr.InvalidArgument("session ID is required")
	}

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

// RevokeByAdminAndSession revokes only the current administrator session.
func (r *adminSessionRepository) RevokeByAdminAndSession(
	ctx context.Context,
	adminID uint,
	sessionID string,
	now time.Time,
) error {
	if adminID == 0 {
		return apperorr.InvalidArgument("admin ID is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return apperorr.InvalidArgument("session ID is required")
	}

	result := r.svcCtx.DB.Gorm().WithContext(ctx).
		Model(&model.AdminSessionModel{}).
		Where(
			"admin_id = ? AND session_id = ? AND status = ? AND revoked_at IS NULL",
			adminID,
			sessionID,
			model.SessionStatusActive,
		).
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

func (r *adminSessionRepository) Create(ctx context.Context, session *model.AdminSessionModel) error {
	if session == nil {
		return apperorr.InvalidArgument("session is nil")
	}
	return r.svcCtx.DB.Gorm().WithContext(ctx).Create(session).Error
}

func (r *adminSessionRepository) FindBySessionID(ctx context.Context, sessionID string) (*model.AdminSessionModel, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, apperorr.InvalidArgument("session ID is required")
	}

	session := new(model.AdminSessionModel)
	now := time.Now()
	err := r.svcCtx.DB.Gorm().WithContext(ctx).
		Where(
			"session_id = ? AND status = ? AND revoked_at IS NULL AND expires_at > ?",
			sessionID,
			model.SessionStatusActive,
			now,
		).
		First(session).Error
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *adminSessionRepository) FindByRefreshTokenHash(ctx context.Context, tokenHash string) (*model.AdminSessionModel, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return nil, apperorr.InvalidArgument("refresh token hash is required")
	}

	session := new(model.AdminSessionModel)
	now := time.Now()
	err := r.svcCtx.DB.Gorm().WithContext(ctx).
		Where(
			"refresh_token_hash = ? AND status = ? AND revoked_at IS NULL AND expires_at > ?",
			tokenHash,
			model.SessionStatusActive,
			now,
		).
		First(session).Error
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *adminSessionRepository) RevokeAllByAdminID(ctx context.Context, adminID uint, now time.Time) error {
	if adminID == 0 {
		return apperorr.InvalidArgument("admin ID is required")
	}
	return r.svcCtx.DB.Gorm().WithContext(ctx).
		Model(&model.AdminSessionModel{}).
		Where("admin_id = ? AND status = ?", adminID, model.SessionStatusActive).
		Updates(map[string]any{
			"status":     model.SessionStatusRevoked,
			"revoked_at": now,
		}).Error
}

func (r *adminSessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result := r.svcCtx.DB.Gorm().WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&model.AdminSessionModel{})
	return result.RowsAffected, result.Error
}
