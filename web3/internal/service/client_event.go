package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"web3/internal/config"
	"web3/internal/model"
	redisstore "web3/internal/redis"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/google/uuid"
)

const clientEventRateScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`

type clientEventRepository interface {
	CreateClientEvent(context.Context, *model.ClientEvent) error
}

type clientEventLimiter interface {
	Allow(context.Context, string) (bool, error)
}

// RedisClientEventLimiter 对每个安装实例执行固定窗口限流。
// Gateway 仍负责 IP 维度的分布式限流，两者不能互相替代。

type RedisFixedWindowLimiter struct {
	redis  redisstore.Client
	prefix string
	limit  int64
	window time.Duration
}

func newRedisFixedWindowLimiter(redisClient redisstore.Client, prefix string, limit int64, windowValue string) (*RedisFixedWindowLimiter, error) {
	if redisClient == nil || strings.TrimSpace(prefix) == "" || limit <= 0 {
		return nil, apperror.InvalidArgument("fixed-window limiter configuration is invalid")
	}
	window, err := time.ParseDuration(windowValue)
	if err != nil || window <= 0 {
		return nil, apperror.InvalidArgument("fixed-window rate window is invalid")
	}
	return &RedisFixedWindowLimiter{redis: redisClient, prefix: prefix, limit: limit, window: window}, nil
}

func NewRedisClientEventLimiter(redisClient redisstore.Client, cfg config.ProductConfig) (*RedisFixedWindowLimiter, error) {
	return newRedisFixedWindowLimiter(redisClient, "product:event-rate:", cfg.ClientEventRateLimit, cfg.ClientEventRateWindow)
}

func NewRedisFeedbackLimiter(redisClient redisstore.Client, cfg config.ProductConfig) (*RedisFixedWindowLimiter, error) {
	return newRedisFixedWindowLimiter(redisClient, "product:feedback-rate:", cfg.FeedbackRateLimit, cfg.FeedbackRateWindow)
}

func (l *RedisFixedWindowLimiter) Allow(ctx context.Context, installationHash string) (bool, error) {
	result, err := l.redis.Eval(
		ctx,
		clientEventRateScript,
		[]string{l.prefix + installationHash},
		l.window.Milliseconds(),
	)
	if err != nil {
		return false, err
	}
	count, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected Redis event-rate result %T", result)
	}
	return count <= l.limit, nil
}

// ClientEventService 校验并保存不含钱包地址和资产信息的匿名产品事件。
type ClientEventService struct {
	repository clientEventRepository
	limiter    clientEventLimiter
	now        func() time.Time
}

func NewClientEventService(repository clientEventRepository, limiter clientEventLimiter) (*ClientEventService, error) {
	if repository == nil || limiter == nil {
		return nil, apperror.InvalidArgument("client-event service dependencies are incomplete")
	}
	return &ClientEventService{repository: repository, limiter: limiter, now: time.Now}, nil
}

func (s *ClientEventService) RecordClientEvent(ctx context.Context, req *web3v1.RecordClientEventRequest) (*web3v1.RecordClientEventResponse, error) {
	if err := validateClientEvent(req); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	occurredAt := req.GetOccurredAt().AsTime().UTC()
	if occurredAt.Before(now.Add(-30*24*time.Hour)) || occurredAt.After(now.Add(5*time.Minute)) {
		return nil, apperror.InvalidArgument("client event time is outside the accepted range")
	}

	installationHash := hashInstallationID(req.GetInstallationId())
	allowed, err := s.limiter.Allow(ctx, installationHash)
	if err != nil {
		return nil, apperror.Unavailable("client-event rate limiter is unavailable")
	}
	if !allowed {
		return nil, apperror.TooManyRequests("client-event rate limit exceeded")
	}

	event := &model.ClientEvent{
		ID:               uuid.NewString(),
		InstallationHash: installationHash,
		EventType:        model.ClientEventType(req.GetEventType()),
		AppVersion:       strings.TrimSpace(req.GetAppVersion()),
		Platform:         int16(req.GetPlatform()),
		Network:          strings.TrimSpace(req.GetNetwork()),
		ResultType:       strings.TrimSpace(req.GetResultType()),
		DurationBucket:   int16(req.GetDurationBucket()),
		OccurredAt:       occurredAt,
		CreatedAt:        now,
	}
	if err := s.repository.CreateClientEvent(ctx, event); err != nil {
		return nil, err
	}
	return &web3v1.RecordClientEventResponse{}, nil
}

func validateClientEvent(req *web3v1.RecordClientEventRequest) error {
	if req == nil {
		return apperror.InvalidArgument("request is nil")
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.GetInstallationId())); err != nil {
		return apperror.InvalidArgument("installation ID must be a UUID")
	}
	if req.GetEventType() < web3v1.ClientEventType_CLIENT_EVENT_TYPE_FIRST_OPEN ||
		req.GetEventType() > web3v1.ClientEventType_CLIENT_EVENT_TYPE_AIRDROP_REMINDER_CREATED {
		return apperror.InvalidArgument("unsupported client event type")
	}
	if req.GetPlatform() < web3v1.ClientPlatform_CLIENT_PLATFORM_IOS ||
		req.GetPlatform() > web3v1.ClientPlatform_CLIENT_PLATFORM_WEB {
		return apperror.InvalidArgument("unsupported client platform")
	}
	if req.GetOccurredAt() == nil || !req.GetOccurredAt().IsValid() {
		return apperror.InvalidArgument("client event time is invalid")
	}
	if len(strings.TrimSpace(req.GetAppVersion())) > 32 ||
		len(strings.TrimSpace(req.GetNetwork())) > 32 ||
		len(strings.TrimSpace(req.GetResultType())) > 32 {
		return apperror.InvalidArgument("client event field is too long")
	}
	if req.GetDurationBucket() < web3v1.DurationBucket_DURATION_BUCKET_UNSPECIFIED ||
		req.GetDurationBucket() > web3v1.DurationBucket_DURATION_BUCKET_OVER_THIRTY_SECONDS {
		return apperror.InvalidArgument("unsupported duration bucket")
	}
	return nil
}

func hashInstallationID(installationID string) string {
	digest := sha256.Sum256([]byte("web3-installation:" + strings.TrimSpace(installationID)))
	return hex.EncodeToString(digest[:])
}
