package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"web3/internal/blockchain"
	"web3/internal/config"
	"web3/internal/model"
	"web3/pkg/apperror"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// scannerCursorRepository 保存每条链已经完成处理的区块位置。
// scannerCursorRepository 保存扫描断点和链事件去重记录，使扫描器重启后能够续跑。
type scannerCursorRepository interface {
	GetCursor(context.Context, uint64) (*model.ChainScanCursor, error)
	SaveCursor(context.Context, *model.ChainScanCursor) error
	CreateEventIfAbsent(context.Context, *model.ChainEventReceipt) (bool, error)
	MarkEventProcessed(context.Context, string, time.Time) error
	MarkEventFailed(context.Context, string, string) error
}

// erc20TransferTopic 是 ERC-20 Transfer(address,address,uint256) 事件的 topic0。
var erc20TransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// ScannerService 负责扫描已达到确认数的原生币交易和 ERC-20 Transfer 事件。
type ScannerService struct {
	wallets      walletRepository
	transactions transactionRepository
	cursors      scannerCursorRepository
	chains       chainExecutor
	config       config.ScannerConfig
}

// NewScannerService 创建链上扫描服务；扫描开关与频率由 ScannerConfig 控制。
func NewScannerService(
	wallets walletRepository,
	transactions transactionRepository,
	cursors scannerCursorRepository,
	chains chainExecutor,
	scannerConfig config.ScannerConfig,
) (*ScannerService, error) {
	if wallets == nil || transactions == nil || cursors == nil || chains == nil {
		return nil, apperror.InvalidArgument("scanner service dependencies are incomplete")
	}
	if scannerConfig.BatchSize <= 0 {
		return nil, apperror.InvalidArgument("scanner batch size is invalid")
	}
	return &ScannerService{
		wallets: wallets, transactions: transactions, cursors: cursors, chains: chains, config: scannerConfig,
	}, nil
}

