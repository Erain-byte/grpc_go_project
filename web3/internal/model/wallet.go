package model

import "time"

// WalletStatus 表示链上钱包在本系统中的绑定状态。
type WalletStatus int16

const (
	WalletStatusActive   WalletStatus = 1 // 已完成签名验证，可以使用。
	WalletStatusDisabled WalletStatus = 2 // 已解绑或被禁用，保留记录用于审计。
)

// Wallet 保存 Keycloak 用户与EVM地址之间经过签名验证的绑定关系。
// 钱包数量远小于交易数量，因此该表不做分表。
type Wallet struct {
	ID          string       `gorm:"type:uuid;primaryKey" json:"id"`
	UserSubject string       `gorm:"type:uuid;not null;index:idx_wallet_user_status,priority:1" json:"user_subject"`
	ChainID     uint64       `gorm:"not null;uniqueIndex:uk_wallet_chain_address,priority:1" json:"chain_id"`
	Address     string       `gorm:"type:char(42);not null;uniqueIndex:uk_wallet_chain_address,priority:2" json:"address"`
	Label       string       `gorm:"size:128;not null;default:''" json:"label"`
	Status      WalletStatus `gorm:"not null;default:1;index:idx_wallet_user_status,priority:2;check:status IN (1,2)" json:"status"`
	IsDefault   bool         `gorm:"not null;default:false;index" json:"is_default"`
	VerifiedAt  time.Time    `gorm:"not null" json:"verified_at"`
	CreatedAt   time.Time    `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time    `gorm:"not null" json:"updated_at"`
}

func (Wallet) TableName() string { return "wallets" }
