package repository

import (
	"context"
	"time"

	"web3/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ScannerRepository 持久化扫描断点和事件处理状态。
type ScannerRepository struct{ db *gorm.DB }

// NewScannerRepository 创建扫描器仓库。
func NewScannerRepository(db *gorm.DB) *ScannerRepository { return &ScannerRepository{db: db} }

// GetCursor 获取一条链最近完成处理的区块断点。
func (r *ScannerRepository) GetCursor(ctx context.Context, chainID uint64) (*model.ChainScanCursor, error) {
	var cursor model.ChainScanCursor
	err := r.db.WithContext(ctx).Where("chain_id = ?", chainID).First(&cursor).Error
	return &cursor, err
}

// SaveCursor 以 chain_id 为唯一键更新断点。
func (r *ScannerRepository) SaveCursor(ctx context.Context, cursor *model.ChainScanCursor) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"block_number", "block_hash", "last_scanned_at", "updated_at"}),
	}).Create(cursor).Error
}

// CreateEventIfAbsent 依靠链事件唯一索引实现数据库级幂等。
func (r *ScannerRepository) CreateEventIfAbsent(ctx context.Context, event *model.ChainEventReceipt) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event)
	return result.RowsAffected == 1, result.Error
}

// MarkEventProcessed 标记事件已成功写入交易快照。
func (r *ScannerRepository) MarkEventProcessed(ctx context.Context, eventID string, processedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.ChainEventReceipt{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"status":       model.ChainEventStatusProcessed,
			"processed_at": processedAt,
			"last_error":   "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkEventFailed 记录失败原因并递增重试次数，供后续扫描再次处理。
func (r *ScannerRepository) MarkEventFailed(ctx context.Context, eventID, failure string) error {
	result := r.db.WithContext(ctx).Model(&model.ChainEventReceipt{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"status":      model.ChainEventStatusFailed,
			"last_error":  failure,
			"retry_count": gorm.Expr("retry_count + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
