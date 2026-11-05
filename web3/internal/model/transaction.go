package model

import "time"

// TransactionStatus 表示本地观察到的链上交易状态。
type TransactionStatus int16

const (
	TransactionStatusPending   TransactionStatus = 1
	TransactionStatusConfirmed TransactionStatus = 2
	TransactionStatusFailed    TransactionStatus = 3
	TransactionStatusReorged   TransactionStatus = 4
)

// TransactionDirection 表示交易相对于当前绑定钱包的方向。
type TransactionDirection int16

const (
	TransactionDirectionIncoming TransactionDirection = 1
	TransactionDirectionOutgoing TransactionDirection = 2
	TransactionDirectionSelf     TransactionDirection = 3
)

// WalletTransaction 保存与一个绑定钱包相关的链上交易快照。
// 实际数据按 WalletID 路由到 wallet_transactions_00 至 wallet_transactions_07，
// 因此不声明固定 TableName。
type WalletTransaction struct {
	ID              string `gorm:"type:uuid;primaryKey" json:"id"`
	WalletID        string `gorm:"type:uuid;not null" json:"wallet_id"`
	ChainID         uint64 `gorm:"not null" json:"chain_id"`
	TransactionHash string `gorm:"type:char(66);not null" json:"transaction_hash"`
	// EventIndex 区分同一笔链上交易中的多条资产变动：原生币交易固定为 -1，
	// ERC-20 Transfer 使用链上日志的 log_index。
	EventIndex  int32                `gorm:"not null;default:-1;check:event_index >= -1" json:"event_index"`
	Direction   TransactionDirection `gorm:"not null;check:direction IN (1,2,3)" json:"direction"`
	FromAddress string               `gorm:"type:char(42);not null" json:"from_address"`
	ToAddress   string               `gorm:"type:char(42);not null" json:"to_address"`
	// ValueRaw 使用最小单位的十进制整数保存 uint256，禁止使用浮点数。
	ValueRaw      string            `gorm:"type:numeric(78,0);not null;default:0" json:"value_raw"`
	TokenContract *string           `gorm:"type:char(42)" json:"token_contract,omitempty"`
	BlockNumber   *int64            `json:"block_number,omitempty"`
	BlockHash     *string           `gorm:"type:char(66)" json:"block_hash,omitempty"`
	Confirmations uint64            `gorm:"not null;default:0" json:"confirmations"`
	Status        TransactionStatus `gorm:"not null;check:status IN (1,2,3,4)" json:"status"`
	BlockTime     *time.Time        `json:"block_time,omitempty"`
	FirstSeenAt   time.Time         `gorm:"not null" json:"first_seen_at"`
	UpdatedAt     time.Time         `gorm:"not null" json:"updated_at"`
}
