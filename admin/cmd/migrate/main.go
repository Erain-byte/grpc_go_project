// Command migrate creates or updates the Admin database schema from the
// current GORM models. It is intended for local development and integration
// environments; production schema changes should use reviewed versioned SQL.
package main

import (
	"flag"
	"fmt"
	"os"

	"admin/internal/config"
	"admin/internal/database"
	"admin/internal/model"

	"gorm.io/gorm"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/admin.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.InitConfig(configFile)
	if err != nil {
		exitWithError("load config", err)
	}

	dbClient, err := database.NewGormClient(cfg.Database)
	if err != nil {
		exitWithError("connect database", err)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close database: %v\n", err)
		}
	}()

	// 先迁移主表，再迁移依赖主表的会话、日志和多对多关联表。
	if err := dbClient.Gorm().AutoMigrate(
		&model.AdminModel{},
		&model.RoleModel{},
		&model.PermissionsModel{},
		&model.AdminSessionModel{},
		&model.OperationLogModel{},
	); err != nil {
		exitWithError("migrate database", err)
	}

	// 兼容旧版 admins 表。旧模型使用 password，新模型统一使用
	// password_hash；AutoMigrate 为保护数据不会自动删除旧列。
	var legacyPasswordColumnCount int64
	if err := dbClient.Gorm().Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'admins'
		  AND column_name = 'password'
	`).Scan(&legacyPasswordColumnCount).Error; err != nil {
		exitWithError("inspect legacy admins.password column", err)
	}
	if legacyPasswordColumnCount > 0 {
		err := dbClient.Gorm().Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`
				UPDATE admins
				SET password_hash = password
				WHERE (password_hash IS NULL OR password_hash = '')
				  AND password IS NOT NULL
				  AND password <> ''
			`).Error; err != nil {
				return err
			}
			return tx.Exec("ALTER TABLE admins DROP COLUMN password").Error
		})
		if err != nil {
			exitWithError("migrate legacy admins.password column", err)
		}
	}

	fmt.Println("admin database migration completed")
}

func exitWithError(action string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