// Start 在后台定时扫描所有启用的区块链，并返回一个等待 goroutine 退出的停止函数。
// 调用方必须在关闭 RPC 客户端前调用停止函数，避免扫描过程中使用已关闭的连接。
// Start 启动后台扫描协程，并返回幂等停止函数。
// 服务关闭时必须先停止扫描器，再关闭 RPC 客户端，避免扫描中途使用已关闭连接。
func (s *ScannerService) Start(parent context.Context, chains []config.BlockchainChainConfig, logger *zap.Logger) func() {
	if s == nil || !s.config.Enabled {
		return func() {}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	interval, err := time.ParseDuration(s.config.PollInterval)
	if err != nil || interval <= 0 {
		logger.Error("Web3 scanner is disabled because its poll interval is invalid", zap.Error(err))
		return func() {}
	}
	chainIDs := enabledChainIDs(chains)
	if len(chainIDs) == 0 {
		logger.Warn("Web3 scanner is enabled but no blockchain is enabled")
		return func() {}
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.scanEnabledChains(ctx, chainIDs, logger)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.scanEnabledChains(ctx, chainIDs, logger)
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

// scanEnabledChains 串行扫描每条已启用的链；一条链失败不会阻断其他链。
func (s *ScannerService) scanEnabledChains(ctx context.Context, chainIDs []uint64, logger *zap.Logger) {
	for _, chainID := range chainIDs {
		if err := ctx.Err(); err != nil {
			return
		}
		processed, err := s.ScanChainOnce(ctx, chainID)
		if err != nil {
			logger.Warn("Web3 chain scan failed", zap.Uint64("chain_id", chainID), zap.Error(err))
			continue
		}
		if processed > 0 {
			logger.Debug("Web3 chain scan completed", zap.Uint64("chain_id", chainID), zap.Int("processed_blocks", processed))
		}
	}
}

func enabledChainIDs(chains []config.BlockchainChainConfig) []uint64 {
	chainIDs := make([]uint64, 0, len(chains))
	for _, chain := range chains {
		if chain.Enabled {
			chainIDs = append(chainIDs, chain.ChainID)
		}
	}
	return chainIDs
}

// ScanChainOnce 从断点开始扫描最多 BatchSize 个已经达到确认数的区块。
// 每成功处理一个区块才推进断点；进程中断时会重扫最后一个未推进的区块，
// TransactionRepository.Upsert 保证这种重试不会重复写入。
// ScanChainOnce 从持久化断点向前扫描最多 BatchSize 个“已达到确认数”的区块。
// 仅当一个区块完整落库后才推进断点，因此失败重试不会漏数据。
func (s *ScannerService) ScanChainOnce(ctx context.Context, chainID uint64) (int, error) {
	chainCfg, ok := s.chains.Chain(chainID)
	if !ok || !chainCfg.Enabled {
		return 0, apperror.InvalidArgument("scanner blockchain is disabled")
	}
	latest, err := s.latestBlock(ctx, chainID)
	if err != nil {
		return 0, err
	}
	// 区块自身计为第 1 次确认：required_confirmations=1 时可以处理最新块。
	// 例如要求 12 次确认，最新高度为 100 时，最晚安全块是 89（100-89+1=12）。
	safeHead := latest
	if chainCfg.RequiredConfirmations > 1 {
		delay := chainCfg.RequiredConfirmations - 1
		if latest < delay {
			return 0, nil
		}
		safeHead = latest - delay
	}
	nextBlock, initialized, err := s.nextBlock(ctx, chainID, safeHead)
	if err != nil {
		return 0, err
	}
	if initialized || nextBlock > safeHead {
		return 0, nil
	}

	processed := 0
	for blockNumber := nextBlock; blockNumber <= safeHead && processed < s.config.BatchSize; blockNumber++ {
		block, err := s.blockByNumber(ctx, chainID, blockNumber)
		if err != nil {
			return processed, err
		}
		if err := s.scanBlock(ctx, chainID, latest, block); err != nil {
			return processed, err
		}
		now := time.Now().UTC()
		if err := s.cursors.SaveCursor(ctx, &model.ChainScanCursor{
			ChainID: chainID, BlockNumber: int64(blockNumber), BlockHash: block.Hash().Hex(), LastScannedAt: now, UpdatedAt: now,
		}); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// nextBlock 根据断点找出下一个可安全处理的高度；首次启动按 StartBlock 或安全高度初始化。
func (s *ScannerService) nextBlock(ctx context.Context, chainID, safeHead uint64) (uint64, bool, error) {
	cursor, err := s.cursors.GetCursor(ctx, chainID)
	if err == nil {
		if cursor.BlockNumber < 0 {
			return 0, false, apperror.InvalidArgument("scanner cursor block number is invalid")
		}
		return uint64(cursor.BlockNumber) + 1, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, err
	}
	if s.config.StartBlock > 0 {
		return s.config.StartBlock, false, nil
	}

	// 未明确要求补历史数据时，从当前安全块开始记录，避免首次启动扫描整条链。
	block, err := s.blockByNumber(ctx, chainID, safeHead)
	if err != nil {
		return 0, false, err
	}
	now := time.Now().UTC()
	if err := s.cursors.SaveCursor(ctx, &model.ChainScanCursor{
		ChainID: chainID, BlockNumber: int64(safeHead), BlockHash: block.Hash().Hex(), LastScannedAt: now, UpdatedAt: now,
	}); err != nil {
		return 0, false, err
	}
	return 0, true, nil
}

func (s *ScannerService) latestBlock(ctx context.Context, chainID uint64) (uint64, error) {
	var latest uint64
	err := s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		latest, callErr = client.BlockNumber(requestCtx)
		return callErr
	})
	return latest, err
}

func (s *ScannerService) blockByNumber(ctx context.Context, chainID, blockNumber uint64) (*types.Block, error) {
	var block *types.Block
	err := s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		block, callErr = client.BlockByNumber(requestCtx, new(big.Int).SetUint64(blockNumber))
		return callErr
	})
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, apperror.NotFound(fmt.Sprintf("block %d was not found", blockNumber))
	}
	return block, nil
}

// scanBlock 同时处理区块内原生币交易和 ERC-20 Transfer 日志。
func (s *ScannerService) scanBlock(ctx context.Context, chainID, latest uint64, block *types.Block) error {
	for _, transaction := range block.Transactions() {
		if err := s.scanTransaction(ctx, chainID, latest, block, transaction); err != nil {
			return err
		}
	}
	return s.scanERC20Transfers(ctx, chainID, latest, block)
}

