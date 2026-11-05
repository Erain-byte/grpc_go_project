package repository

import (
	"context"
	"time"

	"web3/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WalletRepository 负责钱包绑定关系及默认钱包状态的事务性维护。
type WalletRepository struct{ db *gorm.DB }

// NewWalletRepository 创建钱包仓库。
func NewWalletRepository(db *gorm.DB) *WalletRepository { return &WalletRepository{db: db} }

func (r *WalletRepository) Create(ctx context.Context, wallet *model.Wallet) error {
	return r.db.WithContext(ctx).Create(wallet).Error
}

// CreateBoundWallet 按用户串行化“首个默认钱包”判断，避免并发绑定产生两个默认钱包。
// CreateBoundWallet 在同一用户范围内串行化首次默认钱包的判定。
func (r *WalletRepository) CreateBoundWallet(ctx context.Context, wallet *model.Wallet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWalletOwner(tx, wallet.UserSubject); err != nil {
			return err
		}
		var defaultCount int64
		if err := tx.Model(&model.Wallet{}).
			Where("user_subject = ? AND status = ? AND is_default = ?", wallet.UserSubject, model.WalletStatusActive, true).
			Count(&defaultCount).Error; err != nil {
			return err
		}
		wallet.IsDefault = defaultCount == 0
		return tx.Create(wallet).Error
	})
}

// Reactivate 恢复此前解绑的钱包，并在没有默认钱包时把它设为默认。
func (r *WalletRepository) Reactivate(ctx context.Context, wallet *model.Wallet, label string, verifiedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWalletOwner(tx, wallet.UserSubject); err != nil {
			return err
		}
		var defaultCount int64
		if err := tx.Model(&model.Wallet{}).
			Where("user_subject = ? AND status = ? AND is_default = ?", wallet.UserSubject, model.WalletStatusActive, true).
			Count(&defaultCount).Error; err != nil {
			return err
		}
		return tx.Model(&model.Wallet{}).Where("id = ?", wallet.ID).Updates(map[string]any{
			"status":      model.WalletStatusActive,
			"label":       label,
			"verified_at": verifiedAt,
			"is_default":  defaultCount == 0,
		}).Error
	})
}

// FindByID 同时按用户 subject 过滤，防止通过钱包 ID 越权读取其他用户的钱包。
func (r *WalletRepository) FindByID(ctx context.Context, userSubject, walletID string) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_subject = ?", walletID, userSubject).
		First(&wallet).Error
	return &wallet, err
}

// FindByChainAddress 用于检查同一条链上的地址是否已被任何用户绑定。
func (r *WalletRepository) FindByChainAddress(ctx context.Context, chainID uint64, address string) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND address = ?", chainID, address).
		First(&wallet).Error
	return &wallet, err
}

// ListActiveByUser 返回一个用户的启用钱包，默认钱包排在最前面。
func (r *WalletRepository) ListActiveByUser(ctx context.Context, userSubject string) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := r.db.WithContext(ctx).
		Where("user_subject = ? AND status = ?", userSubject, model.WalletStatusActive).
		Order("is_default DESC, created_at ASC").
		Find(&wallets).Error
	return wallets, err
}

// ListActiveByChain 供后台扫描器构建当前链上已绑定的钱包地址集合。
func (r *WalletRepository) ListActiveByChain(ctx context.Context, chainID uint64) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND status = ?", chainID, model.WalletStatusActive).
		Find(&wallets).Error
	return wallets, err
}

// UpdateLabel 只更新当前用户仍启用的钱包。
func (r *WalletRepository) UpdateLabel(ctx context.Context, userSubject, walletID, label string) error {
	result := r.db.WithContext(ctx).Model(&model.Wallet{}).
		Where("id = ? AND user_subject = ? AND status = ?", walletID, userSubject, model.WalletStatusActive).
		Update("label", label)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetDefault 在一个事务内清除旧默认值并设置新默认钱包。
func (r *WalletRepository) SetDefault(ctx context.Context, userSubject, walletID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWalletOwner(tx, userSubject); err != nil {
			return err
		}
		var wallet model.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_subject = ? AND status = ?", walletID, userSubject, model.WalletStatusActive).
			First(&wallet).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Wallet{}).
			Where("user_subject = ? AND is_default = ?", userSubject, true).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.Wallet{}).Where("id = ?", walletID).Update("is_default", true).Error
	})
}

// UnsetDefault 取消指定钱包的默认标识，不会自动选择替代钱包。
func (r *WalletRepository) UnsetDefault(ctx context.Context, userSubject, walletID string) error {
	result := r.db.WithContext(ctx).Model(&model.Wallet{}).
		Where("id = ? AND user_subject = ? AND status = ?", walletID, userSubject, model.WalletStatusActive).
		Update("is_default", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Disable 逻辑解绑钱包；若解绑的是默认钱包，会在剩余启用钱包中补选一个默认钱包。
func (r *WalletRepository) Disable(ctx context.Context, userSubject, walletID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWalletOwner(tx, userSubject); err != nil {
			return err
		}
		var current model.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_subject = ? AND status = ?", walletID, userSubject, model.WalletStatusActive).
			First(&current).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Wallet{}).Where("id = ?", walletID).
			Updates(map[string]any{"status": model.WalletStatusDisabled, "is_default": false}).Error; err != nil {
			return err
		}
		if !current.IsDefault {
			return nil
		}
		var replacement model.Wallet
		err := tx.Where("user_subject = ? AND status = ?", userSubject, model.WalletStatusActive).
			Order("created_at ASC").First(&replacement).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		return tx.Model(&model.Wallet{}).Where("id = ?", replacement.ID).Update("is_default", true).Error
	})
}

// lockWalletOwner 使用 PostgreSQL 事务建议锁串行化同一用户的钱包默认状态变更。
func lockWalletOwner(tx *gorm.DB, userSubject string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", userSubject).Error
}
