package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"web3/internal/blockchain"
	"web3/internal/config"
	"web3/internal/model"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const erc20ABIJSON = `[
  {"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"stateMutability":"view","type":"function"}
]`

// chainExecutor 隔离区块链节点调用，便于余额服务在测试时替换为假节点。
type chainExecutor interface {
	Execute(context.Context, uint64, func(context.Context, blockchain.Client) error) error
	Chain(uint64) (config.BlockchainChainConfig, bool)
}

// balanceCache 是余额短期缓存；缓存失效只会触发重新查询链上节点，不作为账本数据源。
type balanceCache interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, any, time.Duration) error
}

// BalanceService 校验钱包归属后，查询原生币或 ERC-20 余额，并做短期缓存。
// 金额始终使用整数最小单位保存，FormattedBalance 仅用于展示。
type BalanceService struct {
	repository walletRepository
	chains     chainExecutor
	cache      balanceCache
	cacheTTL   time.Duration
	erc20ABI   abi.ABI
	now        func() time.Time
}

type cachedBalance struct {
	ChainID          uint64    `json:"chain_id"`
	Address          string    `json:"address"`
	TokenContract    string    `json:"token_contract"`
	Symbol           string    `json:"symbol"`
	Decimals         uint32    `json:"decimals"`
	RawBalance       string    `json:"raw_balance"`
	FormattedBalance string    `json:"formatted_balance"`
	ObservedAt       time.Time `json:"observed_at"`
}

// NewBalanceService 创建余额查询服务并解析 ERC-20 只读方法 ABI。
func NewBalanceService(repository walletRepository, chains chainExecutor, cache balanceCache, cfg config.BlockchainConfig) (*BalanceService, error) {
	if repository == nil || chains == nil || cache == nil {
		return nil, apperror.InvalidArgument("balance service dependencies are incomplete")
	}
	cacheTTL, err := time.ParseDuration(cfg.BalanceCacheTTL)
	if err != nil || cacheTTL <= 0 {
		return nil, apperror.InvalidArgument("balance cache TTL is invalid")
	}
	contractABI, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "parse ERC20 ABI", http.StatusInternalServerError)
	}
	return &BalanceService{
		repository: repository,
		chains:     chains,
		cache:      cache,
		cacheTTL:   cacheTTL,
		erc20ABI:   contractABI,
		now:        time.Now,
	}, nil
}

// GetBalance 只允许查询当前身份已绑定且处于启用状态的钱包。
func (s *BalanceService) GetBalance(ctx context.Context, req *web3v1.GetBalanceRequest) (*web3v1.GetBalanceResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetWalletId()) == "" {
		return nil, apperror.InvalidArgument("wallet ID is empty")
	}
	boundWallet, err := s.repository.FindByID(ctx, identity.Subject, req.GetWalletId())
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	if boundWallet.Status != model.WalletStatusActive {
		return nil, apperror.NotFound("active wallet was not found")
	}
	chainCfg, ok := s.chains.Chain(boundWallet.ChainID)
	if !ok {
		return nil, apperror.InvalidArgument("wallet blockchain is disabled")
	}

	tokenContract := strings.TrimSpace(req.GetTokenContract())
	if tokenContract != "" {
		if !common.IsHexAddress(tokenContract) {
			return nil, apperror.InvalidArgument("invalid ERC20 token contract address")
		}
		tokenContract = common.HexToAddress(tokenContract).Hex()
	}
	cacheKey := balanceCacheKey(boundWallet.ChainID, boundWallet.Address, tokenContract)
	if response := s.cached(ctx, cacheKey); response != nil {
		return response, nil
	}

	var rawBalance *big.Int
	var symbol string
	var decimals uint32
	if tokenContract == "" {
		rawBalance, err = s.nativeBalance(ctx, boundWallet.ChainID, boundWallet.Address)
		symbol = chainCfg.NativeSymbol
		decimals = uint32(chainCfg.NativeDecimals)
	} else {
		rawBalance, decimals, symbol, err = s.erc20Balance(ctx, boundWallet.ChainID, boundWallet.Address, tokenContract)
	}
	if err != nil {
		return nil, err
	}
	observedAt := s.now().UTC()
	response := &web3v1.GetBalanceResponse{
		ChainId:          boundWallet.ChainID,
		Address:          boundWallet.Address,
		TokenContract:    tokenContract,
		Symbol:           symbol,
		Decimals:         decimals,
		RawBalance:       rawBalance.String(),
		FormattedBalance: formatUnits(rawBalance, decimals),
		ObservedAt:       timestamppb.New(observedAt),
	}
	s.storeCache(ctx, cacheKey, response, observedAt)
	return response, nil
}

// nativeBalance 查询链原生币余额，例如 ETH、MATIC。
func (s *BalanceService) nativeBalance(ctx context.Context, chainID uint64, address string) (*big.Int, error) {
	var balance *big.Int
	err := s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		balance, callErr = client.BalanceAt(requestCtx, common.HexToAddress(address), nil)
		return callErr
	})
	return balance, err
}

