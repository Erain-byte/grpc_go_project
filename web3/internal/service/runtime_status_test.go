package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"web3/internal/config"
	"web3/internal/tron"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

type runtimePingerStub struct{ err error }

func (s runtimePingerStub) Ping(context.Context) error { return s.err }

type runtimeSourceStub struct{ statuses []tron.RuntimeStatus }

func (s runtimeSourceStub) RuntimeStatuses() []tron.RuntimeStatus { return s.statuses }

func TestRuntimeStatusServiceCombinesDependenciesAndUpstreams(t *testing.T) {
	earlier := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Minute)
	service := NewRuntimeStatusService(
		runtimePingerStub{},
		runtimePingerStub{err: errors.New("cache unavailable")},
		runtimeSourceStub{statuses: []tron.RuntimeStatus{
			{Name: tron.ProviderTronScan, LastSuccessAt: &later},
			{Name: tron.ProviderTronGrid, LastSuccessAt: &earlier, LastErrorAt: &later, LastErrorCode: "RATE_LIMITED"},
		}},
		config.TronApprovalConfig{Enabled: true},
		config.ProductConfig{AirdropEnabled: true},
	)
	checkedAt := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return checkedAt }

	response, err := service.GetWeb3RuntimeStatus(context.Background(), &web3v1.GetWeb3RuntimeStatusRequest{})
	if err != nil {
		t.Fatalf("GetWeb3RuntimeStatus() error = %v", err)
	}
	if !response.GetDatabaseAvailable() || response.GetCacheAvailable() {
		t.Fatalf("unexpected dependency state: database=%v cache=%v", response.GetDatabaseAvailable(), response.GetCacheAvailable())
	}
	if len(response.GetUpstreams()) != 3 {
		t.Fatalf("upstream count = %d, want 3", len(response.GetUpstreams()))
	}
	if response.Upstreams[0].GetState() != web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_AVAILABLE {
		t.Fatalf("TRONSCAN state = %s", response.Upstreams[0].GetState())
	}
	if response.Upstreams[1].GetState() != web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DEGRADED || response.Upstreams[1].GetLastErrorCode() != "RATE_LIMITED" {
		t.Fatalf("TRONGRID status = %+v", response.Upstreams[1])
	}
	if response.Upstreams[2].GetState() != web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_NOT_CONFIGURED {
		t.Fatalf("DROPS_API state = %s", response.Upstreams[2].GetState())
	}
	if !response.GetCheckedAt().AsTime().Equal(checkedAt) {
		t.Fatalf("checked_at = %s", response.GetCheckedAt().AsTime())
	}
}

func TestMapUpstreamRuntimeStates(t *testing.T) {
	success := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	beforeSuccess := success.Add(-time.Minute)
	afterSuccess := success.Add(time.Minute)
	tests := []struct {
		name    string
		enabled bool
		status  tron.RuntimeStatus
		want    web3v1.UpstreamRuntimeState
	}{
		{name: "disabled", status: tron.RuntimeStatus{Name: "upstream"}, want: web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DISABLED},
		{name: "not observed", enabled: true, status: tron.RuntimeStatus{Name: "upstream"}, want: web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DEGRADED},
		{name: "first request failed", enabled: true, status: tron.RuntimeStatus{Name: "upstream", LastErrorAt: &success}, want: web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_UNAVAILABLE},
		{name: "recovered", enabled: true, status: tron.RuntimeStatus{Name: "upstream", LastSuccessAt: &success, LastErrorAt: &beforeSuccess}, want: web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_AVAILABLE},
		{name: "failed after success", enabled: true, status: tron.RuntimeStatus{Name: "upstream", LastSuccessAt: &success, LastErrorAt: &afterSuccess}, want: web3v1.UpstreamRuntimeState_UPSTREAM_RUNTIME_STATE_DEGRADED},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mapUpstreamRuntime(test.status, test.enabled).GetState(); got != test.want {
				t.Fatalf("state = %s, want %s", got, test.want)
			}
		})
	}
}