// scanERC20Transfers 分别按 from 和 to 查询标准 Transfer 事件。
// Ethereum 日志过滤不能用一个请求表达 topic1 OR topic2，因此必须分别查两次再去重。
// scanERC20Transfers 只查询与已绑定钱包地址有关的 Transfer 日志，避免扫描全链全部代币事件。
func (s *ScannerService) scanERC20Transfers(ctx context.Context, chainID, latest uint64, block *types.Block) error {
	wallets, err := s.wallets.ListActiveByChain(ctx, chainID)
	if err != nil || len(wallets) == 0 {
		return err
	}
	walletByAddress := make(map[string]*model.Wallet, len(wallets))
	walletTopics := make([]common.Hash, 0, len(wallets))
	for index := range wallets {
		wallet := &wallets[index]
		address := common.HexToAddress(wallet.Address)
		walletByAddress[strings.ToLower(address.Hex())] = wallet
		walletTopics = append(walletTopics, addressTopic(address))
	}

	logs, err := s.transferLogs(ctx, chainID, block.Number(), walletTopics)
	if err != nil {
		return err
	}
	for _, log := range logs {
		if err := s.persistTransferLog(ctx, chainID, latest, block, walletByAddress, log); err != nil {
			return err
		}
	}
	return nil
}

// transferLogs 分别按 from 与 to 查询日志并去重，因为同一地址可能同时是发送方和接收方。
func (s *ScannerService) transferLogs(ctx context.Context, chainID uint64, blockNumber *big.Int, walletTopics []common.Hash) ([]types.Log, error) {
	queries := []ethereum.FilterQuery{
		{FromBlock: blockNumber, ToBlock: blockNumber, Topics: [][]common.Hash{{erc20TransferTopic}, walletTopics, nil}},
		{FromBlock: blockNumber, ToBlock: blockNumber, Topics: [][]common.Hash{{erc20TransferTopic}, nil, walletTopics}},
	}
	seen := make(map[string]struct{})
	result := make([]types.Log, 0)
	for _, query := range queries {
		var logs []types.Log
		err := s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
			var callErr error
			logs, callErr = client.FilterLogs(requestCtx, query)
			return callErr
		})
		if err != nil {
			return nil, err
		}
		for _, log := range logs {
			key := fmt.Sprintf("%s:%d", log.TxHash.Hex(), log.Index)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, log)
		}
	}
	return result, nil
}

