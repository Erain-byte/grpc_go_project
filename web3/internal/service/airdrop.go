package service

import (
	"context"
	"strings"
	"time"

	"web3/internal/config"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const evmAirdropNetwork = "evm"

// AirdropService 提供 B 包空投查询的稳定接口边界。
// Drops 完整 API 尚未配置时只返回明确的失败状态，不生成或保存虚假项目数据。
type AirdropService struct {
	enabled bool
	now     func() time.Time
}

func NewAirdropService(cfg config.ProductConfig) *AirdropService {
	return &AirdropService{enabled: cfg.AirdropEnabled, now: time.Now}
}

func (s *AirdropService) CheckEvmAirdrops(
	_ context.Context,
	req *web3v1.CheckEvmAirdropsRequest,
) (*web3v1.CheckEvmAirdropsResponse, error) {
	network, address, err := validateAirdropRequest(req)
	if err != nil {
		return nil, err
	}
	response := &web3v1.CheckEvmAirdropsResponse{
		RequestId:     uuid.NewString(),
		Network:       network,
		Address:       address,
		Status:        web3v1.QueryStatus_QUERY_STATUS_FAILED,
		Complete:      false,
		IsCached:      false,
		QueriedAt:     timestamppb.New(s.now().UTC()),
		Projects:      []*web3v1.AirdropProject{},
		PartialErrors: []*web3v1.QueryPartialError{},
	}
	if !s.enabled {
		response.Reason = "空投查询功能尚未开放"
		response.Failure = &web3v1.QueryFailure{
			Code:      "FEATURE_DISABLED",
			Message:   "空投查询功能尚未开放",
			Retryable: false,
		}
		return response, nil
	}

	response.Reason = "空投数据供应商尚未配置"
	response.Failure = &web3v1.QueryFailure{
		Code:      "UPSTREAM_NOT_CONFIGURED",
		Message:   "空投数据供应商尚未配置",
		Retryable: false,
	}
	return response, nil
}

func validateAirdropRequest(req *web3v1.CheckEvmAirdropsRequest) (string, string, error) {
	if req == nil {
		return "", "", apperror.InvalidArgument("request is nil")
	}
	network := strings.ToLower(strings.TrimSpace(req.GetNetwork()))
	if network != evmAirdropNetwork {
		return "", "", apperror.InvalidArgument("unsupported airdrop network")
	}
	addressValue := strings.TrimSpace(req.GetAddress())
	if !common.IsHexAddress(addressValue) {
		return "", "", apperror.InvalidArgument("invalid EVM wallet address")
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.GetInstallationId())); err != nil {
		return "", "", apperror.InvalidArgument("installation ID must be a UUID")
	}
	if req.GetPlatform() < web3v1.ClientPlatform_CLIENT_PLATFORM_IOS ||
		req.GetPlatform() > web3v1.ClientPlatform_CLIENT_PLATFORM_WEB {
		return "", "", apperror.InvalidArgument("unsupported client platform")
	}
	if len(strings.TrimSpace(req.GetAppVersion())) > 32 {
		return "", "", apperror.InvalidArgument("app version is too long")
	}
	return network, common.HexToAddress(addressValue).Hex(), nil
}
