package service

import (
	"context"
	"math/big"
	"testing"
	"time"

	"web3/internal/auth"
	"web3/internal/config"
	"web3/internal/model"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
)

const testTransactionHash = "0x1111111111111111111111111111111111111111111111111111111111111111"

type fakeTransactionRepository struct {
	byHash       *model.WalletTransaction
	findErr      error
	upserted     *model.WalletTransaction
	values       []model.WalletTransaction
	total        int64
	listStatus   model.TransactionStatus
	listPage     int
	listPageSize int
}

func (r *fakeTransactionRepository) Upsert(_ context.Context, transaction *model.WalletTransaction) error {
	r.upserted = transaction
	return nil
}

func (r *fakeTransactionRepository) FindByHash(_ context.Context, _ string, _ uint64, _ string) (*model.WalletTransaction, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.byHash, nil
}

func (r *fakeTransactionRepository) ListByWallet(_ context.Context, _ string, status model.TransactionStatus, page, pageSize int) ([]model.WalletTransaction, int64, error) {
	r.listStatus = status
	r.listPage = page
	r.listPageSize = pageSize
	return r.values, r.total, nil
}

type fakeChainTransactionClient struct {
	fakeBalanceClient
	transaction *types.Transaction
	pending     bool
}

func (c *fakeChainTransactionClient) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	return c.transaction, c.pending, nil
}

func TestGetTransactionFallsBackToChainAndPersistsSnapshot(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	from := crypto.PubkeyToAddress(privateKey.PublicKey)
	to := common.HexToAddress(testAddress)
	chainID := big.NewInt(1)
	unsigned := types.NewTx(&types.LegacyTx{
		Nonce:    1,
		To:       &to,
		Value:    big.NewInt(2500),
		Gas:      21000,
		GasPrice: big.NewInt(1),
	})
	transaction, err := types.SignTx(unsigned, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		t.Fatalf("SignTx() error = %v", err)
	}

	wallets := &fakeWalletRepository{byAddress: &model.Wallet{
		ID: "wallet-1", UserSubject: testSubject, ChainID: 1, Address: from.Hex(), Status: model.WalletStatusActive,
	}}
	repository := &fakeTransactionRepository{findErr: gorm.ErrRecordNotFound}
	chainClient := &fakeChainTransactionClient{
		fakeBalanceClient: fakeBalanceClient{balance: big.NewInt(0)},
		transaction:       transaction,
		pending:           true,
	}
	service, err := NewTransactionService(wallets, repository, &fakeChainExecutor{
		client: chainClient,
		config: config.BlockchainChainConfig{ChainID: 1, Enabled: true},
	})
	if err != nil {
		t.Fatalf("NewTransactionService() error = %v", err)
	}

	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	response, err := service.GetTransaction(ctx, &web3v1.GetTransactionRequest{
		WalletId: "wallet-1", ChainId: 1, TransactionHash: transaction.Hash().Hex(),
	})
	if err != nil {
		t.Fatalf("GetTransaction() error = %v", err)
	}
	if repository.upserted == nil || repository.upserted.ValueRaw != "2500" {
		t.Fatalf("persisted snapshot = %+v", repository.upserted)
	}
	if response.GetTransaction().GetStatus() != web3v1.TransactionStatus_TRANSACTION_STATUS_PENDING {
		t.Fatalf("status = %s, want pending", response.GetTransaction().GetStatus())
	}
}

func TestGetTransactionRejectsWalletFromDifferentChain(t *testing.T) {
	wallets := &fakeWalletRepository{byAddress: &model.Wallet{
		ID: "wallet-1", UserSubject: testSubject, ChainID: 1, Status: model.WalletStatusActive,
	}}
	service, err := NewTransactionService(wallets, &fakeTransactionRepository{}, &fakeChainExecutor{config: config.BlockchainChainConfig{ChainID: 1}})
	if err != nil {
		t.Fatalf("NewTransactionService() error = %v", err)
	}
	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	_, err = service.GetTransaction(ctx, &web3v1.GetTransactionRequest{
		WalletId: "wallet-1", ChainId: 137, TransactionHash: testTransactionHash,
	})
	if err == nil {
		t.Fatal("GetTransaction() error = nil, want chain mismatch error")
	}
}

func TestGetTransactionsClampsPaginationAndMapsResponse(t *testing.T) {
	blockNumber := int64(123)
	blockTime := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	wallets := &fakeWalletRepository{byAddress: &model.Wallet{
		ID: "wallet-1", UserSubject: testSubject, ChainID: 1, Status: model.WalletStatusActive,
	}}
	repository := &fakeTransactionRepository{
		total: 1,
		values: []model.WalletTransaction{{
			WalletID: "wallet-1", ChainID: 1, TransactionHash: testTransactionHash,
			FromAddress: testAddress, ToAddress: testAddress, ValueRaw: "1000000",
			Status: model.TransactionStatusConfirmed, BlockNumber: &blockNumber, BlockTime: &blockTime,
		}},
	}
	service, err := NewTransactionService(wallets, repository, &fakeChainExecutor{config: config.BlockchainChainConfig{ChainID: 1}})
	if err != nil {
		t.Fatalf("NewTransactionService() error = %v", err)
	}
	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	response, err := service.GetTransactions(ctx, &web3v1.GetTransactionsRequest{
		WalletId: "wallet-1", Page: 0, PageSize: 500,
		Status: web3v1.TransactionStatus_TRANSACTION_STATUS_CONFIRMED,
	})
	if err != nil {
		t.Fatalf("GetTransactions() error = %v", err)
	}
	if repository.listPage != 1 || repository.listPageSize != maxTransactionPageSize {
		t.Fatalf("pagination = page %d size %d", repository.listPage, repository.listPageSize)
	}
	if repository.listStatus != model.TransactionStatusConfirmed {
		t.Fatalf("status = %d", repository.listStatus)
	}
	if response.GetTotal() != 1 || len(response.GetTransactions()) != 1 || response.GetTransactions()[0].GetBlockNumber() != 123 {
		t.Fatalf("response = %+v", response)
	}
}
