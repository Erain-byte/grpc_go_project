package service

import (
	"context"
	"testing"
	"time"

	"web3/internal/config"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/google/uuid"
)

const testEVMAddress = "0x52908400098527886E0F7030069857D2E4169EE7"

func validAirdropRequest() *web3v1.CheckEvmAirdropsRequest {
	return &web3v1.CheckEvmAirdropsRequest{
		Network:        "evm",
		Address:        testEVMAddress,
		InstallationId: uuid.NewString(),
		AppVersion:     "1.0.0",
		Platform:       web3v1.ClientPlatform_CLIENT_PLATFORM_WEB,
	}
}

func TestAirdropServiceReturnsFeatureDisabled(t *testing.T) {
	service := NewAirdropService(config.ProductConfig{AirdropEnabled: false})
	queriedAt := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return queriedAt }

	response, err := service.CheckEvmAirdrops(context.Background(), validAirdropRequest())
	if err != nil {
		t.Fatalf("CheckEvmAirdrops() error = %v", err)
	}
	if response.GetStatus() != web3v1.QueryStatus_QUERY_STATUS_FAILED || response.GetComplete() {
		t.Fatalf("unexpected response status: %+v", response)
	}
	if response.GetFailure().GetCode() != "FEATURE_DISABLED" || response.GetFailure().GetRetryable() {
		t.Fatalf("unexpected failure = %+v", response.GetFailure())
	}
	if response.GetAddress() != testEVMAddress || !response.GetQueriedAt().AsTime().Equal(queriedAt) {
		t.Fatalf("unexpected normalized response: %+v", response)
	}
}

func TestAirdropServiceReportsMissingProvider(t *testing.T) {
	service := NewAirdropService(config.ProductConfig{AirdropEnabled: true})
	response, err := service.CheckEvmAirdrops(context.Background(), validAirdropRequest())
	if err != nil {
		t.Fatalf("CheckEvmAirdrops() error = %v", err)
	}
	if response.GetFailure().GetCode() != "UPSTREAM_NOT_CONFIGURED" {
		t.Fatalf("failure = %+v", response.GetFailure())
	}
	if len(response.GetProjects()) != 0 || response.GetIsCached() {
		t.Fatalf("missing provider must not produce projects or cached result: %+v", response)
	}
}

func TestAirdropServiceRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*web3v1.CheckEvmAirdropsRequest)
	}{
		{name: "unsupported network", mutate: func(req *web3v1.CheckEvmAirdropsRequest) { req.Network = "tron-mainnet" }},
		{name: "invalid address", mutate: func(req *web3v1.CheckEvmAirdropsRequest) { req.Address = "TInvalidTronAddress" }},
		{name: "invalid installation", mutate: func(req *web3v1.CheckEvmAirdropsRequest) { req.InstallationId = "not-a-uuid" }},
		{name: "invalid platform", mutate: func(req *web3v1.CheckEvmAirdropsRequest) {
			req.Platform = web3v1.ClientPlatform_CLIENT_PLATFORM_UNSPECIFIED
		}},
	}
	service := NewAirdropService(config.ProductConfig{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validAirdropRequest()
			test.mutate(req)
			if _, err := service.CheckEvmAirdrops(context.Background(), req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
