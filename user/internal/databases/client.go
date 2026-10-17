package databases

import (
	"context"
	"database/sql"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"user/internal/config"
	"user/pkg/apperror"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// GormClient owns the GORM handle and its underlying connection pool.
type GormClient struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func NewGormClient(cfg config.DatabaseConfig) (*GormClient, error) {
	primaryAddress := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	db, err := gorm.Open(postgres.Open(getDSN(cfg, primaryAddress)), &gorm.Config{})
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database connect error", http.StatusServiceUnavailable)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnavailable, "failed to get database connection pool", http.StatusServiceUnavailable)
	}

	if cfg.ReadWriteSplitEnabled {
		replicas := getReplicaDialectors(cfg)
		if len(replicas) > 0 {
			if err := db.Use(dbresolver.Register(dbresolver.Config{
				Replicas:          replicas,
				Policy:            dbresolver.RandomPolicy{},
				TraceResolverMode: true,
			})); err != nil {
				_ = sqlDB.Close()
				return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database resolver error", http.StatusServiceUnavailable)
			}
		}
	}

	connMaxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		_ = sqlDB.Close()
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "invalid database connection lifetime", http.StatusServiceUnavailable)
	}
	connMaxIdleTime, err := time.ParseDuration(cfg.ConnMaxIdleTime)
	if err != nil {
		_ = sqlDB.Close()
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "invalid database connection idle time", http.StatusServiceUnavailable)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	return &GormClient{db: db, sqlDB: sqlDB}, nil
}

func getDSN(cfg config.DatabaseConfig, address string) string {
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

func getReplicaDialectors(cfg config.DatabaseConfig) []gorm.Dialector {
	replicas := make([]gorm.Dialector, 0, len(cfg.ReplicaAddresses))
	for _, address := range cfg.ReplicaAddresses {
		address = strings.TrimSpace(address)
		if address != "" {
			replicas = append(replicas, postgres.Open(getDSN(cfg, address)))
		}
	}
	return replicas
}

func (g *GormClient) Gorm() *gorm.DB {
	if g == nil {
		return nil
	}
	return g.db
}

func (g *GormClient) Ping(ctx context.Context) error {
	return g.sqlDB.PingContext(ctx)
}

func (g *GormClient) Close() error {
	if g == nil || g.sqlDB == nil {
		return nil
	}
	return g.sqlDB.Close()
}
