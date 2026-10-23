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

	// 先迁移主表，再迁移日志和多对多关联表。
	if err := dbClient.Gorm().AutoMigrate(
		&model.AdminModel{},
		&model.RoleModel{},
		&model.PermissionsModel{},
		&model.OperationLogModel{},
	); err != nil {
		exitWithError("migrate database", err)
	}

	// 兼容旧版 admins 表。旧模型使用 password，新模型统一使用
	// password_hash；AutoMigrate 为保护数据不会自动删除旧列。
	if dbClient.Gorm().Migrator().HasColumn("admins", "password") {
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
			return tx.Migrator().DropColumn("admins", "password")
		})
		if err != nil {
			exitWithError("migrate legacy admins.password column", err)
		}
	}

	// Keycloak 与 oauth2-proxy 已接管 Token 和浏览器 Session。旧表只保存
	// Admin 自签 Refresh Token 的哈希，当前代码不再读取，迁移时将其删除。
	if dbClient.Gorm().Migrator().HasTable("admin_sessions") {
		if err := dbClient.Gorm().Migrator().DropTable("admin_sessions"); err != nil {
			exitWithError("drop legacy admin_sessions table", err)
		}
	}

	fmt.Println("admin database migration completed")
}

func exitWithError(action string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
