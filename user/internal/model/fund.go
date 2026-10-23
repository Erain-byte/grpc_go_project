package model

import "time"

// WalletStatus 表示用户钱包状态。
type WalletStatus int16

const (
	WalletStatusEnabled WalletStatus = 1 // 正常，可以进行资金操作。
	WalletStatusFrozen  WalletStatus = 2 // 冻结，拒绝资金变更。
)

// UserWallet 保存用户在某个币种下的当前余额。
// 金额统一使用最小货币单位的整数保存，避免浮点数精度问题。
// 实际数据按照 UserID 路由到 user_wallets_00 至 user_wallets_07。
type UserWallet struct {
	UserID          int64        `gorm:"primaryKey;autoIncrement:false" json:"user_id"`                          // 本地用户 ID，也是资金表分片键。
	Currency        string       `gorm:"primaryKey;type:char(3)" json:"currency"`                                // ISO 4217 三位币种代码，例如 CNY、USD。
	AvailableAmount int64        `gorm:"not null;default:0;check:available_amount >= 0" json:"available_amount"` // 可用余额，单位为最小货币单位。
	FrozenAmount    int64        `gorm:"not null;default:0;check:frozen_amount >= 0" json:"frozen_amount"`       // 冻结余额，单位为最小货币单位。
	Status          WalletStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"`           // 钱包状态。
	Version         int64        `gorm:"not null;default:1" json:"version"`                                      // 乐观锁版本号，每次余额变更后递增。
	CreatedAt       time.Time    `gorm:"not null" json:"created_at"`                                             // 钱包创建时间。
	UpdatedAt       time.Time    `gorm:"not null" json:"updated_at"`                                             // 最近一次余额或状态更新时间。
}

// FundTransactionType 表示资金流水的变更类型。
type FundTransactionType int16

const (
	FundTransactionCredit   FundTransactionType = 1 // 入账，增加可用余额。
	FundTransactionDebit    FundTransactionType = 2 // 扣款，减少可用余额。
	FundTransactionFreeze   FundTransactionType = 3 // 冻结，从可用余额转入冻结余额。
	FundTransactionUnfreeze FundTransactionType = 4 // 解冻，从冻结余额转回可用余额。
)

// UserFundTransaction 保存每次资金变化后的不可变流水快照。
// 实际数据按照 UserID 路由到 user_fund_transactions_00 至
// user_fund_transactions_07。
type UserFundTransaction struct {
	ID             int64               `gorm:"primaryKey;autoIncrement:false" json:"id"`                   // 雪花算法生成的资金流水 ID。
	UserID         int64               `gorm:"not null;index" json:"user_id"`                              // 本地用户 ID，也是流水表分片键。
	Currency       string              `gorm:"type:char(3);not null;index" json:"currency"`                // 本次变更涉及的币种。
	Type           FundTransactionType `gorm:"not null;index;check:type IN (1,2,3,4)" json:"type"`         // 资金变更类型。
	Amount         int64               `gorm:"not null;check:amount > 0" json:"amount"`                    // 本次变更金额，始终为正数。
	AvailableAfter int64               `gorm:"not null;check:available_after >= 0" json:"available_after"` // 变更完成后的可用余额快照。
	FrozenAfter    int64               `gorm:"not null;check:frozen_after >= 0" json:"frozen_after"`       // 变更完成后的冻结余额快照。
	BusinessType   string              `gorm:"size:32;not null;index" json:"business_type"`                // 来源业务类型，例如 order、refund。
	BusinessID     string              `gorm:"size:128;not null" json:"business_id"`                       // 来源业务单号，用于追踪和幂等控制。
	Remark         string              `gorm:"size:512;not null;default:''" json:"remark"`                 // 资金变更说明。
	CreatedAt      time.Time           `gorm:"not null;index" json:"created_at"`                           // 流水创建时间。
}
