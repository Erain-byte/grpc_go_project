package main

import (
	"flag"
	"fmt"
	"os"

	"web3/internal/config"
	"web3/internal/databases"
	"web3/internal/model"
	"web3/internal/sharding"

	"gorm.io/gorm"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/web3.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.InitConfig(configFile)
	if err != nil {
		exitWithError("load config", err)
	}
	if !cfg.Database.Enabled {
		exitWithError("validate config", fmt.Errorf("database is disabled"))
	}

	database, err := databases.NewGormClient(cfg.Database)
	if err != nil {
		exitWithError("connect database", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close database: %v\n", err)
		}
	}()

	db := database.Gorm()
	if err := migrateSharedTables(db); err != nil {
		exitWithError("migrate shared tables", err)
	}

	router := sharding.NewTransactionRouter()
	for _, table := range router.TableNames() {
		if err := migrateTransactionTable(db, table); err != nil {
			exitWithError("migrate "+table, err)
		}
	}

	fmt.Printf("web3 database migration completed: %d transaction shards\n", len(router.TableNames()))
}

func migrateSharedTables(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Wallet{},
		&model.ChainScanCursor{},
		&model.ChainEventReceipt{},
		&model.TronUsageDaily{},
		&model.TronUsageAddressDaily{},
		&model.ClientEvent{},
		&model.FeedbackItem{},
	); err != nil {
		return err
	}

	// PostgreSQL部分唯一索引保证一个用户最多只有一个处于启用状态的默认钱包。
	return db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uk_wallets_user_default
		ON wallets (user_subject)
		WHERE is_default = TRUE AND status = 1
	`).Error
}

func migrateTransactionTable(db *gorm.DB, table string) error {
	if err := db.Table(table).AutoMigrate(&model.WalletTransaction{}); err != nil {
		return err
	}

	statements := []string{
		// 旧版本用交易哈希作为唯一键，会让同一交易内的多个 ERC-20 Transfer 互相覆盖。
		// event_index=-1 表示原生币交易；ERC-20 使用链上 log_index。
		fmt.Sprintf(
			`DROP INDEX IF EXISTS %s`,
			quoteIdentifier("uk_"+table+"_wallet_chain_hash"),
		),
		fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (wallet_id, chain_id, transaction_hash, event_index)`,
			quoteIdentifier("uk_"+table+"_wallet_chain_hash_event"),
			quoteIdentifier(table),
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS %s ON %s (wallet_id, status, block_time DESC, id DESC)`,
			quoteIdentifier("idx_"+table+"_wallet_list"),
			quoteIdentifier(table),
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS %s ON %s (chain_id, block_number)`,
			quoteIdentifier("idx_"+table+"_chain_block"),
			quoteIdentifier(table),
		),
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// table和index名称全部来自固定Router，仍统一加双引号避免标识符解析差异。
func quoteIdentifier(identifier string) string {
	return `"` + identifier + `"`
}

func exitWithError(action string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
