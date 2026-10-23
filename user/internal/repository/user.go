package repository

import (
	"context"
	"errors"

	"user/internal/model"
	"user/internal/sharding"

	"gorm.io/gorm"
)

type UserRepository struct {
	db     *gorm.DB
	router *sharding.UserRouter
}

func NewUserRepository(db *gorm.DB, router *sharding.UserRouter) *UserRepository {
	return &UserRepository{db: db, router: router}
}

func (r *UserRepository) FindBySubject(ctx context.Context, subject string) (*model.User, error) {
	var identity model.UserIdentityIndex
	if err := r.db.WithContext(ctx).Where("keycloak_subject = ?", subject).First(&identity).Error; err != nil {
		return nil, err
	}
	tableName, err := r.router.TableNameByIndex(int(identity.TableIndex))
	if err != nil {
		return nil, err
	}
	var user model.User
	if err := r.db.WithContext(ctx).Table(tableName).Where("id = ?", identity.UserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindOrCreateBySubject(ctx context.Context, user *model.User) (*model.User, bool, error) {
	existing, err := r.FindBySubject(ctx, user.KeycloakSubject)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	txErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		index, routeErr := r.router.TableIndex(user.ID)
		if routeErr != nil {
			return routeErr
		}
		table, routeErr := r.router.TableNameByIndex(index)
		if routeErr != nil {
			return routeErr
		}
		if createErr := tx.Table(table).Create(user).Error; createErr != nil {
			return createErr
		}
		return tx.Create(&model.UserIdentityIndex{KeycloakSubject: user.KeycloakSubject, UserID: user.ID, TableIndex: int16(index)}).Error
	})
	if txErr == nil {
		return user, true, nil
	}

	// Concurrent initialization can lose the unique-key race. Return the winner.
	existing, lookupErr := r.FindBySubject(ctx, user.KeycloakSubject)
	if lookupErr == nil {
		return existing, false, nil
	}
	return nil, false, txErr
}
