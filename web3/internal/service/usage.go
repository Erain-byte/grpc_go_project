package service

import (
	"context"
	"time"

	"web3/internal/config"
	"web3/internal/repository"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maximumUsageSummaryRange = 366 * 24 * time.Hour

type usageSummaryRepository interface {
	Summarize(context.Context, time.Time, time.Time) (*repository.UsageSummary, error)
}

// UsageSummaryService 提供匿名聚合指标，不暴露地址摘要和单次查询明细。
type UsageSummaryService struct {
	repository         usageSummaryRepository
	dailyQueryLimit    int64
	dailyAddressLimit  int64
	dailyUpstreamLimit int64
	now                func() time.Time
}

func NewUsageSummaryService(repository usageSummaryRepository, cfg config.TronApprovalConfig) (*UsageSummaryService, error) {
	if repository == nil || cfg.DailyQueryLimit <= 0 || cfg.DailyAddressLimit <= 0 || cfg.DailyUpstreamCallLimit <= 0 {
		return nil, apperror.InvalidArgument("usage summary dependencies or limits are invalid")
	}
	return &UsageSummaryService{
		repository:         repository,
		dailyQueryLimit:    cfg.DailyQueryLimit,
		dailyAddressLimit:  cfg.DailyAddressLimit,
		dailyUpstreamLimit: cfg.DailyUpstreamCallLimit,
		now:                time.Now,
	}, nil
}

func (s *UsageSummaryService) GetWeb3UsageSummary(ctx context.Context, req *web3v1.GetWeb3UsageSummaryRequest) (*web3v1.GetWeb3UsageSummaryResponse, error) {
	startedAt, endedAt, err := s.normalizeRange(req)
	if err != nil {
		return nil, err
	}
	summary, err := s.repository.Summarize(ctx, startedAt, endedAt)
	if err != nil {
		return nil, err
	}

	cacheHitRate := 0.0
	if summary.QueryCount > 0 {
		cacheHitRate = float64(summary.CacheHitCount) / float64(summary.QueryCount)
	}
	return &web3v1.GetWeb3UsageSummaryResponse{
		StartedAt:               timestamppb.New(startedAt),
		EndedAt:                 timestamppb.New(endedAt),
		QueryCount:              summary.QueryCount,
		DistinctAddressCount:    summary.DistinctAddressCount,
		UpstreamCallCount:       summary.UpstreamCallCount,
		SuccessCount:            summary.SuccessCount,
		PartialCount:            summary.PartialCount,
		FailedCount:             summary.FailedCount,
		CacheHitCount:           summary.CacheHitCount,
		CacheHitRate:            cacheHitRate,
		QueryBudgetExhausted:    summary.MaximumDailyQueryCount >= s.dailyQueryLimit,
		AddressBudgetExhausted:  summary.MaximumDailyAddressCount >= s.dailyAddressLimit,
		UpstreamBudgetExhausted: summary.MaximumDailyUpstreamCall >= s.dailyUpstreamLimit,
		GeneratedAt:             timestamppb.New(s.now().UTC()),
	}, nil
}

func (s *UsageSummaryService) normalizeRange(req *web3v1.GetWeb3UsageSummaryRequest) (time.Time, time.Time, error) {
	now := s.now().UTC()
	endedAt := now
	if req != nil && req.GetEndedAt() != nil {
		if !req.GetEndedAt().IsValid() {
			return time.Time{}, time.Time{}, apperror.InvalidArgument("usage summary end time is invalid")
		}
		endedAt = req.GetEndedAt().AsTime().UTC()
	}
	startedAt := utcDayStart(endedAt)
	if req != nil && req.GetStartedAt() != nil {
		if !req.GetStartedAt().IsValid() {
			return time.Time{}, time.Time{}, apperror.InvalidArgument("usage summary start time is invalid")
		}
		startedAt = req.GetStartedAt().AsTime().UTC()
	}
	if startedAt.After(endedAt) {
		return time.Time{}, time.Time{}, apperror.InvalidArgument("usage summary start time must not be after end time")
	}
	if endedAt.Sub(startedAt) > maximumUsageSummaryRange {
		return time.Time{}, time.Time{}, apperror.InvalidArgument("usage summary range cannot exceed 366 days")
	}
	return startedAt, endedAt, nil
}

func utcDayStart(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
