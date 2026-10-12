package grpcclient

import (
	"admin/internal/consul"
	"admin/internal/logger"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"

	apperror "admin/pkg/apperorr"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// 配置负载均衡策略为轮询
const roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// grpc客服端链接转发管理器
type ClientManager struct {
	registry consul.GRPCServiceWatcher
	config   *GrpcConfig
	mu       sync.RWMutex
	conns    map[string]*grpc.ClientConn
	pending  map[string]*pendingConnection
	closed   bool
}
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

// 构造函数
func NewClientManager(registry consul.GRPCServiceWatcher, config *GrpcConfig) *ClientManager {
	cfg := GrpcConfig{}
	if config != nil {
		cfg = *config
	}
	return &ClientManager{
		registry: registry,
		config:   &cfg,
		conns:    make(map[string]*grpc.ClientConn),
		pending:  make(map[string]*pendingConnection),
	}
}

// GetClient 返回 serviceName 对应的共享 ClientConn
func (cm *ClientManager) GetClient(ctx context.Context, servicerName string) (*grpc.ClientConn, error) {

	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	servicerName = strings.TrimSpace(servicerName)
	if servicerName == "" {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("service name is empty"))
	}
	if cm.registry == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("registry is nil"))
	}

	cm.mu.RLock()
	conn := cm.conns[servicerName]
	cloesd := cm.closed
	cm.mu.RUnlock()

	if cloesd {
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if conn != nil {
		return conn, nil
	}
	cm.mu.Lock()
	if cm.closed {
		cm.mu.Unlock()
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if cnn := cm.conns[servicerName]; cnn != nil {
		cm.mu.Unlock()
		return cnn, nil
	}
	// 如果有其他goroutine正在创建连接，则等待
	if pending := cm.pending[servicerName]; pending != nil {
		cm.mu.Unlock()
		select {
		case <-pending.done:
			return pending.conn, pending.err
		case <-ctx.Done():
			return nil, apperror.ToGRPC(ctx.Err())
		}
	}
	// 创建一个等待连接创建完成的channel
	pending := &pendingConnection{done: make(chan struct{})}
	cm.pending[servicerName] = pending
	cm.mu.Unlock()
	conn, err := cm.newClient(servicerName)
	if err != nil {
		err = apperror.ToGRPC(err)
	}
	cm.mu.Lock()
	delete(cm.pending, servicerName)
	if cm.closed && conn != nil {
		_ = conn.Close()
		conn = nil
		err = apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	} else if err == nil {
		cm.conns[servicerName] = conn
	}
	pending.conn = conn
	pending.err = err
	close(pending.done)
	cm.mu.Unlock()
	return conn, err
}

// newClient 创建客户端
func (cm *ClientManager) newClient(serviceName string) (*grpc.ClientConn, error) {
	//获取证书
	cred, err := cm.createTransportCredentials()
	if err != nil {
		return nil, err
	}
	//创建链接
	resolverBuilder := consul.NewGRPCResolverBuilder(cm.registry)
	target := fmt.Sprintf("%s:///%s-grpc", resolverBuilder.Scheme(), serviceName)
	conn, err := grpc.NewClient(
		target,
		// 添加gRPC统计处理程序
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		// 使用TLS证书
		grpc.WithTransportCredentials(cred),
		// 使用自定义的consul resolver
		grpc.WithResolvers(resolverBuilder),
		//轮询策略
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client for %q: %w", serviceName, err)
	}

	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof("Created gRPC round_robin client for service %q", serviceName)
	}
	return conn, nil
}

// createTransportCredentials 创建传输凭证
func (cm *ClientManager) createTransportCredentials() (credentials.TransportCredentials, error) {
	if cm.config == nil || !cm.config.UseTLS {
		return insecure.NewCredentials(), nil
	}
	hasCert := strings.TrimSpace(cm.config.CertFile) != ""
	hasKey := strings.TrimSpace(cm.config.KeyFile) != ""
	if hasCert != hasKey {
		return nil, apperror.InvalidArgument("client certificate and private key must be configured together")
	}
	//读取证书和私钥
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         strings.TrimSpace(cm.config.ServerName),
		InsecureSkipVerify: cm.config.InsecureSkipVerify,
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

// Close 阻止后续新建客户端连接，并关闭当前已存在的 ClientConn（
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
				logger.SugaredLogger.Errorf("close gRPC client for service %q: %v", serviceName, err)
			}
		}
	}
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Info("all gRPC connections are closed")
	}
}

// 后续内部服务继续添加
// func(cm *ClientManager) xxxClient(ctx context.Context) (pbxxx.XxxServiceClient, error) {

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

// 泛型定义通用客户端工厂函数类型
// 泛型定义通用客户端创建函数
/*func CreateClient[T any](ctx context.Context, cm *ClientManager, servicerName string, factory func(grpc.ClientConnInterface) T) (T, error) {
	var zero T
	if cm == nil {
		return zero, apperror.InvalidArgument("client manager is nil")
	}
	if factory == nil {
		return zero, apperror.InvalidArgument("client factory is nil")
	}
	servicerName = strings.TrimSpace(servicerName)
	if servicerName == "" {
		return zero, apperror.InvalidArgument("service name is empty")
	}
	conn, err := cm.GetClient(ctx, servicerName)
	if err != nil {
		return zero, err
	}
	return factory(conn), nil
}*/
