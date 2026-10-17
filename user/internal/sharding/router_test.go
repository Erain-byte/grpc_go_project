package sharding

import (
	"testing"

	"user/internal/config"
)

func TestUserRouterCreatesEightStableTableNames(t *testing.T) {
	router, err := NewUserRouter(config.ShardingConfig{Enabled: true, TableCount: 8, TablePrefix: "users_"})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	if got := len(router.TableNames()); got != 8 {
		t.Fatalf("table count = %d, want 8", got)
	}
	if table, err := router.TableName(10); err != nil || table != "users_02" {
		t.Fatalf("user table = %q, err = %v", table, err)
	}
}

func TestUserRouterCoLocatesUserWalletAndFundTransactions(t *testing.T) {
	router, err := NewUserRouter(config.ShardingConfig{Enabled: true, TableCount: 8, TablePrefix: "users_"})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}

	userID := int64(18)
	walletTable, err := router.WalletTableName(userID)
	if err != nil {
		t.Fatalf("route wallet: %v", err)
	}
	transactionTable, err := router.FundTransactionTableName(userID)
	if err != nil {
		t.Fatalf("route transaction: %v", err)
	}
	if walletTable != "user_wallets_02" {
		t.Fatalf("wallet table = %q, want user_wallets_02", walletTable)
	}
	if transactionTable != "user_fund_transactions_02" {
		t.Fatalf("transaction table = %q, want user_fund_transactions_02", transactionTable)
	}
}

func TestUserRouterRejectsChangedTableCount(t *testing.T) {
	if _, err := NewUserRouter(config.ShardingConfig{Enabled: true, TableCount: 16, TablePrefix: "users_"}); err == nil {
		t.Fatal("router accepted a changed table count")
	}
}
