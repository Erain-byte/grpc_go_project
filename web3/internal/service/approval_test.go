package service

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"web3/internal/config"
	"web3/internal/tron"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"go.uber.org/zap"
)

const (
	testOwner    = "TWd4WrZ9wn84f5x1hZhL4DHvk738ns5jwb"
	testSpender1 = "TEpjT8xbAe3FPCPFziqFfEjLVXaw9NbGXj"
	testSpender2 = "TSUfx6xDEUGNzWQVMbqjf1zD5xwX4DQ9PL"
	testUSDT     = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
)

type fakeApprovalUpstream struct {
	mu        sync.Mutex
	listCalls int
}

func (f *fakeApprovalUpstream) Enabled() bool        { return true }
func (f *fakeApprovalUpstream) USDTContract() string { return testUSDT }
func (f *fakeApprovalUpstream) ListApprovals(context.Context, string, int, int) (*tron.ApprovalPage, error) {
	f.mu.Lock()
	f.listCalls++
	f.mu.Unlock()
	return &tron.ApprovalPage{
		Total: 2,
		Data: []tron.ApprovalRecord{
			{FromAddress: testOwner, ToAddress: testSpender1, ContractAddress: testUSDT, Unlimited: false},
			{FromAddress: testOwner, ToAddress: testSpender2, ContractAddress: testUSDT, Unlimited: true},
		},
		ContractMap:  map[string]bool{testSpender1: false, testSpender2: true},
		ContractInfo: map[string]tron.AddressInfo{testSpender2: {Tag1: "Risk Contract"}},
	}, nil
}
func (f *fakeApprovalUpstream) RiskySpenders(context.Context, string) (map[string]struct{}, error) {
	return map[string]struct{}{testSpender2: {}}, nil
}
func (f *fakeApprovalUpstream) Allowance(_ context.Context, _ string, spender string) (*big.Int, error) {
	if spender == testSpender2 {
		return new(big.Int).Set(maxUint256), nil
	}
	return big.NewInt(1234567), nil
}
func (f *fakeApprovalUpstream) USDTDecimals(context.Context, string) (uint32, error) {
	return 6, nil
}

type fakeApprovalCache struct {
	mu     sync.Mutex
	values map[string]string
}

type fakeApprovalUsage struct {
	mu            sync.Mutex
	queries       int
	upstreamCalls int
	results       int
}

func (f *fakeApprovalUsage) ReserveQuery(context.Context, time.Time, string, int64, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries++
	return nil
}

func (f *fakeApprovalUsage) ReserveUpstreamCall(context.Context, time.Time, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upstreamCalls++
	return nil
}

func (f *fakeApprovalUsage) RecordResult(context.Context, time.Time, string, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results++
	return nil
}

func testApprovalConfig() config.TronApprovalConfig {
	return config.TronApprovalConfig{
		Enabled: true, CacheTTL: "1m", PageSize: 50, MaxPages: 20, MaxSpenders: 100, VerifyWorkers: 2,
		UsageHMACKey: "0123456789abcdef0123456789abcdef", DailyQueryLimit: 100,
		DailyAddressLimit: 100, DailyUpstreamCallLimit: 1000,
	}
}

func (f *fakeApprovalCache) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values[key], nil
}
func (f *fakeApprovalCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch typed := value.(type) {
	case []byte:
		f.values[key] = string(typed)
	case string:
		f.values[key] = typed
	}
	return nil
}

func TestTronApprovalServiceChecksAndCaches(t *testing.T) {
	t.Parallel()
	upstream := new(fakeApprovalUpstream)
	cache := &fakeApprovalCache{values: make(map[string]string)}
	usage := new(fakeApprovalUsage)
	service, err := NewTronApprovalService(upstream, cache, usage, zap.NewNop(), testApprovalConfig())
	if err != nil {
		t.Fatalf("NewTronApprovalService() error = %v", err)
	}

	response, err := service.CheckTronUSDTApprovals(context.Background(), &web3v1.CheckTronUSDTApprovalsRequest{Address: testOwner})
	if err != nil {
		t.Fatalf("CheckTronUSDTApprovals() error = %v", err)
	}
	if response.Status != web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_SUCCESS {
		t.Fatalf("status = %v, want success", response.Status)
	}
	if len(response.Approvals) != 2 || response.Approvals[0].Spender != testSpender2 {
		t.Fatalf("risk-first approvals = %#v", response.Approvals)
	}
	if response.Approvals[0].UnlimitedStatus != web3v1.TronUnlimitedStatus_TRON_UNLIMITED_STATUS_UNLIMITED {
		t.Fatalf("unlimited status = %v", response.Approvals[0].UnlimitedStatus)
	}

	cached, err := service.CheckTronUSDTApprovals(context.Background(), &web3v1.CheckTronUSDTApprovalsRequest{Address: testOwner})
	if err != nil {
		t.Fatalf("cached CheckTronUSDTApprovals() error = %v", err)
	}
	if !cached.IsCached {
		t.Fatal("expected cached response")
	}
	upstream.mu.Lock()
	defer upstream.mu.Unlock()
	if upstream.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", upstream.listCalls)
	}
	usage.mu.Lock()
	defer usage.mu.Unlock()
	if usage.queries != 2 || usage.upstreamCalls != 5 || usage.results != 2 {
		t.Fatalf("usage counters = queries:%d upstream:%d results:%d, want 2/5/2", usage.queries, usage.upstreamCalls, usage.results)
	}
}

func TestTronApprovalServiceRejectsInvalidAddress(t *testing.T) {
	t.Parallel()
	service, err := NewTronApprovalService(
		new(fakeApprovalUpstream),
		&fakeApprovalCache{values: make(map[string]string)},
		new(fakeApprovalUsage),
		zap.NewNop(),
		testApprovalConfig(),
	)
	if err != nil {
		t.Fatalf("NewTronApprovalService() error = %v", err)
	}
	if _, err := service.CheckTronUSDTApprovals(context.Background(), &web3v1.CheckTronUSDTApprovalsRequest{Address: "0x1234"}); err == nil {
		t.Fatal("expected invalid address error")
	}
}
