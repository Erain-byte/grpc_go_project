package blockchain

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"web3/internal/config"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type fakeClient struct {
	name   string
	closed bool
}

func (*fakeClient) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}
func (*fakeClient) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, nil
}
func (*fakeClient) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	return nil, false, nil
}
func (*fakeClient) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return nil, nil
}
func (*fakeClient) BlockNumber(context.Context) (uint64, error) { return 0, nil }
func (*fakeClient) BlockByNumber(context.Context, *big.Int) (*types.Block, error) {
	return nil, nil
}
func (*fakeClient) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return nil, nil
}
func (*fakeClient) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return nil, nil
}
func (c *fakeClient) Close() { c.closed = true }

func TestClientManagerFailsOverToNextEndpoint(t *testing.T) {
	clients := map[string]*fakeClient{
		"rpc-a": {name: "rpc-a"},
		"rpc-b": {name: "rpc-b"},
	}
	manager, err := newClientManager(testConfig(1, "rpc-a", "rpc-b"), func(_ context.Context, rawURL string) (Client, error) {
		return clients[rawURL], nil
	})
	if err != nil {
		t.Fatalf("newClientManager() error = %v", err)
	}
	defer manager.Close()

	var called []string
	err = manager.Execute(context.Background(), 1, func(_ context.Context, client Client) error {
		name := client.(*fakeClient).name
		called = append(called, name)
		if len(called) == 1 {
			return errors.New("temporary RPC failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(called) != 2 || called[0] == called[1] {
		t.Fatalf("Execute() endpoints = %v, want two different endpoints", called)
	}
}

func TestClientManagerRejectsChainWithoutAvailableEndpoint(t *testing.T) {
	_, err := newClientManager(testConfig(1, "rpc-a"), func(context.Context, string) (Client, error) {
		return nil, errors.New("dial failed")
	})
	if err == nil {
		t.Fatal("newClientManager() error = nil")
	}
}

func TestClientManagerCloseClosesEveryEndpointOnce(t *testing.T) {
	first := &fakeClient{name: "rpc-a"}
	second := &fakeClient{name: "rpc-b"}
	clients := map[string]*fakeClient{"rpc-a": first, "rpc-b": second}
	manager, err := newClientManager(testConfig(1, "rpc-a", "rpc-b"), func(_ context.Context, rawURL string) (Client, error) {
		return clients[rawURL], nil
	})
	if err != nil {
		t.Fatalf("newClientManager() error = %v", err)
	}
	manager.Close()
	manager.Close()
	if !first.closed || !second.closed {
		t.Fatalf("Close() states = %v, %v", first.closed, second.closed)
	}
}

func testConfig(chainID uint64, urls ...string) config.BlockchainConfig {
	return config.BlockchainConfig{
		RequestTimeout: "1s",
		RetryCount:     1,
		Chains: []config.BlockchainChainConfig{{
			ChainID: chainID,
			Enabled: true,
			RPCURLs: urls,
		}},
	}
}
