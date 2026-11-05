package service

import (
	"context"
	"math/big"
	"testing"
	"time"

	"web3/internal/auth"
	"web3/internal/blockchain"
	"web3/internal/config"
	"web3/internal/model"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type fakeBalanceCache struct{ values map[string]string }

func (c *fakeBalanceCache) Get(_ context.Context, key string) (string, error) {
	return c.values[key], nil
}
func (c *fakeBalanceCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	switch encoded := value.(type) {
	case []byte:
		c.values[key] = string(encoded)
	case string:
		c.values[key] = encoded
	}
	return nil
}

type fakeChainExecutor struct {
	client blockchain.Client
	config config.BlockchainChainConfig
}

func (e *fakeChainExecutor) Execute(ctx context.Context, _ uint64, operation func(context.Context, blockchain.Client) error) error {
	return operation(ctx, e.client)
}
func (e *fakeChainExecutor) Chain(chainID uint64) (config.BlockchainChainConfig, bool) {
	return e.config, chainID == e.config.ChainID
}

type fakeBalanceClient struct {
	balance *big.Int
	calls   int
}

func (c *fakeBalanceClient) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	c.calls++
	return new(big.Int).Set(c.balance), nil
}
func (*fakeBalanceClient) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, nil
}
func (*fakeBalanceClient) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	return nil, false, nil
}
func (*fakeBalanceClient) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return nil, nil
}
func (*fakeBalanceClient) BlockNumber(context.Context) (uint64, error) { return 0, nil }
func (*fakeBalanceClient) BlockByNumber(context.Context, *big.Int) (*types.Block, error) {
	return nil, nil
}
func (*fakeBalanceClient) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return nil, nil
}
func (*fakeBalanceClient) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return nil, nil
}
func (*fakeBalanceClient) Close() {}

func TestGetBalanceCachesNativeBalance(t *testing.T) {
	repository := &fakeWalletRepository{byAddress: &model.Wallet{
		ID:          "wallet-1",
		UserSubject: testSubject,
		ChainID:     1,
		Address:     testAddress,
		Status:      model.WalletStatusActive,
	}}
	rpcClient := &fakeBalanceClient{balance: big.NewInt(1_234_500_000_000_000_000)}
	chains := &fakeChainExecutor{
		client: rpcClient,
		config: config.BlockchainChainConfig{ChainID: 1, Enabled: true, NativeSymbol: "ETH", NativeDecimals: 18},
	}
	service, err := NewBalanceService(repository, chains, &fakeBalanceCache{}, config.BlockchainConfig{BalanceCacheTTL: "10s"})
	if err != nil {
		t.Fatalf("NewBalanceService() error = %v", err)
	}
	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	req := &web3v1.GetBalanceRequest{WalletId: "wallet-1"}

	first, err := service.GetBalance(ctx, req)
	if err != nil {
		t.Fatalf("first GetBalance() error = %v", err)
	}
	second, err := service.GetBalance(ctx, req)
	if err != nil {
		t.Fatalf("second GetBalance() error = %v", err)
	}
	if rpcClient.calls != 1 {
		t.Fatalf("BalanceAt() calls = %d, want 1", rpcClient.calls)
	}
	if first.GetRawBalance() != "1234500000000000000" || first.GetFormattedBalance() != "1.2345" {
		t.Fatalf("first balance = raw %q formatted %q", first.GetRawBalance(), first.GetFormattedBalance())
	}
	if second.GetRawBalance() != first.GetRawBalance() {
		t.Fatalf("cached balance = %q, want %q", second.GetRawBalance(), first.GetRawBalance())
	}
}

func TestFormatUnits(t *testing.T) {
	tests := []struct {
		value    string
		decimals uint32
		want     string
	}{
		{"0", 18, "0"},
		{"1", 18, "0.000000000000000001"},
		{"1234500", 6, "1.2345"},
		{"100", 0, "100"},
	}
	for _, test := range tests {
		value, _ := new(big.Int).SetString(test.value, 10)
		if got := formatUnits(value, test.decimals); got != test.want {
			t.Errorf("formatUnits(%s, %d) = %q, want %q", test.value, test.decimals, got, test.want)
		}
	}
}