// persistTransferLog 将一条 ERC-20 日志转换为对应钱包的交易快照，并通过事件收据保证幂等。
func (s *ScannerService) persistTransferLog(
	ctx context.Context,
	chainID, latest uint64,
	block *types.Block,
	walletByAddress map[string]*model.Wallet,
	log types.Log,
) error {
	if len(log.Topics) != 3 || log.Topics[0] != erc20TransferTopic || len(log.Data) != 32 {
		return nil
	}
	from := topicAddress(log.Topics[1])
	to := topicAddress(log.Topics[2])
	participants := make(map[string]*model.Wallet, 2)
	if wallet := walletByAddress[strings.ToLower(from)]; wallet != nil {
		participants[wallet.ID] = wallet
	}
	if wallet := walletByAddress[strings.ToLower(to)]; wallet != nil {
		participants[wallet.ID] = wallet
	}
	if len(participants) == 0 {
		return nil
	}

	eventID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%d:%s:%d", chainID, log.TxHash.Hex(), log.Index))).String()
	payload, err := json.Marshal(map[string]string{"from": from, "to": to, "value": new(big.Int).SetBytes(log.Data).String()})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.cursors.CreateEventIfAbsent(ctx, &model.ChainEventReceipt{
		ID: eventID, ChainID: chainID, TransactionHash: log.TxHash.Hex(), LogIndex: uint32(log.Index),
		BlockNumber: int64(block.NumberU64()), BlockHash: block.Hash().Hex(), ContractAddress: log.Address.Hex(),
		Topic0: erc20TransferTopic.Hex(), Payload: payload, Status: model.ChainEventStatusPending,
		ObservedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	status, err := s.minedTransactionStatus(ctx, chainID, log.TxHash)
	if err != nil {
		return err
	}

	for _, wallet := range participants {
		direction, belongs := transactionDirection(wallet.Address, from, to)
		if !belongs {
			continue
		}
		blockNumber := int64(block.NumberU64())
		blockHash := block.Hash().Hex()
		blockTime := time.Unix(int64(block.Time()), 0).UTC()
		contract := log.Address.Hex()
		snapshot := &model.WalletTransaction{
			ID: uuid.NewString(), WalletID: wallet.ID, ChainID: chainID, TransactionHash: log.TxHash.Hex(), EventIndex: int32(log.Index),
			Direction: direction, FromAddress: from, ToAddress: to, ValueRaw: new(big.Int).SetBytes(log.Data).String(), TokenContract: &contract,
			BlockNumber: &blockNumber, BlockHash: &blockHash, Confirmations: latest - block.NumberU64() + 1,
			Status: status, BlockTime: &blockTime, FirstSeenAt: now, UpdatedAt: now,
		}
		if err := s.transactions.Upsert(ctx, snapshot); err != nil {
			_ = s.cursors.MarkEventFailed(ctx, eventID, err.Error())
			return err
		}
	}
	return s.cursors.MarkEventProcessed(ctx, eventID, time.Now().UTC())
}

func addressTopic(address common.Address) common.Hash {
	return common.BytesToHash(common.LeftPadBytes(address.Bytes(), common.HashLength))
}

func topicAddress(topic common.Hash) string {
	return common.BytesToAddress(topic.Bytes()[common.HashLength-common.AddressLength:]).Hex()
}

// scanTransaction 处理区块中的原生币交易；仅当 from 或 to 是已绑定钱包时才写入快照。
func (s *ScannerService) scanTransaction(ctx context.Context, chainID, latest uint64, block *types.Block, transaction *types.Transaction) error {
	from, err := transactionSender(transaction)
	if err != nil {
		return nil // 无法恢复发送方的异常交易不应阻断整块扫描。
	}
	to := ""
	if transaction.To() != nil {
		to = transaction.To().Hex()
	}

	wallets := make(map[string]*model.Wallet, 2)
	for _, address := range []string{from, to} {
		if strings.TrimSpace(address) == "" {
			continue
		}
		wallet, findErr := s.wallets.FindByChainAddress(ctx, chainID, address)
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			continue
		}
		if findErr != nil {
			return findErr
		}
		if wallet != nil && wallet.Status == model.WalletStatusActive {
			wallets[wallet.ID] = wallet
		}
	}
	if len(wallets) == 0 {
		return nil
	}
	status, err := s.minedTransactionStatus(ctx, chainID, transaction.Hash())
	if err != nil {
		return err
	}

	for _, wallet := range wallets {
		direction, belongs := transactionDirection(wallet.Address, from, to)
		if !belongs {
			continue
		}
		blockNumber := int64(block.NumberU64())
		blockHash := block.Hash().Hex()
		blockTime := time.Unix(int64(block.Time()), 0).UTC()
		now := time.Now().UTC()
		snapshot := &model.WalletTransaction{
			ID: uuid.NewString(), WalletID: wallet.ID, ChainID: chainID, TransactionHash: transaction.Hash().Hex(), EventIndex: -1,
			Direction: direction, FromAddress: from, ToAddress: to, ValueRaw: transaction.Value().String(),
			BlockNumber: &blockNumber, BlockHash: &blockHash, Confirmations: latest - block.NumberU64() + 1,
			Status: status, BlockTime: &blockTime, FirstSeenAt: now, UpdatedAt: now,
		}
		if err := s.transactions.Upsert(ctx, snapshot); err != nil {
			return err
		}
	}
	return nil
}

// minedTransactionStatus 以交易回执为准区分“已上链成功”和“已上链但执行失败”。
// 区块包含交易只说明交易被打包，不代表智能合约调用一定成功。
// minedTransactionStatus 读取回执，将链上执行成功/失败映射为本地交易状态。
func (s *ScannerService) minedTransactionStatus(ctx context.Context, chainID uint64, hash common.Hash) (model.TransactionStatus, error) {
	var receipt *types.Receipt
	err := s.chains.Execute(ctx, chainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		receipt, callErr = client.TransactionReceipt(requestCtx, hash)
		return callErr
	})
	if err != nil {
		return 0, err
	}
	if receipt == nil {
		return 0, apperror.NotFound("transaction receipt was not found")
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return model.TransactionStatusFailed, nil
	}
	return model.TransactionStatusConfirmed, nil
}
