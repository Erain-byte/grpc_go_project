package model

import "time"

// ChainScanCursor 记录每条链已经安全处理到的区块位置。
// 更新游标和写入事件回执应放在同一个数据库事务中。
type ChainScanCursor struct {
	ChainID       uint64    `gorm:"primaryKey;autoIncrement:false" json:"chain_id"`
	BlockNumber   int64     `gorm:"not null;check:block_number >= 0" json:"block_number"`
	BlockHash     string    `gorm:"type:char(66);not null" json:"block_hash"`
	LastScannedAt time.Time `gorm:"not null" json:"last_scanned_at"`
	UpdatedAt     time.Time `gorm:"not null" json:"updated_at"`
}

func (ChainScanCursor) TableName() string { return "chain_scan_cursors" }

// ChainEventStatus 表示链上事件是否已完成业务处理。
type ChainEventStatus int16

const (
	ChainEventStatusPending   ChainEventStatus = 1
	ChainEventStatusProcessed ChainEventStatus = 2
	ChainEventStatusFailed    ChainEventStatus = 3
	ChainEventStatusReorged   ChainEventStatus = 4
)

// ChainEventReceipt 是链上事件的幂等回执。
// (chain_id, transaction_hash, log_index) 唯一，防止扫描重试造成重复入账或重复发消息。
type ChainEventReceipt struct {
	ID              string           `gorm:"type:uuid;primaryKey" json:"id"`
	ChainID         uint64           `gorm:"not null;uniqueIndex:uk_chain_event,priority:1;index" json:"chain_id"`
	TransactionHash string           `gorm:"type:char(66);not null;uniqueIndex:uk_chain_event,priority:2" json:"transaction_hash"`
	LogIndex        uint32           `gorm:"not null;uniqueIndex:uk_chain_event,priority:3" json:"log_index"`
	BlockNumber     int64            `gorm:"not null;index;check:block_number >= 0" json:"block_number"`
	BlockHash       string           `gorm:"type:char(66);not null" json:"block_hash"`
	ContractAddress string           `gorm:"type:char(42);not null;index" json:"contract_address"`
	Topic0          string           `gorm:"type:char(66);not null;index" json:"topic0"`
	Payload         []byte           `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	Status          ChainEventStatus `gorm:"not null;default:1;index;check:status IN (1,2,3,4)" json:"status"`
	RetryCount      int              `gorm:"not null;default:0;check:retry_count >= 0" json:"retry_count"`
	LastError       string           `gorm:"type:text;not null;default:''" json:"last_error"`
	ObservedAt      time.Time        `gorm:"not null" json:"observed_at"`
	ProcessedAt     *time.Time       `json:"processed_at,omitempty"`
	CreatedAt       time.Time        `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time        `gorm:"not null" json:"updated_at"`
}

func (ChainEventReceipt) TableName() string { return "chain_event_receipts" }
