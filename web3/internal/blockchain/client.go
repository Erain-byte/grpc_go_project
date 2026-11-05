// Package blockchain 管理 Web3 服务访问 EVM JSON-RPC 节点的客户端。
package blockchain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"web3/internal/config"
	"web3/pkg/apperror"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client 是余额和交易业务实际需要的 EVM 节点能力。
// 使用接口可以在业务测试中替换为本地假客户端，不需要连接真实区块链节点。
// Client 是本项目使用的最小 EVM JSON-RPC 能力集合。
// 以接口表达可让业务层在单元测试中使用假节点，不必连接真实链。
type Client interface {
	BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	BlockNumber(context.Context) (uint64, error)
	BlockByNumber(context.Context, *big.Int) (*types.Block, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
	Close()
}

type dialFunc func(context.Context, string) (Client, error)

// chainPool 保存同一链的多个 RPC 节点，按轮询顺序选择。
type chainPool struct {
	config  config.BlockchainChainConfig
	clients []Client
	next    atomic.Uint64
}

// ClientManager 管理所有启用链的 RPC 节点池、单次请求超时和只读调用重试。
type ClientManager struct {
	chains         map[uint64]*chainPool
	requestTimeout time.Duration
	retryCount     int
	closeOnce      sync.Once
}

// NewClientManager 根据配置连接每条启用链的全部 RPC URL。
// 任一启用链没有可用节点时启动失败，避免服务启动后才暴露半可用状态。
func NewClientManager(cfg config.BlockchainConfig) (*ClientManager, error) {
	return newClientManager(cfg, func(ctx context.Context, rawURL string) (Client, error) {
		return ethclient.DialContext(ctx, rawURL)
	})
}

func newClientManager(cfg config.BlockchainConfig, dial dialFunc) (*ClientManager, error) {
	if dial == nil {
		return nil, apperror.InvalidArgument("blockchain RPC dialer is nil")
	}
	timeout, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil || timeout <= 0 {
		return nil, apperror.InvalidArgument("blockchain request timeout is invalid")
	}
	manager := &ClientManager{
		chains:         make(map[uint64]*chainPool),
		requestTimeout: timeout,
		retryCount:     cfg.RetryCount,
	}
	for _, chainCfg := range cfg.Chains {
		if !chainCfg.Enabled {
			continue
		}
		pool := &chainPool{config: chainCfg}
		var dialErrors []error
		for _, rawURL := range chainCfg.RPCURLs {
			rawURL = strings.TrimSpace(rawURL)
			if rawURL == "" {
				continue
			}
			dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
			client, dialErr := dial(dialCtx, rawURL)
			cancel()
			if dialErr != nil {
				dialErrors = append(dialErrors, fmt.Errorf("dial %q: %w", rawURL, dialErr))
				continue
			}
			pool.clients = append(pool.clients, client)
		}
		if len(pool.clients) == 0 {
			manager.Close()
			cause := errors.Join(dialErrors...)
			if cause == nil {
				cause = errors.New("no RPC URL is configured")
			}
			return nil, apperror.Wrap(
				cause,
				apperror.CodeUnavailable,
				fmt.Sprintf("no RPC endpoint is available for chain %d", chainCfg.ChainID),
				http.StatusServiceUnavailable,
			)
		}
		manager.chains[chainCfg.ChainID] = pool
	}
	return manager, nil
}

// Execute 按轮询顺序选择节点；请求失败时切换到下一个 RPC 地址重试。
// operation 必须是一次只读调用，不应用于可能重复提交交易的操作。
func (m *ClientManager) Execute(ctx context.Context, chainID uint64, operation func(context.Context, Client) error) error {
	if m == nil || operation == nil {
		return apperror.InvalidArgument("blockchain client manager or operation is nil")
	}
	pool, ok := m.chains[chainID]
	if !ok || len(pool.clients) == 0 {
		return apperror.InvalidArgument("unsupported or disabled blockchain")
	}
	attempts := m.retryCount + 1
	if attempts < 1 {
		attempts = 1
	}
	start := int(pool.next.Add(1)-1) % len(pool.clients)
	var operationErrors []error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		client := pool.clients[(start+attempt)%len(pool.clients)]
		requestCtx, cancel := context.WithTimeout(ctx, m.requestTimeout)
		err := operation(requestCtx, client)
		cancel()
		if err == nil {
			return nil
		}
		operationErrors = append(operationErrors, err)
	}
	return apperror.Wrap(
		errors.Join(operationErrors...),
		apperror.CodeUnavailable,
		fmt.Sprintf("all RPC attempts failed for chain %d", chainID),
		http.StatusServiceUnavailable,
	)
}

// Chain 返回已启用链的静态配置。
func (m *ClientManager) Chain(chainID uint64) (config.BlockchainChainConfig, bool) {
	if m == nil {
		return config.BlockchainChainConfig{}, false
	}
	pool, ok := m.chains[chainID]
	if !ok {
		return config.BlockchainChainConfig{}, false
	}
	return pool.config, true
}

// Close 关闭所有底层 RPC 客户端；可重复调用。
func (m *ClientManager) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		for _, pool := range m.chains {
			for _, client := range pool.clients {
				client.Close()
			}
		}
	})
}
