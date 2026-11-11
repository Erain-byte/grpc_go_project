package service

import (
	"context"
	"time"

	"web3/internal/config"
	"web3/internal/tron"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const runtimeDependencyTimeout = 2 * time.Second

type runtimePinger interface {
	Ping(context.Context) error
}

type upstreamRuntimeSource interface {
	RuntimeStatuses() []tron.RuntimeStatus
}

// RuntimeStatusService 汇总依赖可用性和上游最近状态，不暴露连接信息或密钥。
type RuntimeStatusService struct {
	database runtimePinger
	cache    runtimePinger
	upstream upstreamRuntimeSource
	tron     config.TronApprovalConfig
	product  config.ProductConfig
	timeout  time.Duration
	now      func() time.Time
}

func NewRuntimeStatusService(
	database runtimePinger,
	cache runtimePinger,
	upstream upstreamRuntimeSource,
	tronConfig config.TronApprovalConfig,
	productConfig config.ProductConfig,
) *RuntimeStatusService {
	return &RuntimeStatusService{
		database: database,
		cache:    cache,
		upstream: upstream,
		tron:     tronConfig,
		product:  productConfig,
		timeout:  runtimeDependencyTimeout,
		now:      time.Now,
	}
}

func (s *RuntimeStatusService) GetWeb3RuntimeStatus(
	ctx context.Context,
	_ *web3v1.GetWeb3RuntimeStatusRequest,
) (*web3v1.GetWeb3RuntimeStatusResponse, error) {
	databaseAvailable, cacheAvailable := s.probeDependencies(ctx)
	response := &web3v1.GetWeb3RuntimeStatusResponse{
		TronApprovalEnabled: s.tron.Enabled,
		AirdropEnabled:      s.product.AirdropEnabled,
		DatabaseAvailable:   databaseAvailable,
		CacheAvailable:      cacheAvailable,
		CheckedAt:           timestamppb.New(s.now().UTC()),
	}

	for _, status := range s.upstream.RuntimeStatuses() {
		response.Upstreams = append(response.Upstreams, mapUpstreamRuntime(status, s.tron.Enabled))
	}
	dropsState := web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DISABLED
	if s.product.AirdropEnabled {
		// 当前尚未配置空投数据供应商；开启功能开关也不能伪装成可用。
		dropsState = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_NOT_CONFIGURED
	}
	response.Upstreams = append(response.Upstreams, &web3v1.UpstreamRuntimeStatus{
		Name:  "DROPS_API",
		State: dropsState,
	})
	return response, nil
}

func (s *RuntimeStatusService) probeDependencies(ctx context.Context) (bool, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	databaseResult := make(chan bool, 1)
	cacheResult := make(chan bool, 1)
	go func() { databaseResult <- s.database != nil && s.database.Ping(probeCtx) == nil }()
	go func() { cacheResult <- s.cache != nil && s.cache.Ping(probeCtx) == nil }()
	return <-databaseResult, <-cacheResult
}

func mapUpstreamRuntime(status tron.RuntimeStatus, enabled bool) *web3v1.UpstreamRuntimeStatus {
	result := &web3v1.UpstreamRuntimeStatus{
		Name:          status.Name,
		LastErrorCode: status.LastErrorCode,
	}
	if status.LastSuccessAt != nil {
		result.LastSuccessAt = timestamppb.New(status.LastSuccessAt.UTC())
	}
	if status.LastErrorAt != nil {
		result.LastErrorAt = timestamppb.New(status.LastErrorAt.UTC())
	}
	if !enabled {
		result.State = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DISABLED
		return result
	}

	switch {
	case status.LastSuccessAt == nil && status.LastErrorAt == nil:
		result.State = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DEGRADED
	case status.LastSuccessAt == nil:
		result.State = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_UNAVAILABLE
	case status.LastErrorAt == nil || !status.LastErrorAt.After(*status.LastSuccessAt):
		result.State = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_AVAILABLE
	default:
		result.State = web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DEGRADED
	}
	return result
}
