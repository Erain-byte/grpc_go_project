// Package databases 封装 Web3 服务的 PostgreSQL 连接和连接池生命周期。
package databases

import (
	"context"
	"database/sql"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"web3/internal/config"
	"web3/pkg/apperror"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// GormClient 持有 GORM 实例及其底层主库连接池。
// 一个实例应由 ServiceContext 共享，并在服务退出时统一关闭。
type GormClient struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

// NewGormClient 创建主库连接池，并在启用读写分离时注册只读副本。
func NewGormClient(cfg config.DatabaseConfig) (*GormClient, error) {
	db, err := gorm.Open(postgres.Open(buildDSN(cfg, cfg.Address())), &gorm.Config{})
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnavailable, "failed to create database client", http.StatusServiceUnavailable)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnavailable, "failed to get database connection pool", http.StatusServiceUnavailable)
	}

	maxIdleTime, err := time.ParseDuration(cfg.ConnMaxIdleTime)
	if err != nil {
		_ = sqlDB.Close()
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "invalid database connection idle time", http.StatusBadRequest)
	}
	maxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		_ = sqlDB.Close()
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "invalid database connection lifetime", http.StatusBadRequest)
	}

	setPoolLimits(sqlDB, cfg, maxIdleTime, maxLifetime)

	if cfg.ReadWriteSplitEnabled {
		resolver := dbresolver.Register(dbresolver.Config{
			Replicas:          replicaDialectors(cfg),
			Policy:            dbresolver.RandomPolicy{},
			TraceResolverMode: true,
		}).
			SetMaxIdleConns(cfg.MaxIdleConns).
			SetMaxOpenConns(cfg.MaxOpenConns).
			SetConnMaxIdleTime(maxIdleTime).
			SetConnMaxLifetime(maxLifetime)

		if err := db.Use(resolver); err != nil {
			_ = sqlDB.Close()
			return nil, apperror.Wrap(err, apperror.CodeUnavailable, "failed to configure database replicas", http.StatusServiceUnavailable)
		}
	}

	return &GormClient{db: db, sqlDB: sqlDB}, nil
}

func setPoolLimits(db *sql.DB, cfg config.DatabaseConfig, maxIdleTime, maxLifetime time.Duration) {
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetConnMaxIdleTime(maxIdleTime)
	db.SetConnMaxLifetime(maxLifetime)
}

func buildDSN(cfg config.DatabaseConfig, address string) string {
	connectTimeout, _ := time.ParseDuration(cfg.ConnectTimeout)
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   address,
		Path:   cfg.DBName,
	}

	query := dsn.Query()
	query.Set("sslmode", cfg.SSLMode)
	query.Set("TimeZone", cfg.TimeZone)
	query.Set("connect_timeout", strconv.Itoa(int(math.Ceil(connectTimeout.Seconds()))))
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func replicaDialectors(cfg config.DatabaseConfig) []gorm.Dialector {
	replicas := make([]gorm.Dialector, 0, len(cfg.ReplicaAddresses))
	for _, address := range cfg.ReplicaAddresses {
		if address = strings.TrimSpace(address); address != "" {
			replicas = append(replicas, postgres.Open(buildDSN(cfg, address)))
		}
	}
	return replicas
}

// Gorm 返回供 Repository 使用的共享数据库句柄。
func (c *GormClient) Gorm() *gorm.DB {
	if c == nil {
		return nil
	}
	return c.db
}

// Read 显式选择只读副本；未配置读写分离时仍使用主库。
func (c *GormClient) Read(ctx context.Context) *gorm.DB {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.WithContext(ctx).Clauses(dbresolver.Read)
}

// Write 显式选择主库，适用于事务以及写入后立即读取的场景。
func (c *GormClient) Write(ctx context.Context) *gorm.DB {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.WithContext(ctx).Clauses(dbresolver.Write)
}

func (c *GormClient) Ping(ctx context.Context) error {
	if c == nil || c.sqlDB == nil {
		return apperror.Unavailable("database client is not initialized")
	}
	if err := c.sqlDB.PingContext(ctx); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "database is unavailable", http.StatusServiceUnavailable)
	}
	return nil
}

func (c *GormClient) Close() error {
	if c == nil || c.sqlDB == nil {
		return nil
	}
	return apperror.Wrap(c.sqlDB.Close(), apperror.CodeInternal, "failed to close database client", http.StatusInternalServerError)
}