// erc20Balance 通过 balanceOf、decimals、symbol 三个只读合约调用获取代币余额与展示信息。
func (s *BalanceService) erc20Balance(ctx context.Context, chainID uint64, walletAddress, tokenContract string) (*big.Int, uint32, string, error) {
	contract := common.HexToAddress(tokenContract)
	balanceCall, err := s.erc20ABI.Pack("balanceOf", common.HexToAddress(walletAddress))
	if err != nil {
		return nil, 0, "", err
	}
	decimalsCall, err := s.erc20ABI.Pack("decimals")
	if err != nil {
		return nil, 0, "", err
	}
	symbolCall, err := s.erc20ABI.Pack("symbol")
	if err != nil {
		return nil, 0, "", err
	}

	var balance *big.Int
	var decimals uint32
	var symbol string
	err = s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
		rawBalance, err := client.CallContract(requestCtx, ethereum.CallMsg{To: &contract, Data: balanceCall}, nil)
		if err != nil {
			return err
		}
		values, err := s.erc20ABI.Unpack("balanceOf", rawBalance)
		if err != nil {
			return fmt.Errorf("decode ERC20 balanceOf result: %w", err)
		}
		if len(values) != 1 {
			return errorsNewUnexpected("balanceOf")
		}
		var ok bool
		balance, ok = values[0].(*big.Int)
		if !ok {
			return errorsNewUnexpected("balanceOf")
		}

		rawDecimals, err := client.CallContract(requestCtx, ethereum.CallMsg{To: &contract, Data: decimalsCall}, nil)
		if err != nil {
			return err
		}
		values, err = s.erc20ABI.Unpack("decimals", rawDecimals)
		if err != nil {
			return fmt.Errorf("decode ERC20 decimals result: %w", err)
		}
		if len(values) != 1 {
			return errorsNewUnexpected("decimals")
		}
		decimalValue, ok := values[0].(uint8)
		if !ok {
			return errorsNewUnexpected("decimals")
		}
		decimals = uint32(decimalValue)

		rawSymbol, err := client.CallContract(requestCtx, ethereum.CallMsg{To: &contract, Data: symbolCall}, nil)
		if err != nil {
			return err
		}
		values, err = s.erc20ABI.Unpack("symbol", rawSymbol)
		if err != nil {
			return fmt.Errorf("decode ERC20 symbol result: %w", err)
		}
		if len(values) != 1 {
			return errorsNewUnexpected("symbol")
		}
		symbol, ok = values[0].(string)
		if !ok {
			return errorsNewUnexpected("symbol")
		}
		return nil
	})
	return balance, decimals, symbol, err
}

func errorsNewUnexpected(method string) error {
	return fmt.Errorf("ERC20 %s returned an unexpected value", method)
}

// balanceCacheKey 使用规范化的小写地址，避免同一 EVM 地址因大小写不同产生多个缓存项。
func balanceCacheKey(chainID uint64, address, tokenContract string) string {
	if tokenContract == "" {
		tokenContract = "native"
	}
	return fmt.Sprintf("balance:%d:%s:%s", chainID, strings.ToLower(address), strings.ToLower(tokenContract))
}

func (s *BalanceService) cached(ctx context.Context, key string) *web3v1.GetBalanceResponse {
	payload, err := s.cache.Get(ctx, key)
	if err != nil || payload == "" {
		return nil
	}
	var value cachedBalance
	if json.Unmarshal([]byte(payload), &value) != nil {
		return nil
	}
	return &web3v1.GetBalanceResponse{
		ChainId: value.ChainID, Address: value.Address, TokenContract: value.TokenContract,
		Symbol: value.Symbol, Decimals: value.Decimals, RawBalance: value.RawBalance,
		FormattedBalance: value.FormattedBalance, ObservedAt: timestamppb.New(value.ObservedAt),
	}
}

func (s *BalanceService) storeCache(ctx context.Context, key string, response *web3v1.GetBalanceResponse, observedAt time.Time) {
	value := cachedBalance{
		ChainID: response.GetChainId(), Address: response.GetAddress(), TokenContract: response.GetTokenContract(),
		Symbol: response.GetSymbol(), Decimals: response.GetDecimals(), RawBalance: response.GetRawBalance(),
		FormattedBalance: response.GetFormattedBalance(), ObservedAt: observedAt,
	}
	payload, err := json.Marshal(value)
	if err == nil {
		_ = s.cache.Set(ctx, key, payload, s.cacheTTL)
	}
}

// formatUnits 将链上整数最小单位转换为十进制展示字符串；不使用 float64，避免精度丢失。
func formatUnits(value *big.Int, decimals uint32) string {
	if value == nil {
		return "0"
	}
	if decimals == 0 {
		return value.String()
	}
	digits := value.String()
	negative := strings.HasPrefix(digits, "-")
	if negative {
		digits = strings.TrimPrefix(digits, "-")
	}
	for len(digits) <= int(decimals) {
		digits = "0" + digits
	}
	split := len(digits) - int(decimals)
	formatted := digits[:split] + "." + digits[split:]
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		formatted = "0"
	}
	if negative && formatted != "0" {
		formatted = "-" + formatted
	}
	return formatted
}
