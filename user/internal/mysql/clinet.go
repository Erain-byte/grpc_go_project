package mysql

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"user/internal/config"
	"user/pkg/apperror"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type GormClient struct {
	Db *gorm.DB
	db *sql.DB
}

// 构造函数
func NewGormClient(cfg config.DatabaseConfig) (*GormClient, error) {
	//实列地址
	primaryAddress := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	//主库
	db, err := gorm.Open(mysql.Open(getDSN(cfg, primaryAddress)), &gorm.Config{})
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInvalidArgument, "database connect error", http.StatusServiceUnavailable)
	}
	sqlDB, err := db.DB()
	//主从配置
	replicas := getreplicaDSN(cfg)
	if len(replicas) > 0 && cfg.ReadWriteSplitEnabled {
		err = db.Use(dbresolver.Register(dbresolver.Config{

			// 从库的地址
			Replicas: replicas,
			// 主库的负载均衡策略
			Policy: dbresolver.RandomPolicy{},
			//负载均衡策略的参数
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
	return &GormClient{Db: db, db: sqlDB}, nil
}

// 辅助函数
func getDSN(cfg config.DatabaseConfig, address string) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		cfg.Username,
		cfg.Password,
		address,
		cfg.DBName,
		cfg.Charset,
		cfg.ParseTime,
		url.QueryEscape(cfg.Location),
	)
}

func getreplicaDSN(cfg config.DatabaseConfig) []gorm.Dialector {
	if len(cfg.ReplicaAddresses) == 0 {
		return []gorm.Dialector{}
	}
	//replicas
	replicas := make([]gorm.Dialector, len(cfg.ReplicaAddresses))
	for _, address := range cfg.ReplicaAddresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		replicas = append(replicas, mysql.Open(getDSN(cfg, address)))
	}
	return replicas
}
