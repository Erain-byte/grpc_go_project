// Package svc 集中保存 Web3 服务进程内需要共享的长期依赖。
package svc

import (
	"context"

	"web3/internal/blockchain"
	"web3/internal/config"
	"web3/internal/consul"
	"web3/internal/databases"
	redisstore "web3/internal/redis"
	"web3/internal/repository"
	"web3/internal/service"
	"web3/internal/sharding"
	"web3/internal/tron"
	"web3/internal/wallet"
	"web3/pkg/apperror"

	"go.uber.org/zap"
)

type ServiceContext struct {
	Config       *config.Config
	Consul       *consul.ConsulRegistry
	Redis        redisstore.Client
	Database     *databases.GormClient
	Router       *sharding.TransactionRouter
	WalletRepo   *repository.WalletRepository
	TxRepo       *repository.TransactionRepository
	ScannerRepo  *repository.ScannerRepository
	UsageRepo    *repository.UsageRepository
	Wallets      *service.WalletService
	Balances     *service.BalanceService
	Transactions *service.TransactionService
	Approvals    *service.TronApprovalService
	Scanner      *service.ScannerService
	Blockchain   *blockchain.ClientManager
	Tron         *tron.Client
	Logger       *zap.Logger
}

func NewServiceContext(
	cfg *config.Config,
	consulRegistry *consul.ConsulRegistry,
	redisClient redisstore.Client,
	database *databases.GormClient,
	logger *zap.Logger,
) (*ServiceContext, error) {
	if cfg == nil || redisClient == nil || database == nil || logger == nil {
		return nil, apperror.InvalidArgument("Web3 service context dependencies are incomplete")
	}
	db := database.Write(context.Background())
	if db == nil {
		return nil, apperror.InvalidArgument("Web3 database is not initialized")
	}

	router := sharding.NewTransactionRouter()
	walletRepo := repository.NewWalletRepository(db)
	nonceStore, err := wallet.NewNonceStore(redisClient, cfg.WalletSignature)
	if err != nil {
		return nil, err
	}
	walletService, err := service.NewWalletService(
		walletRepo,
		nonceStore,
		wallet.NewEIP191Verifier(),
		cfg.Blockchain,
	)
	if err != nil {
		return nil, err
	}
	blockchainClients, err := blockchain.NewClientManager(cfg.Blockchain)
	if err != nil {
		return nil, err
	}
	balanceService, err := service.NewBalanceService(walletRepo, blockchainClients, redisClient, cfg.Blockchain)
	if err != nil {
		blockchainClients.Close()
		return nil, err
	}
	transactionRepo := repository.NewTransactionRepository(db, router)
	transactionService, err := service.NewTransactionService(walletRepo, transactionRepo, blockchainClients)
	if err != nil {
		blockchainClients.Close()
		return nil, err
	}
	tronClient, err := tron.NewClient(cfg.TronApprovals, logger.Named("tron"))
	if err != nil {
		blockchainClients.Close()
		return nil, err
	}
	usageRepo := repository.NewUsageRepository(db)
	approvalService, err := service.NewTronApprovalService(
		tronClient,
		redisClient,
		usageRepo,
		logger.Named("approval"),
		cfg.TronApprovals,
	)
	if err != nil {
		blockchainClients.Close()
		return nil, err
	}
	scannerRepo := repository.NewScannerRepository(db)
	scannerService, err := service.NewScannerService(walletRepo, transactionRepo, scannerRepo, blockchainClients, cfg.Blockchain.Scanner)
	if err != nil {
		blockchainClients.Close()
		return nil, err
	}

	return &ServiceContext{
		Config:       cfg,
		Consul:       consulRegistry,
		Redis:        redisClient,
		Database:     database,
		Router:       router,
		WalletRepo:   walletRepo,
		TxRepo:       transactionRepo,
		ScannerRepo:  scannerRepo,
		UsageRepo:    usageRepo,
		Wallets:      walletService,
		Balances:     balanceService,
		Transactions: transactionService,
		Approvals:    approvalService,
		Scanner:      scannerService,
		Blockchain:   blockchainClients,
		Tron:         tronClient,
		Logger:       logger,
	}, nil
}
