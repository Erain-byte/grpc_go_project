package service

import (
	"context"
	"strings"
	"time"

	"web3/internal/config"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const tronUSDTDecimals = 6

// ProductService 负责把内部配置映射为可以公开返回的白名单结构。
type ProductService struct {
	response *web3v1.GetPublicWeb3ConfigResponse
}

func NewProductService(product config.ProductConfig, tron config.TronApprovalConfig) (*ProductService, error) {
	refreshInterval, err := time.ParseDuration(product.MinimumRefreshInterval)
	if err != nil || refreshInterval <= 0 {
		return nil, apperror.InvalidArgument("product minimum refresh interval is invalid")
	}
	updatedAt, err := time.Parse(time.RFC3339, product.UpdatedAt)
	if err != nil {
		return nil, apperror.InvalidArgument("product updated time is invalid")
	}

	response := &web3v1.GetPublicWeb3ConfigResponse{
		TronApprovalEnabled:           tron.Enabled,
		AirdropEnabled:                product.AirdropEnabled,
		FavoriteLimit:                 product.FavoriteLimit,
		MinimumRefreshIntervalSeconds: uint32(refreshInterval / time.Second),
		SupportEmail:                  strings.TrimSpace(product.SupportEmail),
		FeedbackUrl:                   strings.TrimSpace(product.FeedbackURL),
		TestflightUrl:                 strings.TrimSpace(product.TestFlightURL),
		PrivacyPolicyVersion:          strings.TrimSpace(product.PrivacyPolicyVersion),
		MaintenanceMessage:            strings.TrimSpace(product.MaintenanceMessage),
		UpdatedAt:                     timestamppb.New(updatedAt),
		Chains: []*web3v1.PublicChainConfig{{
			Network:       "tron-mainnet",
			DisplayName:   "TRON Mainnet",
			AddressFormat: "Base58Check",
			TokenSymbol:   "USDT",
			TokenContract: tron.USDTContract,
			TokenDecimals: tronUSDTDecimals,
			Enabled:       tron.Enabled,
		}},
	}
	return &ProductService{response: response}, nil
}

func (s *ProductService) GetPublicWeb3Config(context.Context, *web3v1.GetPublicWeb3ConfigRequest) (*web3v1.GetPublicWeb3ConfigResponse, error) {
	// 返回克隆，避免调用方修改进程内共享配置。
	return proto.Clone(s.response).(*web3v1.GetPublicWeb3ConfigResponse), nil
}
