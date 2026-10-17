package database

import (
	"admin/internal/config"
	apperror "admin/pkg/apperorr"
	"context"
	"database/sql"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// GormClient is a gorm client
type GormClient struct {
	db *gorm.DB
	DB *sql.DB
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
	//主从配置
	replicas := getreplicaDSN(cfg)
	if cfg.ReadWriteSplitEnabled && len(replicas) > 0 {
		err = db.Use(dbresolver.Register(dbresolver.Config{
			Replicas:          replicas,
			Policy:            dbresolver.RandomPolicy{},
			TraceResolverMode: true,
		}))
		if err != nil {
			return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database connect error", http.StatusServiceUnavailable)
		}
	}

	//链接池设置
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	connMaxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database connect error", http.StatusServiceUnavailable)
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	//设置最大空闲时间
	connIdleTimeout, err := time.ParseDuration(cfg.ConnMaxIdleTime)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database connect error", http.StatusServiceUnavailable)
	}
	sqlDB.SetConnMaxIdleTime(connIdleTimeout)
	return &GormClient{db: db, DB: sqlDB}, nil
}

func getreplicaDSN(cfg config.DatabaseConfig) []gorm.Dialector {
	if len(cfg.ReplicaAddresses) == 0 {
		return []gorm.Dialector{}
	}
	replicas := make([]gorm.Dialector, 0, len(cfg.ReplicaAddresses))

	for _, v := range cfg.ReplicaAddresses {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		replicas = append(replicas, postgres.Open(getDSN(cfg, v)))
	}
	return replicas
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

func (g *GormClient) Gorm() *gorm.DB {
	return g.db
}
func (g *GormClient) Close() error {
	if g == nil || g.DB == nil {
		return nil
	}
	return g.DB.Close()
}

func (g *GormClient) Ping(ctx context.Context) error {
	return g.DB.PingContext(ctx)
}
