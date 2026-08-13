package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"gateway/internal/consul"
	"gateway/internal/logger"
	"gateway/internal/middleware"
	"gateway/pkg/apperror"
	"os"
	"strings"
	"sync"
	"time"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	certificateWarnAfter = 1 * 24 * time.Hour
)

const roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}` // 配置负载均衡策略为轮询

// GrpcConfig configures gRPC client transport security.
type GrpcConfig struct {
	UseTLS             bool
	InsecureSkipVerify bool
	CertFile           string
	KeyFile            string
	CaFile             string
	ServerName         string
}

// 管理gRPC客户端连接状态通知管理器
type pendingConnection struct {
	done chan struct{}
	conn *grpc.ClientConn
	err  error
}

// 创建gRPC客户端管理器
type ClientManager struct {
	registry consul.GRPCServiceDiscoverer // 服务发现器
	config   *GrpcConfig                  // gRPC配置

	mu      sync.RWMutex
	conns   map[string]*grpc.ClientConn //缓存已建立的连接
	pending map[string]*pendingConnection
	closed  bool
}

func NewClientManager(registry consul.GRPCServiceDiscoverer, config *GrpcConfig) *ClientManager {
	cfg := GrpcConfig{}
	if config != nil {
		cfg = *config // Do not mutate configuration owned by the caller.
	}
	return &ClientManager{
		registry: registry,
		config:   &cfg,
		conns:    make(map[string]*grpc.ClientConn),
		pending:  make(map[string]*pendingConnection),
	}
}

// GetClient returns the shared ClientConn for serviceName. Context controls
// waiting for another goroutine that is creating the same connection.
func (cm *ClientManager) GetClient(ctx context.Context, serviceName string) (*grpc.ClientConn, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("service name is empty"))
	}
	if cm.registry == nil {
		return nil, apperror.ToGRPC(apperror.Unavailable("service registry is unavailable"))
	}

	cm.mu.RLock()
	conn := cm.conns[serviceName]
	closed := cm.closed
	cm.mu.RUnlock()
	if closed {
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if conn != nil {
		return conn, nil
	}

	cm.mu.Lock() // 加锁
	if cm.closed {
		cm.mu.Unlock()
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if conn = cm.conns[serviceName]; conn != nil {
		cm.mu.Unlock()
		return conn, nil
	}
	if pending := cm.pending[serviceName]; pending != nil { // 如果有其他goroutine正在创建连接，则等待
		cm.mu.Unlock()
		select {
		case <-pending.done: // 等待连接创建完成
			return pending.conn, pending.err
		case <-ctx.Done():
			return nil, apperror.ToGRPC(ctx.Err())
		}
	}
	pending := &pendingConnection{done: make(chan struct{})} // 创建一个等待连接创建完成的channel
	cm.pending[serviceName] = pending
	cm.mu.Unlock()

	conn, err := cm.newClient(serviceName) // 创建新的连接
	if err != nil {
		err = apperror.ToGRPC(err)
	}

	cm.mu.Lock()
	delete(cm.pending, serviceName)
	if cm.closed && conn != nil {
		_ = conn.Close()
		conn = nil
		err = apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	} else if err == nil {
		cm.conns[serviceName] = conn
	}
	pending.conn = conn
	pending.err = err
	close(pending.done) //关闭后，其他等待的goroutine会收到通知
	cm.mu.Unlock()

	return conn, err
}

func (cm *ClientManager) newClient(serviceName string) (*grpc.ClientConn, error) {
	creds, err := cm.createTransportCredentials()
	if err != nil {
		return nil, err
	}

	resolverBuilder := consul.NewGRPCResolverBuilder(cm.registry)
	target := fmt.Sprintf("%s:///%s-grpc", resolverBuilder.Scheme(), serviceName)
	conn, err := grpc.NewClient(
		target,
		grpc.WithStatsHandler(middleware.NewClientStatsHandler()), // 添加gRPC统计处理程序
		grpc.WithTransportCredentials(creds),                      // 使用TLS证书
		grpc.WithResolvers(resolverBuilder),                       // 使用自定义的consul resolver
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),    // 配置负载均衡策略为轮询
	)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client for %q: %w", serviceName, err)
	}

	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof("Created gRPC round_robin client for service %q", serviceName)
	}
	return conn, nil
}

func (cm *ClientManager) createTransportCredentials() (credentials.TransportCredentials, error) {
	if cm.config == nil || !cm.config.UseTLS {
		return insecure.NewCredentials(), nil
	}

	hasCert := strings.TrimSpace(cm.config.CertFile) != ""
	hasKey := strings.TrimSpace(cm.config.KeyFile) != ""
	if hasCert != hasKey {
		return nil, apperror.InvalidArgument("client certificate and private key must be configured together")
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         strings.TrimSpace(cm.config.ServerName),
		InsecureSkipVerify: cm.config.InsecureSkipVerify, //nolint:gosec -- explicit development option
	}
	if cm.config.InsecureSkipVerify {
		if logger.SugaredLogger != nil {
			logger.SugaredLogger.Warn("InsecureSkipVerify is enabled; do not use it in production")
		}
	}

	if hasCert {
		cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if strings.TrimSpace(cm.config.CaFile) != "" {
		rootCAs, err := loadCertPool(cm.config.CaFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = rootCAs
	}

	return credentials.NewTLS(tlsConfig), nil
}

func loadCertPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, apperror.InvalidArgument("CA certificate does not contain a valid PEM certificate")
	}
	return pool, nil
}

/**
 * 检查客户端证书是否即将过期
**/
func (cm *ClientManager) CheckCertificateExpiry(ctx context.Context, _ string) error {
	if ctx == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return apperror.ToGRPC(err)
	}
	if cm.config == nil || !cm.config.UseTLS {
		return nil
	}

	hasCert := strings.TrimSpace(cm.config.CertFile) != ""
	hasKey := strings.TrimSpace(cm.config.KeyFile) != ""
	if !hasCert && !hasKey {
		return nil
	}
	if hasCert != hasKey {
		return apperror.ToGRPC(apperror.InvalidArgument("client certificate and private key must be configured together"))
	}

	cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
	if err != nil {
		return apperror.ToGRPC(fmt.Errorf("load client certificate: %w", err))
	}
	if len(cert.Certificate) == 0 {
		return apperror.ToGRPC(apperror.InvalidArgument("client certificate chain is empty"))
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return apperror.ToGRPC(fmt.Errorf("parse client certificate: %w", err))
	}

	remaining := time.Until(x509Cert.NotAfter) //转换为剩余时间
	if remaining <= 0 {
		return apperror.ToGRPC(apperror.InvalidArgument("client certificate has expired"))
	}
	if remaining < certificateWarnAfter {
		return apperror.ToGRPC(apperror.InvalidArgument("client certificate will expire in less than 1 day"))
	}
	return nil
}

// 证书定期预警
func (cm *ClientManager) MonitorCertificate(
	ctx context.Context,
	interval time.Duration,
	log *zap.Logger,
) {
	if cm == nil || cm.config == nil || !cm.config.UseTLS {
		return
	}
	check := func() {
		if err := cm.CheckCertificateExpiry(ctx, ""); err != nil {
			log.Error("certificate check failed", zap.Error(err))
		}
	}
	//启动立即检查一次
	check()
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

// 泛型定义通用客户端工厂函数类型
type ClientFactory[T any] func(grpc.ClientConnInterface) T

// 泛型定义通用客户端创建函数
func CreateClient[T any](ctx context.Context, manager *ClientManager, serviceName string, factory ClientFactory[T]) (T, error) {
	var zero T
	if manager == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client manager is nil"))
	}
	if factory == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client factory is nil"))
	}
	conn, err := manager.GetClient(ctx, serviceName)
	if err != nil {
		return zero, err
	}
	return factory(conn), nil
}

// admin	service
func (cm *ClientManager) AdminClient(ctx context.Context) (pbAdmin.AdminServiceClient, error) {
	return CreateClient(ctx, cm, "admin-service", pbAdmin.NewAdminServiceClient)
}

// LLM service
func (cm *ClientManager) LlmClient(ctx context.Context) (pbLlm.LlmServiceClient, error) {
	return CreateClient(ctx, cm, "llm-service", pbLlm.NewLlmServiceClient)
}

// 后续需要的服务。。。
//
// Close prevents future clients and closes existing ClientConns outside the
// manager lock so a slow close cannot block readers.
func (cm *ClientManager) Close() {
	if cm == nil {
		return
	}
	cm.mu.Lock()
	if cm.closed {
		cm.mu.Unlock()
		return
	}
	cm.closed = true
	conns := cm.conns
	cm.conns = make(map[string]*grpc.ClientConn)
	cm.mu.Unlock()

	for serviceName, conn := range conns {
		if err := conn.Close(); err != nil {
			if logger.SugaredLogger != nil {
				logger.SugaredLogger.Errorf("close gRPC connection for %q: %v", serviceName, err)
			}
		}
	}
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Info("all gRPC connections are closed")
	}
}
