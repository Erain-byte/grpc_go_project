package main

import (
	"flag"
	"fmt"
	"os"

	"user/internal/config"
	"user/internal/databases"
	"user/internal/model"
	"user/internal/sharding"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/user.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.InitConfig(configFile)
	if err != nil {
		exitWithError("load config", err)
	}
	if !cfg.Database.Enabled {
		exitWithError("validate config", fmt.Errorf("database is disabled"))
	}

	router, err := sharding.NewUserRouter(cfg.Database.Sharding)
	if err != nil {
		exitWithError("create user shard router", err)
	}
	dbClient, err := databases.NewGormClient(cfg.Database)
	if err != nil {
		exitWithError("connect database", err)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close database: %v\n", err)
		}
	}()

	db := dbClient.Gorm()
	if err := db.AutoMigrate(
		&model.UserIdentityIndex{},
		&model.UserGroup{},
		&model.UserGroupMember{},
	); err != nil {
		exitWithError("migrate user shared tables", err)
	}
	for index, userTableName := range router.TableNames() {
		if err := db.Table(userTableName).AutoMigrate(&model.User{}); err != nil {
			exitWithError("migrate "+userTableName, err)
		}
		walletTableName, err := router.WalletTableNameByIndex(index)
		if err != nil {
			exitWithError("route wallet table", err)
		}
		if err := db.Table(walletTableName).AutoMigrate(&model.UserWallet{}); err != nil {
			exitWithError("migrate "+walletTableName, err)
		}
		transactionTableName, err := router.FundTransactionTableNameByIndex(index)
		if err != nil {
			exitWithError("route fund transaction table", err)
		}
		if err := db.Table(transactionTableName).AutoMigrate(&model.UserFundTransaction{}); err != nil {
			exitWithError("migrate "+transactionTableName, err)
		}

		indexName := "uk_" + transactionTableName + "_business"
		createIndexSQL := fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS "%s" ON "%s" (business_type, business_id)`,
			indexName,
			transactionTableName,
		)
		if err := db.Exec(createIndexSQL).Error; err != nil {
			exitWithError("create "+indexName, err)
		}
	}

	fmt.Printf("user database migration completed: %d shard groups\n", len(router.TableNames()))
}

func exitWithError(action string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
