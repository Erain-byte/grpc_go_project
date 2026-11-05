package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"web3/internal/blockchain"
	"web3/internal/model"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

const (
	defaultTransactionPageSize = 20
	maxTransactionPageSize     = 100
)

// transactionRepository 定义交易快照的分表读写能力。
type transactionRepository interface {
	Upsert(context.Context, *model.WalletTransaction) error
	FindByHash(context.Context, string, uint64, string) (*model.WalletTransaction, error)
	ListByWallet(context.Context, string, model.TransactionStatus, int, int) ([]model.WalletTransaction, int64, error)
}

// TransactionService 查询由 Scanner 写入本地分表的交易快照。
// 链节点补查在下一步接入，当前不存在记录时明确返回 NotFound。
// TransactionService 优先读取 Scanner 写入的本地交易快照；缓存未命中时才补查链上节点。
type TransactionService struct {
	wallets      walletRepository
	transactions transactionRepository
	chains       chainExecutor
}

// NewTransactionService 创建交易查询服务。
func NewTransactionService(wallets walletRepository, transactions transactionRepository, chains chainExecutor) (*TransactionService, error) {
	if wallets == nil || transactions == nil || chains == nil {
		return nil, apperror.InvalidArgument("transaction service dependencies are incomplete")
	}
	return &TransactionService{wallets: wallets, transactions: transactions, chains: chains}, nil
}

// GetTransaction 查询某一笔交易，并确认该交易属于当前用户的钱包。
func (s *TransactionService) GetTransaction(ctx context.Context, req *web3v1.GetTransactionRequest) (*web3v1.GetTransactionResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetWalletId()) == "" {
		return nil, apperror.InvalidArgument("wallet ID is empty")
	}
	if req.GetChainId() == 0 || !common.IsHexHash(req.GetTransactionHash()) {
		return nil, apperror.InvalidArgument("chain ID or transaction hash is invalid")
	}
	wallet, err := s.activeWallet(ctx, identity.Subject, req.GetWalletId(), req.GetChainId())
	if err != nil {
		return nil, err
	}
	transaction, err := s.transactions.FindByHash(ctx, wallet.ID, req.GetChainId(), common.HexToHash(req.GetTransactionHash()).Hex())
	if err == nil {
		return &web3v1.GetTransactionResponse{Transaction: transactionToProto(transaction)}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	transaction, err = s.queryChainTransaction(ctx, wallet, common.HexToHash(req.GetTransactionHash()))
	if err != nil {
		return nil, err
	}
	return &web3v1.GetTransactionResponse{Transaction: transactionToProto(transaction)}, nil
}

// queryChainTransaction 在本地没有快照时从 RPC 节点补查原生币交易，并写回本地分表。
func (s *TransactionService) queryChainTransaction(ctx context.Context, wallet *model.Wallet, hash common.Hash) (*model.WalletTransaction, error) {
	chainCfg, ok := s.chains.Chain(wallet.ChainID)
	if !ok {
		return nil, apperror.InvalidArgument("wallet blockchain is disabled")
	}
	var transaction *types.Transaction
	var pending bool
	err := s.chains.Execute(ctx, wallet.ChainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		transaction, pending, callErr = client.TransactionByHash(requestCtx, hash)
		return callErr
	})
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil, apperror.NotFound("transaction was not found")
		}
		return nil, err
	}
	if transaction == nil {
		return nil, apperror.NotFound("transaction was not found")
	}

	from, err := transactionSender(transaction)
	if err != nil {
		return nil, apperror.InvalidArgument("transaction sender cannot be recovered")
	}
	to := ""
	if transaction.To() != nil {
		to = transaction.To().Hex()
	}
	direction, belongsToWallet := transactionDirection(wallet.Address, from, to)
	if !belongsToWallet {
		return nil, apperror.NotFound("transaction does not belong to the requested wallet")
	}

	now := time.Now().UTC()
	snapshot := &model.WalletTransaction{
		ID:              uuid.NewString(),
		WalletID:        wallet.ID,
		ChainID:         wallet.ChainID,
		TransactionHash: hash.Hex(),
		EventIndex:      -1,
		Direction:       direction,
		FromAddress:     from,
		ToAddress:       to,
		ValueRaw:        transaction.Value().String(),
		Status:          model.TransactionStatusPending,
		FirstSeenAt:     now,
		UpdatedAt:       now,
	}
	if !pending {
		if err := s.enrichMinedTransaction(ctx, snapshot, chainCfg.RequiredConfirmations); err != nil {
			return nil, err
		}
	}
	if err := s.transactions.Upsert(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// enrichMinedTransaction 用回执、区块头和最新高度补齐确认数与最终交易状态。
func (s *TransactionService) enrichMinedTransaction(ctx context.Context, snapshot *model.WalletTransaction, requiredConfirmations uint64) error {
	hash := common.HexToHash(snapshot.TransactionHash)
	var receipt *types.Receipt
	err := s.chains.Execute(ctx, snapshot.ChainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		receipt, callErr = client.TransactionReceipt(requestCtx, hash)
		return callErr
	})
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil
		}
		return err
	}
	if receipt == nil || receipt.BlockNumber == nil {
		return nil
	}
	blockNumber := int64(receipt.BlockNumber.Uint64())
	blockHash := receipt.BlockHash.Hex()
	snapshot.BlockNumber = &blockNumber
	snapshot.BlockHash = &blockHash
	var latest uint64
	err = s.chains.Execute(ctx, snapshot.ChainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		latest, callErr = client.BlockNumber(requestCtx)
		return callErr
	})
	if err != nil {
		return err
	}
	if latest >= receipt.BlockNumber.Uint64() {
		snapshot.Confirmations = latest - receipt.BlockNumber.Uint64() + 1
	}
	var header *types.Header
	err = s.chains.Execute(ctx, snapshot.ChainID, func(requestCtx context.Context, client blockchain.Client) error {
		var callErr error
		header, callErr = client.HeaderByNumber(requestCtx, receipt.BlockNumber)
		return callErr
	})
	if err != nil {
		return err
	}
	if header != nil {
		blockTime := time.Unix(int64(header.Time), 0).UTC()
		snapshot.BlockTime = &blockTime
	}
	if receipt.Status == types.ReceiptStatusFailed {
		snapshot.Status = model.TransactionStatusFailed
	} else if requiredConfirmations == 0 || snapshot.Confirmations >= requiredConfirmations {
		snapshot.Status = model.TransactionStatusConfirmed
	}
	return nil
}

