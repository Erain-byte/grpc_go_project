package repository

import (
	"context"

	"web3/internal/model"
	"web3/internal/sharding"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TransactionRepository 按 wallet_id 将交易快照路由到固定分表。
type TransactionRepository struct {
	db     *gorm.DB
	router *sharding.TransactionRouter
}

// NewTransactionRepository 创建交易分表仓库。
func NewTransactionRepository(db *gorm.DB, router *sharding.TransactionRouter) *TransactionRepository {
	return &TransactionRepository{db: db, router: router}
}

// Upsert 将重复扫描到的交易更新为最新确认状态，不重复插入记录。
func (r *TransactionRepository) Upsert(ctx context.Context, transaction *model.WalletTransaction) error {
	table, err := r.router.TableName(transaction.WalletID)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Table(table).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "wallet_id"}, {Name: "chain_id"}, {Name: "transaction_hash"}, {Name: "event_index"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"direction", "from_address", "to_address", "value_raw", "token_contract",
			"block_number", "block_hash", "confirmations", "status", "block_time", "updated_at",
		}),
	}).Create(transaction).Error
}

// FindByHash 返回同一交易哈希最早的事件记录；原生币记录的 event_index 为 -1。
func (r *TransactionRepository) FindByHash(ctx context.Context, walletID string, chainID uint64, hash string) (*model.WalletTransaction, error) {
	table, err := r.router.TableName(walletID)
	if err != nil {
		return nil, err
	}
	var transaction model.WalletTransaction
	err = r.db.WithContext(ctx).Table(table).
		Where("wallet_id = ? AND chain_id = ? AND transaction_hash = ?", walletID, chainID, hash).
		Order("event_index ASC").
		First(&transaction).Error
	return &transaction, err
}

// ListByWallet 按区块时间或首次发现时间倒序分页读取一个钱包的交易。
func (r *TransactionRepository) ListByWallet(
	ctx context.Context,
	walletID string,
	status model.TransactionStatus,
	page int,
	pageSize int,
) ([]model.WalletTransaction, int64, error) {
	table, err := r.router.TableName(walletID)
	if err != nil {
		return nil, 0, err
	}
	query := r.db.WithContext(ctx).Table(table).Where("wallet_id = ?", walletID)
	if status != 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var transactions []model.WalletTransaction
	err = query.Order("COALESCE(block_time, first_seen_at) DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&transactions).Error
	return transactions, total, err
}
