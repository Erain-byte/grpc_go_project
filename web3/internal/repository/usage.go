package repository

import (
	"context"
	"errors"
	"time"

	"web3/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDailyQueryLimit    = errors.New("daily TRON query limit reached")
	ErrDailyAddressLimit  = errors.New("daily TRON distinct-address limit reached")
	ErrDailyUpstreamLimit = errors.New("daily TRON upstream-call limit reached")
)

type UsageRepository struct{ db *gorm.DB }

// UsageSummary 是后台统计使用的聚合结果，最大值用于判断范围内是否有某一天耗尽预算。
type UsageSummary struct {
	QueryCount               int64
	DistinctAddressCount     int64
	UpstreamCallCount        int64
	CacheHitCount            int64
	SuccessCount             int64
	PartialCount             int64
	FailedCount              int64
	MaximumDailyQueryCount   int64
	MaximumDailyAddressCount int64
	MaximumDailyUpstreamCall int64
}

func NewUsageRepository(db *gorm.DB) *UsageRepository { return &UsageRepository{db: db} }

// Summarize 按包含首尾日期的 UTC 日范围聚合用量。
func (r *UsageRepository) Summarize(ctx context.Context, startedDay, endedDay time.Time) (*UsageSummary, error) {
	var summary UsageSummary
	err := r.db.WithContext(ctx).
		Model(&model.TronUsageDaily{}).
		Select(`
			COALESCE(SUM(query_count), 0) AS query_count,
			COALESCE(SUM(distinct_address_count), 0) AS distinct_address_count,
			COALESCE(SUM(upstream_call_count), 0) AS upstream_call_count,
			COALESCE(SUM(cache_hit_count), 0) AS cache_hit_count,
			COALESCE(SUM(success_count), 0) AS success_count,
			COALESCE(SUM(partial_count), 0) AS partial_count,
			COALESCE(SUM(failed_count), 0) AS failed_count,
			COALESCE(MAX(query_count), 0) AS maximum_daily_query_count,
			COALESCE(MAX(distinct_address_count), 0) AS maximum_daily_address_count,
			COALESCE(MAX(upstream_call_count), 0) AS maximum_daily_upstream_call
		`).
		Where("usage_date >= ? AND usage_date <= ?", utcDay(startedDay), utcDay(endedDay)).
		Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// ReserveQuery atomically increments the daily request count and, when this
// HMAC has not appeared today, the distinct-address count.
func (r *UsageRepository) ReserveQuery(ctx context.Context, day time.Time, addressHMAC string, queryLimit, addressLimit int64) error {
	day = utcDay(day)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TronUsageDaily{
			UsageDate: day, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}

		var daily model.TronUsageDaily
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("usage_date = ?", day).First(&daily).Error; err != nil {
			return err
		}
		if daily.QueryCount >= queryLimit {
			return ErrDailyQueryLimit
		}

		var address model.TronUsageAddressDaily
		findErr := tx.Where("usage_date = ? AND address_hmac = ?", day, addressHMAC).First(&address).Error
		isNewAddress := errors.Is(findErr, gorm.ErrRecordNotFound)
		if findErr != nil && !isNewAddress {
			return findErr
		}
		if isNewAddress && daily.DistinctAddressCount >= addressLimit {
			return ErrDailyAddressLimit
		}

		updates := map[string]any{
			"query_count": gorm.Expr("query_count + 1"),
			"updated_at":  now,
		}
		if isNewAddress {
			updates["distinct_address_count"] = gorm.Expr("distinct_address_count + 1")
			if err := tx.Create(&model.TronUsageAddressDaily{
				UsageDate: day, AddressHMAC: addressHMAC, CreatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.TronUsageDaily{}).Where("usage_date = ?", day).Updates(updates).Error
	})
}

// ReserveUpstreamCall performs a conditional atomic increment before each
// paid/provider request, so concurrent service instances cannot exceed the limit.
func (r *UsageRepository) ReserveUpstreamCall(ctx context.Context, day time.Time, limit int64) error {
	result := r.db.WithContext(ctx).Model(&model.TronUsageDaily{}).
		Where("usage_date = ? AND upstream_call_count < ?", utcDay(day), limit).
		Updates(map[string]any{
			"upstream_call_count": gorm.Expr("upstream_call_count + 1"),
			"updated_at":          time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDailyUpstreamLimit
	}
	return nil
}

func (r *UsageRepository) RecordResult(ctx context.Context, day time.Time, status string, cached bool) error {
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if cached {
		updates["cache_hit_count"] = gorm.Expr("cache_hit_count + 1")
	}
	switch status {
	case "success":
		updates["success_count"] = gorm.Expr("success_count + 1")
	case "partial":
		updates["partial_count"] = gorm.Expr("partial_count + 1")
	case "failed":
		updates["failed_count"] = gorm.Expr("failed_count + 1")
	}
	return r.db.WithContext(ctx).Model(&model.TronUsageDaily{}).
		Where("usage_date = ?", utcDay(day)).Updates(updates).Error
}

func utcDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
