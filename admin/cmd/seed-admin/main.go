// Command seed-admin creates an initial local administrator and assigns the
// super_admin role. Plaintext passwords are read from the environment and are
// never persisted.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"admin/internal/config"
	"admin/internal/database"
	"admin/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	usernameEnvironment = "ADMIN_INITIAL_USERNAME"
	passwordEnvironment = "ADMIN_INITIAL_PASSWORD"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/admin.yaml", "path to config file")
	flag.Parse()

	username := strings.TrimSpace(os.Getenv(usernameEnvironment))
	password := os.Getenv(passwordEnvironment)
	if username == "" {
		exitWithError(usernameEnvironment + " is required")
	}
	if len(password) < 6 {
		exitWithError(passwordEnvironment + " must contain at least 6 characters")
	}

	cfg, err := config.InitConfig(configFile)
	if err != nil {
		exitWithWrappedError("load config", err)
	}
	dbClient, err := database.NewGormClient(cfg.Database)
	if err != nil {
		exitWithWrappedError("connect database", err)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close database: %v\n", err)
		}
	}()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		exitWithWrappedError("hash initial password", err)
	}

	err = dbClient.Gorm().Transaction(func(tx *gorm.DB) error {
		role := model.RoleModel{
			Name:   "Super Administrator",
			Code:   "super_admin",
			Desc:   "Built-in administrator with full access",
			Status: 1,
		}
		if err := tx.Where("code = ?", role.Code).FirstOrCreate(&role).Error; err != nil {
			return err
		}

		admin := model.AdminModel{
			Username:     username,
			PasswordHash: string(passwordHash),
			Nickname:     "Admin Test",
			Status:       model.AdminStatusEnabled,
		}
		result := tx.Where("username = ?", username).FirstOrCreate(&admin)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("administrator %q already exists", username)
		}

		return tx.Model(&admin).Association("Roles").Replace(&role)
	})
	if err != nil {
		exitWithWrappedError("seed administrator", err)
	}

	fmt.Printf("administrator %q created with role super_admin\n", username)
}

func exitWithWrappedError(action string, err error) {
	exitWithError(fmt.Sprintf("%s: %v", action, err))
}

func exitWithError(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
