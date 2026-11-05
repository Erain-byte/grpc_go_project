package model

import "time"

// TronUsageDaily stores aggregate counters only. It deliberately contains no
// plaintext wallet address or API credential.
type TronUsageDaily struct {
	UsageDate            time.Time `gorm:"type:date;primaryKey" json:"usage_date"`
	QueryCount           int64     `gorm:"not null;default:0;check:query_count >= 0" json:"query_count"`
	DistinctAddressCount int64     `gorm:"not null;default:0;check:distinct_address_count >= 0" json:"distinct_address_count"`
	UpstreamCallCount    int64     `gorm:"not null;default:0;check:upstream_call_count >= 0" json:"upstream_call_count"`
	CacheHitCount        int64     `gorm:"not null;default:0;check:cache_hit_count >= 0" json:"cache_hit_count"`
	SuccessCount         int64     `gorm:"not null;default:0;check:success_count >= 0" json:"success_count"`
	PartialCount         int64     `gorm:"not null;default:0;check:partial_count >= 0" json:"partial_count"`
	FailedCount          int64     `gorm:"not null;default:0;check:failed_count >= 0" json:"failed_count"`
	CreatedAt            time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt            time.Time `gorm:"not null" json:"updated_at"`
}

func (TronUsageDaily) TableName() string { return "tron_usage_daily" }

// TronUsageAddressDaily is used only to count unique addresses per UTC day.
// AddressHMAC is HMAC-SHA256(address); the raw public address is never stored.
type TronUsageAddressDaily struct {
	UsageDate   time.Time `gorm:"type:date;primaryKey" json:"usage_date"`
	AddressHMAC string    `gorm:"type:char(64);primaryKey" json:"address_hmac"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

func (TronUsageAddressDaily) TableName() string { return "tron_usage_address_daily" }
