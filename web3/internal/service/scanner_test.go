package service

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"web3/internal/auth"
	"web3/internal/config"
	"web3/internal/model"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
)

type fakeScannerRepository struct {
	cursor         *model.ChainScanCursor
	processedEvent string
	createdEvent   *model.ChainEventReceipt
}

func (r *fakeScannerRepository) GetCursor(context.Context, uint64) (*model.ChainScanCursor, error) {
	if r.cursor == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.cursor, nil
}
func (r *fakeScannerRepository) SaveCursor(_ context.Context, cursor *model.ChainScanCursor) error {
	r.cursor = cursor
	return nil
}
func (r *fakeScannerRepository) CreateEventIfAbsent(_ context.Context, event *model.ChainEventReceipt) (bool, error) {
	r.createdEvent = event
	return true, nil
}
func (r *fakeScannerRepository) MarkEventProcessed(_ context.Context, eventID string, _ time.Time) error {
	r.processedEvent = eventID
	return nil
}
func (*fakeScannerRepository) MarkEventFailed(context.Context, string, string) error { return nil }

type fakeScannerClient struct {
	fakeBalanceClient
	latest uint64
	blocks map[uint64]*types.Block
	logs   []types.Log
}

func (c *fakeScannerClient) BlockNumber(context.Context) (uint64, error) { return c.latest, nil }
func (c *fakeScannerClient) BlockByNumber(_ context.Context, number *big.Int) (*types.Block, error) {
	block := c.blocks[number.Uint64()]
	if block == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return block, nil
}
func (c *fakeScannerClient) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return c.logs, nil
}
func (*fakeScannerClient) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return &types.Receipt{Status: types.ReceiptStatusSuccessful}, nil
}

func TestScanChainOnceWritesBoundWalletTransactionAndCursor(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	from := crypto.PubkeyToAddress(privateKey.PublicKey)
	to := common.HexToAddress(testAddress)
	chainID := big.NewInt(1)
	transaction, err := types.SignTx(types.NewTx(&types.LegacyTx{
		Nonce: 1, To: &to, Value: big.NewInt(99), Gas: 21000, GasPrice: big.NewInt(1),
	}), types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		t.Fatalf("SignTx() error = %v", err)
	}
	block := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(10), Time: uint64(time.Now().Unix())}).WithBody(types.Body{
		Transactions: []*types.Transaction{transaction},
	})
	wallet := &model.Wallet{ID: "wallet-1", ChainID: 1, Address: from.Hex(), Status: model.WalletStatusActive}
	wallets := &fakeWalletRepository{byChainAddress: map[string]*model.Wallet{
		"1:" + strings.ToLower(from.Hex()): wallet,
	}}
	transactions := &fakeTransactionRepository{}
	cursors := &fakeScannerRepository{}
	service, err := NewScannerService(wallets, transactions, cursors, &fakeChainExecutor{
		client: &fakeScannerClient{fakeBalanceClient: fakeBalanceClient{balance: big.NewInt(0)}, latest: 12, blocks: map[uint64]*types.Block{10: block}},
		config: config.BlockchainChainConfig{ChainID: 1, Enabled: true, RequiredConfirmations: 3},
	}, config.ScannerConfig{BatchSize: 10, StartBlock: 10})
	if err != nil {
		t.Fatalf("NewScannerService() error = %v", err)
	}
	processed, err := service.ScanChainOnce(auth.NewContext(context.Background(), auth.Identity{Subject: testSubject}), 1)
	if err != nil {
		t.Fatalf("ScanChainOnce() error = %v", err)
	}
	if processed != 1 || transactions.upserted == nil {
		t.Fatalf("processed = %d, transaction = %+v", processed, transactions.upserted)
	}
	if transactions.upserted.WalletID != wallet.ID || transactions.upserted.ValueRaw != "99" || transactions.upserted.Confirmations != 3 {
		t.Fatalf("transaction = %+v", transactions.upserted)
	}
	if cursors.cursor == nil || cursors.cursor.BlockNumber != 10 {
		t.Fatalf("cursor = %+v", cursors.cursor)
	}
}

func TestScanChainOnceWritesERC20TransferWithLogIndex(t *testing.T) {
	from := common.HexToAddress(testAddress)
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	block := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(10), Time: uint64(time.Now().Unix())})
	transfer := types.Log{
		Address:     common.HexToAddress("0x0000000000000000000000000000000000000002"),
		Topics:      []common.Hash{erc20TransferTopic, addressTopic(from), addressTopic(to)},
		Data:        common.LeftPadBytes(big.NewInt(1234567).Bytes(), common.HashLength),
		BlockNumber: 10,
		BlockHash:   block.Hash(),
		TxHash:      common.HexToHash(testTransactionHash),
		Index:       7,
	}
	wallet := &model.Wallet{ID: "wallet-1", ChainID: 1, Address: from.Hex(), Status: model.WalletStatusActive}
	wallets := &fakeWalletRepository{byChainAddress: map[string]*model.Wallet{
		"1:" + strings.ToLower(from.Hex()): wallet,
	}}
	transactions := &fakeTransactionRepository{}
	cursors := &fakeScannerRepository{}
	service, err := NewScannerService(wallets, transactions, cursors, &fakeChainExecutor{
		client: &fakeScannerClient{
			fakeBalanceClient: fakeBalanceClient{balance: big.NewInt(0)}, latest: 12,
			blocks: map[uint64]*types.Block{10: block}, logs: []types.Log{transfer},
		},
		config: config.BlockchainChainConfig{ChainID: 1, Enabled: true, RequiredConfirmations: 3},
	}, config.ScannerConfig{BatchSize: 10, StartBlock: 10})
	if err != nil {
		t.Fatalf("NewScannerService() error = %v", err)
	}
	if _, err := service.ScanChainOnce(context.Background(), 1); err != nil {
		t.Fatalf("ScanChainOnce() error = %v", err)
	}
	if transactions.upserted == nil || transactions.upserted.EventIndex != 7 || transactions.upserted.TokenContract == nil {
		t.Fatalf("token snapshot = %+v", transactions.upserted)
	}
	if transactions.upserted.ValueRaw != "1234567" || transactions.upserted.Direction != model.TransactionDirectionOutgoing {
		t.Fatalf("token snapshot = %+v", transactions.upserted)
	}
	if cursors.createdEvent == nil || cursors.processedEvent == "" {
		t.Fatalf("event receipt = %+v, processed = %q", cursors.createdEvent, cursors.processedEvent)
	}
}
