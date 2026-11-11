package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"web3/internal/model"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	minimumFeedbackLength   = 10
	maximumFeedbackLength   = 2000
	defaultFeedbackPageSize = 20
	maximumFeedbackPageSize = 100
)

type feedbackRepository interface {
	CreateFeedback(context.Context, *model.FeedbackItem) (*model.FeedbackItem, error)
	ListFeedback(context.Context, model.FeedbackStatus, int, int) ([]model.FeedbackItem, int64, error)
}

// GetFeedbackList 返回后台可查看的反馈分页列表，不暴露安装标识摘要。
func (s *FeedbackService) GetFeedbackList(ctx context.Context, req *web3v1.GetFeedbackListRequest) (*web3v1.GetFeedbackListResponse, error) {
	if req == nil {
		return nil, apperror.InvalidArgument("request is nil")
	}
	if req.GetStatus() < web3v1.FeedbackStatus_FEEDBACK_STATUS_UNSPECIFIED ||
		req.GetStatus() > web3v1.FeedbackStatus_FEEDBACK_STATUS_CLOSED {
		return nil, apperror.InvalidArgument("unsupported feedback status")
	}
	page, pageSize := normalizeFeedbackPage(req.GetPage(), req.GetPageSize())
	items, total, err := s.repository.ListFeedback(ctx, model.FeedbackStatus(req.GetStatus()), page, pageSize)
	if err != nil {
		return nil, err
	}

	response := &web3v1.GetFeedbackListResponse{
		FeedbackItems: make([]*web3v1.FeedbackItem, 0, len(items)),
		Total:         total,
		Page:          int32(page),
		PageSize:      int32(pageSize),
	}
	for i := range items {
		response.FeedbackItems = append(response.FeedbackItems, feedbackToProto(&items[i]))
	}
	return response, nil
}

// FeedbackService 接收用户明确提交的问题描述，不自动附带钱包地址、查询结果或身份信息。
type FeedbackService struct {
	repository feedbackRepository
	limiter    clientEventLimiter
	now        func() time.Time
}

func NewFeedbackService(repository feedbackRepository, limiter clientEventLimiter) (*FeedbackService, error) {
	if repository == nil || limiter == nil {
		return nil, apperror.InvalidArgument("feedback service dependencies are incomplete")
	}
	return &FeedbackService{repository: repository, limiter: limiter, now: time.Now}, nil
}

func (s *FeedbackService) SubmitFeedback(ctx context.Context, req *web3v1.SubmitFeedbackRequest) (*web3v1.SubmitFeedbackResponse, error) {
	if err := validateFeedback(req); err != nil {
		return nil, err
	}

	installationHash := hashInstallationID(req.GetInstallationId())
	allowed, err := s.limiter.Allow(ctx, installationHash)
	if err != nil {
		return nil, apperror.Unavailable("feedback rate limiter is unavailable")
	}
	if !allowed {
		return nil, apperror.TooManyRequests("feedback rate limit exceeded")
	}

	now := s.now().UTC()
	feedback := &model.FeedbackItem{
		ID:               uuid.NewString(),
		InstallationHash: installationHash,
		IssueID:          strings.TrimSpace(req.GetIssueId()),
		AppVersion:       strings.TrimSpace(req.GetAppVersion()),
		Platform:         int16(req.GetPlatform()),
		Description:      strings.TrimSpace(req.GetDescription()),
		Status:           model.FeedbackStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	stored, err := s.repository.CreateFeedback(ctx, feedback)
	if err != nil {
		return nil, err
	}
	return &web3v1.SubmitFeedbackResponse{
		FeedbackId: stored.ID,
		CreatedAt:  timestamppb.New(stored.CreatedAt),
	}, nil
}

func validateFeedback(req *web3v1.SubmitFeedbackRequest) error {
	if req == nil {
		return apperror.InvalidArgument("request is nil")
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.GetInstallationId())); err != nil {
		return apperror.InvalidArgument("installation ID must be a UUID")
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.GetIssueId())); err != nil {
		return apperror.InvalidArgument("feedback issue ID must be a UUID")
	}
	if req.GetPlatform() < web3v1.ClientPlatform_CLIENT_PLATFORM_IOS ||
		req.GetPlatform() > web3v1.ClientPlatform_CLIENT_PLATFORM_WEB {
		return apperror.InvalidArgument("unsupported client platform")
	}
	if len(strings.TrimSpace(req.GetAppVersion())) > 32 {
		return apperror.InvalidArgument("app version is too long")
	}
	descriptionLength := utf8.RuneCountInString(strings.TrimSpace(req.GetDescription()))
	if descriptionLength < minimumFeedbackLength || descriptionLength > maximumFeedbackLength {
		return apperror.InvalidArgument("feedback description must contain 10 to 2000 characters")
	}
	return nil
}

func normalizeFeedbackPage(page, pageSize int32) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultFeedbackPageSize
	}
	if pageSize > maximumFeedbackPageSize {
		pageSize = maximumFeedbackPageSize
	}
	return int(page), int(pageSize)
}

func feedbackToProto(item *model.FeedbackItem) *web3v1.FeedbackItem {
	if item == nil {
		return nil
	}
	return &web3v1.FeedbackItem{
		FeedbackId:  item.ID,
		IssueId:     item.IssueID,
		AppVersion:  item.AppVersion,
		Platform:    web3v1.ClientPlatform(item.Platform),
		Description: item.Description,
		Status:      web3v1.FeedbackStatus(item.Status),
		CreatedAt:   timestamppb.New(item.CreatedAt),
		UpdatedAt:   timestamppb.New(item.UpdatedAt),
	}
}
