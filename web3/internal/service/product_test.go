package service

import (
	"context"
	"testing"

	"web3/internal/config"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

func TestProductServiceReturnsPublicWhitelist(t *testing.T) {
	service, err := NewProductService(config.ProductConfig{
		FavoriteLimit:          3,
		MinimumRefreshInterval: "1m",
		PrivacyPolicyVersion:   "2026-11-09",
		UpdatedAt:              "2026-11-09T00:00:00+08:00",
	}, config.TronApprovalConfig{
		Enabled:      true,
		USDTContract: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
	})
	if err != nil {
		t.Fatalf("NewProductService() error = %v", err)
	}

	response, err := service.GetPublicWeb3Config(context.Background(), &web3v1.GetPublicWeb3ConfigRequest{})
	if err != nil {
		t.Fatalf("GetPublicWeb3Config() error = %v", err)
	}
	if !response.GetTronApprovalEnabled() || response.GetFavoriteLimit() != 3 {
		t.Fatalf("unexpected public config: %+v", response)
	}
	if response.GetMinimumRefreshIntervalSeconds() != 60 {
		t.Fatalf("refresh interval = %d, want 60", response.GetMinimumRefreshIntervalSeconds())
	}
	if len(response.GetChains()) != 1 || response.GetChains()[0].GetTokenSymbol() != "USDT" {
		t.Fatalf("unexpected public chains: %+v", response.GetChains())
	}

	// 修改返回值不能污染服务内部缓存。
	response.FavoriteLimit = 99
	second, _ := service.GetPublicWeb3Config(context.Background(), &web3v1.GetPublicWeb3ConfigRequest{})
	if second.GetFavoriteLimit() != 3 {
		t.Fatalf("shared response was mutated: %d", second.GetFavoriteLimit())
	}
}

func TestProductServiceRejectsInvalidRefreshInterval(t *testing.T) {
	_, err := NewProductService(config.ProductConfig{
		FavoriteLimit:          3,
		MinimumRefreshInterval: "invalid",
		UpdatedAt:              "2026-11-09T00:00:00+08:00",
	}, config.TronApprovalConfig{})
	if err == nil {
		t.Fatal("NewProductService() error = nil, want validation error")
	}
}
