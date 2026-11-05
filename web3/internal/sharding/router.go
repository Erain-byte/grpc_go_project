// Package sharding 负责将钱包交易稳定路由到固定分表。
package sharding

import (
	"fmt"
	"hash/fnv"
	"strings"
)

const transactionTableCount = 8
const transactionTablePrefix = "wallet_transactions_"

type TransactionRouter struct {
	tableNames [transactionTableCount]string
}

func NewTransactionRouter() *TransactionRouter {
	router := &TransactionRouter{}
	for index := range router.tableNames {
		router.tableNames[index] = fmt.Sprintf("%s%02d", transactionTablePrefix, index)
	}
	return router
}

func (r *TransactionRouter) TableIndex(walletID string) (int, error) {
	walletID = strings.ToLower(strings.TrimSpace(walletID))
	if walletID == "" {
		return 0, fmt.Errorf("wallet ID is empty")
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(walletID))
	return int(hasher.Sum64() % transactionTableCount), nil
}

func (r *TransactionRouter) TableName(walletID string) (string, error) {
	index, err := r.TableIndex(walletID)
	if err != nil {
		return "", err
	}
	return r.tableNames[index], nil
}

func (r *TransactionRouter) TableNameByIndex(index int) (string, error) {
	if index < 0 || index >= len(r.tableNames) {
		return "", fmt.Errorf("transaction shard index %d is outside 0..%d", index, len(r.tableNames)-1)
	}
	return r.tableNames[index], nil
}

func (r *TransactionRouter) TableNames() []string {
	names := make([]string, len(r.tableNames))
	copy(names, r.tableNames[:])
	return names
}
