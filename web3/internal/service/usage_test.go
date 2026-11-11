package service

import (
	"context"
	"math"
	"testing"
	"time"

	"web3/internal/config"
	"web3/internal/repository"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type usageSummaryRepositoryStub struct {
	summary *repository.UsageSummary
	start   time.Time
	end     time.Time
}

func (s *usageSummaryRepositoryStub) Summarize(_ context.Context, start, end time.Time) (*repository.UsageSummary, error) {
	s.start, s.end = start, end
	return s.summary, nil
}

func TestUsageSummaryServiceAggregatesAndChecksDailyBudgets(t *testing.T) {
	repositoryStub := &usageSummaryRepositoryStub{summary: &repository.UsageSummary{
		QueryCount: 20, DistinctAddressCount: 8, UpstreamCallCount: 100,
		CacheHitCount: 5, SuccessCount: 15, PartialCount: 3, FailedCount: 2,
		MaximumDailyQueryCount: 10, MaximumDailyAddressCount: 4, MaximumDailyUpstreamCall: 70,
	}}
	service, err := NewUsageSummaryService(repositoryStub, config.TronApprovalConfig{
		DailyQueryLimit: 10, DailyAddressLimit: 5, DailyUpstreamCallLimit: 70,
	})
	if err != nil {
		t.Fatalf("NewUsageSummaryService() error = %v", err)
	}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	start := now.Add(-48 * time.Hour)

	response, err := service.GetWeb3UsageSummary(context.Background(), &web3v1.GetWeb3UsageSummaryRequest{
		StartedAt: timestamppb.New(start), EndedAt: timestamppb.New(now),
	})
	if err != nil {
		t.Fatalf("GetWeb3UsageSummary() error = %v", err)
	}
	if math.Abs(response.GetCacheHitRate()-0.25) > 0.000001 {
		t.Fatalf("cache hit rate = %f", response.GetCacheHitRate())
	}
	if !response.GetQueryBudgetExhausted() || response.GetAddressBudgetExhausted() || !response.GetUpstreamBudgetExhausted() {
		t.Fatalf("unexpected budget flags: %+v", response)
	}
	if !repositoryStub.start.Equal(start) || !repositoryStub.end.Equal(now) {
		t.Fatalf("repository range = %s .. %s", repositoryStub.start, repositoryStub.end)
	}
}

func TestUsageSummaryServiceDefaultsToCurrentUTCDay(t *testing.T) {
	repositoryStub := &usageSummaryRepositoryStub{summary: new(repository.UsageSummary)}
	service, _ := NewUsageSummaryService(repositoryStub, config.TronApprovalConfig{
		DailyQueryLimit: 10, DailyAddressLimit: 5, DailyUpstreamCallLimit: 70,
	})
	now := time.Date(2026, 11, 9, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	service.now = func() time.Time { return now }
	_, err := service.GetWeb3UsageSummary(context.Background(), &web3v1.GetWeb3UsageSummaryRequest{})
	if err != nil {
		t.Fatalf("GetWeb3UsageSummary() error = %v", err)
	}
	wantStart := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	if !repositoryStub.start.Equal(wantStart) {
		t.Fatalf("default start = %s, want %s", repositoryStub.start, wantStart)
	}
}