func transactionSender(transaction *types.Transaction) (string, error) {
	signer := types.LatestSignerForChainID(transaction.ChainId())
	address, err := types.Sender(signer, transaction)
	if err != nil {
		return "", err
	}
	return address.Hex(), nil
}

func transactionDirection(walletAddress, from, to string) (model.TransactionDirection, bool) {
	walletAddress = strings.ToLower(walletAddress)
	fromMatches := walletAddress == strings.ToLower(from)
	toMatches := walletAddress == strings.ToLower(to)
	switch {
	case fromMatches && toMatches:
		return model.TransactionDirectionSelf, true
	case fromMatches:
		return model.TransactionDirectionOutgoing, true
	case toMatches:
		return model.TransactionDirectionIncoming, true
	default:
		return 0, false
	}
}

// GetTransactions 分页读取当前用户指定钱包的本地交易快照。
func (s *TransactionService) GetTransactions(ctx context.Context, req *web3v1.GetTransactionsRequest) (*web3v1.GetTransactionsResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetWalletId()) == "" {
		return nil, apperror.InvalidArgument("wallet ID is empty")
	}
	wallet, err := s.activeWallet(ctx, identity.Subject, req.GetWalletId(), 0)
	if err != nil {
		return nil, err
	}
	status, err := transactionStatusFromProto(req.GetStatus())
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizeTransactionPage(req.GetPage(), req.GetPageSize())
	transactions, total, err := s.transactions.ListByWallet(ctx, wallet.ID, status, page, pageSize)
	if err != nil {
		return nil, err
	}
	response := &web3v1.GetTransactionsResponse{
		Transactions: make([]*web3v1.TransactionInfo, 0, len(transactions)),
		Total:        total,
		Page:         int32(page),
		PageSize:     int32(pageSize),
	}
	for index := range transactions {
		response.Transactions = append(response.Transactions, transactionToProto(&transactions[index]))
	}
	return response, nil
}

func (s *TransactionService) activeWallet(ctx context.Context, subject, walletID string, chainID uint64) (*model.Wallet, error) {
	wallet, err := s.wallets.FindByID(ctx, subject, walletID)
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	if wallet.Status != model.WalletStatusActive {
		return nil, apperror.NotFound("active wallet was not found")
	}
	if chainID != 0 && wallet.ChainID != chainID {
		return nil, apperror.InvalidArgument("wallet does not belong to the requested blockchain")
	}
	return wallet, nil
}

func transactionStatusFromProto(status web3v1.TransactionStatus) (model.TransactionStatus, error) {
	switch status {
	case web3v1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED:
		return 0, nil
	case web3v1.TransactionStatus_TRANSACTION_STATUS_PENDING:
		return model.TransactionStatusPending, nil
	case web3v1.TransactionStatus_TRANSACTION_STATUS_CONFIRMED:
		return model.TransactionStatusConfirmed, nil
	case web3v1.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return model.TransactionStatusFailed, nil
	case web3v1.TransactionStatus_TRANSACTION_STATUS_REORGED:
		return model.TransactionStatusReorged, nil
	default:
		return 0, apperror.InvalidArgument("transaction status is invalid")
	}
}

func normalizeTransactionPage(page, pageSize int32) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultTransactionPageSize
	}
	if pageSize > maxTransactionPageSize {
		pageSize = maxTransactionPageSize
	}
	return int(page), int(pageSize)
}

func transactionToProto(value *model.WalletTransaction) *web3v1.TransactionInfo {
	if value == nil {
		return nil
	}
	response := &web3v1.TransactionInfo{
		ChainId:         value.ChainID,
		TransactionHash: value.TransactionHash,
		FromAddress:     value.FromAddress,
		ToAddress:       value.ToAddress,
		Value:           value.ValueRaw,
		Confirmations:   value.Confirmations,
		Status:          web3v1.TransactionStatus(value.Status),
		EventIndex:      value.EventIndex,
	}
	if value.TokenContract != nil {
		response.TokenContract = *value.TokenContract
	}
	if value.BlockNumber != nil {
		response.BlockNumber = uint64(*value.BlockNumber)
	}
	if value.BlockHash != nil {
		response.BlockHash = *value.BlockHash
	}
	if value.BlockTime != nil {
		response.BlockTime = timestamppb.New(*value.BlockTime)
	}
	return response
}
