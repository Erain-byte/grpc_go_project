package service

import (
	"context"
	"testing"
	"time"

	"web3/internal/model"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

type feedbackRepositoryStub struct {
	feedback   *model.FeedbackItem
	items      []model.FeedbackItem
	total      int64
	listPage   int
	listSize   int
	listStatus model.FeedbackStatus
}

func (s *feedbackRepositoryStub) ListFeedback(_ context.Context, status model.FeedbackStatus, page, pageSize int) ([]model.FeedbackItem, int64, error) {
	s.listStatus = status
	s.listPage = page
	s.listSize = pageSize
	return s.items, s.total, nil
}

func (s *feedbackRepositoryStub) CreateFeedback(_ context.Context, feedback *model.FeedbackItem) (*model.FeedbackItem, error) {
	s.feedback = feedback
	return feedback, nil
}

func TestFeedbackServiceStoresExplicitDescriptionWithoutIdentity(t *testing.T) {
	repository := new(feedbackRepositoryStub)
	limiter := &clientEventLimiterStub{allowed: true}
	service, err := NewFeedbackService(repository, limiter)
	if err != nil {
		t.Fatalf("NewFeedbackService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }

	response, err := service.SubmitFeedback(context.Background(), &web3v1.SubmitFeedbackRequest{
		InstallationId: "c85d5517-92b8-4cb7-93eb-266b1e738631",
		IssueId:        "5202293c-419d-466c-96d0-19c22e56ffb8",
		AppVersion:     "1.0.0",
		Platform:       web3v1.ClientPlatform_CLIENT_PLATFORM_WEB,
		Description:    "查询页面在网络恢复后没有自动刷新。",
	})
	if err != nil {
		t.Fatalf("SubmitFeedback() error = %v", err)
	}
	if response.GetFeedbackId() == "" || repository.feedback == nil {
		t.Fatal("feedback was not stored")
	}
	if len(repository.feedback.InstallationHash) != 64 {
		t.Fatalf("installation hash = %q", repository.feedback.InstallationHash)
	}
	if repository.feedback.Status != model.FeedbackStatusPending {
		t.Fatalf("feedback status = %d", repository.feedback.Status)
	}
}

func TestFeedbackServiceRejectsShortDescription(t *testing.T) {
	service, _ := NewFeedbackService(new(feedbackRepositoryStub), &clientEventLimiterStub{allowed: true})
	_, err := service.SubmitFeedback(context.Background(), &web3v1.SubmitFeedbackRequest{
		InstallationId: "c85d5517-92b8-4cb7-93eb-266b1e738631",
		IssueId:        "5202293c-419d-466c-96d0-19c22e56ffb8",
		Platform:       web3v1.ClientPlatform_CLIENT_PLATFORM_IOS,
		Description:    "太短",
	})
	if err == nil {
		t.Fatal("SubmitFeedback() error = nil, want validation error")
	}
}

func TestFeedbackServiceListsFeedbackWithoutInstallationHash(t *testing.T) {
	createdAt := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	repository := &feedbackRepositoryStub{
		total: 1,
		items: []model.FeedbackItem{{
			ID: "feedback-1", InstallationHash: "must-not-be-returned", IssueID: "issue-1",
			AppVersion: "1.0.0", Platform: int16(web3v1.ClientPlatform_CLIENT_PLATFORM_WEB),
			Description: "页面在网络恢复后没有自动刷新。", Status: model.FeedbackStatusPending,
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}},
	}
	service, _ := NewFeedbackService(repository, &clientEventLimiterStub{allowed: true})
	response, err := service.GetFeedbackList(context.Background(), &web3v1.GetFeedbackListRequest{
		Page: 0, PageSize: 500, Status: web3v1.FeedbackStatus_FEEDBACK_STATUS_PENDING,
	})
	if err != nil {
		t.Fatalf("GetFeedbackList() error = %v", err)
	}
	if repository.listPage != 1 || repository.listSize != maximumFeedbackPageSize {
		t.Fatalf("pagination = (%d, %d)", repository.listPage, repository.listSize)
	}
	if repository.listStatus != model.FeedbackStatusPending || response.GetTotal() != 1 {
		t.Fatalf("unexpected list response: %+v", response)
	}
	if got := response.GetFeedbackItems(); len(got) != 1 || got[0].GetFeedbackId() != "feedback-1" {
		t.Fatalf("feedback items = %+v", got)
	}
}
