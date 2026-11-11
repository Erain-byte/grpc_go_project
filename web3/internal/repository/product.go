package repository

import (
	"context"

	"web3/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProductRepository 持久化匿名客户端事件和用户反馈。
type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) CreateClientEvent(ctx context.Context, event *model.ClientEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// CreateFeedback 创建反馈；同一安装实例重复提交相同 issue_id 时返回已存在的记录。
func (r *ProductRepository) CreateFeedback(ctx context.Context, feedback *model.FeedbackItem) (*model.FeedbackItem, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "installation_hash"}, {Name: "issue_id"}},
		DoNothing: true,
	}).Create(feedback)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		return feedback, nil
	}

	var existing model.FeedbackItem
	err := r.db.WithContext(ctx).
		Where("installation_hash = ? AND issue_id = ?", feedback.InstallationHash, feedback.IssueID).
		First(&existing).Error
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

// ListFeedback 按创建时间倒序返回反馈；status=0 表示不筛选状态。
func (r *ProductRepository) ListFeedback(
	ctx context.Context,
	status model.FeedbackStatus,
	page int,
	pageSize int,
) ([]model.FeedbackItem, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.FeedbackItem{})
	if status != 0 {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.FeedbackItem
	err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}
