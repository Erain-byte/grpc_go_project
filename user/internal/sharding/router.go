package sharding

import (
	"fmt"
	"regexp"

	"user/internal/config"
)

var validTablePrefix = regexp.MustCompile(`^[a-z][a-z0-9_]*_$`)

const (
	userWalletTablePrefix          = "user_wallets_"
	userFundTransactionTablePrefix = "user_fund_transactions_"
)

type UserRouter struct {
	tableCount uint64
	tableNames []string
}

func NewUserRouter(cfg config.ShardingConfig) (*UserRouter, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("user sharding is disabled")
	}
	if cfg.TableCount != 8 {
		return nil, fmt.Errorf("user shard table count must be 8")
	}
	if !validTablePrefix.MatchString(cfg.TablePrefix) {
		return nil, fmt.Errorf("invalid user shard table prefix %q", cfg.TablePrefix)
	}

	tableNames := make([]string, cfg.TableCount)
	for index := range tableNames {
		tableNames[index] = fmt.Sprintf("%s%02d", cfg.TablePrefix, index)
	}
	return &UserRouter{tableCount: uint64(cfg.TableCount), tableNames: tableNames}, nil
}

func (r *UserRouter) TableIndex(userID int64) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("user ID must be positive")
	}
	return int(uint64(userID) % r.tableCount), nil
}

func (r *UserRouter) TableName(userID int64) (string, error) {
	index, err := r.TableIndex(userID)
	if err != nil {
		return "", err
	}
	return r.tableNames[index], nil
}

func (r *UserRouter) TableNameByIndex(index int) (string, error) {
	if index < 0 || index >= len(r.tableNames) {
		return "", fmt.Errorf("user shard table index %d is outside 0..%d", index, len(r.tableNames)-1)
	}
	return r.tableNames[index], nil
}

func (r *UserRouter) TableNames() []string {
	return append([]string(nil), r.tableNames...)
}

func (r *UserRouter) WalletTableName(userID int64) (string, error) {
	index, err := r.TableIndex(userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", userWalletTablePrefix, index), nil
}

func (r *UserRouter) FundTransactionTableName(userID int64) (string, error) {
	index, err := r.TableIndex(userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", userFundTransactionTablePrefix, index), nil
}

func (r *UserRouter) WalletTableNameByIndex(index int) (string, error) {
	if _, err := r.TableNameByIndex(index); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", userWalletTablePrefix, index), nil
}

func (r *UserRouter) FundTransactionTableNameByIndex(index int) (string, error) {
	if _, err := r.TableNameByIndex(index); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", userFundTransactionTablePrefix, index), nil
}
