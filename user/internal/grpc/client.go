package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"user/internal/consul"
	"user/internal/logger"
	"user/internal/middleware"
	"user/pkg/apperror"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// 本脚本实现服务发现和负载均衡
// 配置负载均衡策略为轮询
const roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// grpc客服端链接转发管理器
type ClientManager struct {
	registry consul.GRPCServiceWatcher
	config   *GrpcConfig
	mu       sync.RWMutex
	//缓存的连接
	conns map[string]*grpc.ClientConn
	//标记是否关闭
	pendingConnection map[string]*pendingConnection
	closed            bool
}

// 链接配置
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
func NewClientManager(registry consul.GRPCServiceWatcher, grpcConfig *GrpcConfig) *ClientManager {
	cfg := GrpcConfig{} // 默认配置
	if grpcConfig != nil {
		cfg = *grpcConfig
	}
	return &ClientManager{
		registry:          registry,
		config:            &cfg,
		conns:             make(map[string]*grpc.ClientConn),
		pendingConnection: make(map[string]*pendingConnection),
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
	//读写锁读取
	cm.mu.RLock()
	conn := cm.conns[servicerName] // 获取连接
	closed := cm.closed
	cm.mu.RUnlock()
	if closed {
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if conn != nil {
		return conn, nil
	}
	//开始写入
	cm.mu.Lock()
	if cm.closed {
		cm.mu.Unlock()
		return nil, apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	}
	if conn = cm.conns[servicerName]; conn != nil {
		cm.mu.Unlock()
		return conn, nil
	}
	// 如果有其他goroutine正在创建连接，则等待
	if pending := cm.pendingConnection[servicerName]; pending != nil {
		cm.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, apperror.ToGRPC(ctx.Err()) // 超时或取消
		case <-pending.done:
			return pending.conn, pending.err
		}
	}
	// 创建一个等待连接创建完成的channel
	pending := &pendingConnection{done: make(chan struct{})}
	cm.pendingConnection[servicerName] = pending
	cm.mu.Unlock()
	// 创建连接
	conn, err := cm.newClient(servicerName)
	if err != nil {
		err = apperror.ToGRPC(err)
	}
	cm.mu.Lock()
	// 更新连接状态
	delete(cm.pendingConnection, servicerName)
	if cm.closed && conn != nil {
		conn.Close()
		conn = nil
		err = apperror.ToGRPC(apperror.Unavailable("gRPC client manager is closed"))
	} else {
		cm.conns[servicerName] = conn
	}
	pending.conn = conn
	pending.err = err
	close(pending.done)
	cm.mu.Unlock()
	return conn, err
}

// newClient方法实现查询
func (cm *ClientManager) newClient(serviceName string) (*grpc.ClientConn, error) {
	//获取证书
	creds, err := cm.createTransportCredentials()
	if err != nil {
		return nil, err
	}
	//创建链接
	resolverBuilder := consul.NewGRPCResolverBuilder(cm.registry)
	target := fmt.Sprintf("%s:///%s-grpc", resolverBuilder.Scheme(), serviceName)
	conn, err := grpc.NewClient(
		target,
		// 添加gRPC统计处理程序
		grpc.WithStatsHandler(middleware.NewClientStatsHandler()),
		//grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		//使用证书
		grpc.WithTransportCredentials(creds),
		// 使用自定义的consul resolver
		grpc.WithResolvers(resolverBuilder),
		// 使用轮询策略
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client for %q: %w", serviceName, err)
	}
	//写入日志
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof("gRPC client for %q created", serviceName)
	}
	return conn, nil
}

// Close 阻止后续新建客户端连接，并关闭当前已存在的 ClientConn
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
	for _, conn := range conns {
		err := conn.Close()
		if err != nil {
			if logger.SugaredLogger != nil {
				logger.SugaredLogger.Errorf("close gRPC client connection: %v", err)
			}
		}
	}
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Info("gRPC client manager closed")
	}
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
