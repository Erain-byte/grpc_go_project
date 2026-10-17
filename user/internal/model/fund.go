package model

import "time"

type WalletStatus int16

const (
	WalletStatusEnabled WalletStatus = 1
	WalletStatusFrozen  WalletStatus = 2
)

type UserWallet struct {
	UserID          int64        `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	Currency        string       `gorm:"primaryKey;type:char(3)" json:"currency"`
	AvailableAmount int64        `gorm:"not null;default:0;check:available_amount >= 0" json:"available_amount"`
	FrozenAmount    int64        `gorm:"not null;default:0;check:frozen_amount >= 0" json:"frozen_amount"`
	Status          WalletStatus `gorm:"not null;default:1;index;check:status IN (1,2)" json:"status"`
	Version         int64        `gorm:"not null;default:1" json:"version"`
	CreatedAt       time.Time    `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time    `gorm:"not null" json:"updated_at"`
}

type FundTransactionType int16

const (
	FundTransactionCredit   FundTransactionType = 1
	FundTransactionDebit    FundTransactionType = 2
	FundTransactionFreeze   FundTransactionType = 3
	FundTransactionUnfreeze FundTransactionType = 4
)

type UserFundTransaction struct {
	ID             int64               `gorm:"primaryKey;autoIncrement:false" json:"id"`
	UserID         int64               `gorm:"not null;index" json:"user_id"`
	Currency       string              `gorm:"type:char(3);not null;index" json:"currency"`
	Type           FundTransactionType `gorm:"not null;index;check:type IN (1,2,3,4)" json:"type"`
	Amount         int64               `gorm:"not null;check:amount > 0" json:"amount"`
	AvailableAfter int64               `gorm:"not null;check:available_after >= 0" json:"available_after"`
	FrozenAfter    int64               `gorm:"not null;check:frozen_after >= 0" json:"frozen_after"`
	BusinessType   string              `gorm:"size:32;not null;index" json:"business_type"`
	BusinessID     string              `gorm:"size:128;not null" json:"business_id"`
	Remark         string              `gorm:"size:512;not null;default:''" json:"remark"`
	CreatedAt      time.Time           `gorm:"not null;index" json:"created_at"`
}
