package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"gateway/internal/consul"
	"gateway/internal/logger"
	"gateway/pkg/apperror"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin"
	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

/*
 * @Description: 创建grpc服务
 */
// GrpcConfig gRPC 客户端配置
type GrpcConfig struct {
	UseTLS             bool   // 是否启用 TLS
	InsecureSkipVerify bool   // 跳过证书验证（仅用于开发环境）
	CertFile           string // 客户端证书文件路径
	KeyFile            string // 客户端私钥文件路径
	CaFile             string // CA 证书文件路径
	ServerName         string // TLS Server Name
}

// ClientManager gRPC 客户端连接管理器
type ClientManager struct {
	registry *consul.ConsulRegistry      // 服务注册中心
	config   *GrpcConfig                 //	gRPC 配置
	mu       sync.RWMutex                // 读写锁
	conns    map[string]*grpc.ClientConn // 服务名称到 gRPC 客户端连接的映射
	rrCount  uint64                      // 轮询计数器
}

// 构造函数
func NewClientManager(rep *consul.ConsulRegistry, config *GrpcConfig) *ClientManager {
	if config == nil {
		config = &GrpcConfig{
			UseTLS:             false,
			InsecureSkipVerify: false,
		}
	}
	//证书自动推导机制：当证书路径未明确指定时，根据服务名自动推导
	// TODO
	if config.UseTLS && config.CertFile == "" && config.KeyFile == "" {
		logger.SugaredLogger.Warn("TLS enabled but certificate paths not specified, using default paths")
		// 默认证书路径：/etc/certs/{service-name}/
		defaultCertDir := "/etc/certs/gateway"
		config.CertFile = fmt.Sprintf("%s/client.crt", defaultCertDir)
		config.KeyFile = fmt.Sprintf("%s/client.key", defaultCertDir)
		if config.CaFile == "" {
			config.CaFile = fmt.Sprintf("%s/ca.crt", defaultCertDir)
		}
		if config.ServerName == "" {
			config.ServerName = "gateway-service"
		}
	}
	return &ClientManager{
		registry: rep,
		config:   config,
		conns:    make(map[string]*grpc.ClientConn), //初始化
	}
}

// 获取或创建 gRPC 连接（带连接池和负载均衡）
// TODO: 实现连接池和负载均衡
func (cm *ClientManager) GetClient(serviceName string) (*grpc.ClientConn, error) {
	if serviceName == "" {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("service name is empty"))
	}
	serviceName = strings.TrimSpace(serviceName)                      //清理空格
	addr, err := cm.resolveService(context.Background(), serviceName) //TODO: 传递context
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	key := fmt.Sprintf("%s-%s", serviceName, addr)
	cm.mu.RLock()             // 读锁
	conn, ok := cm.conns[key] // 查询连接
	cm.mu.RUnlock()           // 释放读锁
	if ok {
		return conn, nil // 如果连接存在，直接返回
	}
	//创建新的链接
	// TODO: 实现连接池和负载均衡
	cm.mu.Lock() // 写锁
	defer cm.mu.Unlock()
	// 双重检查，防止在获取锁后连接被创建
	if conn, ok := cm.conns[key]; ok {
		return conn, nil
	}
	// 根据配置创建传输凭证
	creds, err := cm.createTransportCredentials()
	// 创建新的 gRPC 客户端连接
	conn, err = grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(creds), //传输凭证
	)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	cm.conns[key] = conn // 将连接存储到连接池中
	logger.SugaredLogger.Infof("Created new gRPC client for service %q at %q", serviceName, addr)
	return conn, nil

}

// createTransportCredentials 创建传输凭证
// TODO: 实现传输凭证
func (cm *ClientManager) createTransportCredentials() (credentials.TransportCredentials, error) {

	if !cm.config.UseTLS {
		logger.SugaredLogger.Info("TLS is not enabled")
		return insecure.NewCredentials(), nil
	}
	// 生产环境：使用 TLS
	if cm.config.CertFile != "" && cm.config.KeyFile != "" {
		// 双向 TLS 认证（mTLS）
		cert, err := tls.LoadX509KeyPair(cm.config.CaFile, cm.config.KeyFile) //读取证书和私钥
		if err != nil {
			return nil, apperror.ToGRPC(err)
		}
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: cm.config.InsecureSkipVerify,
			ServerName:         cm.config.ServerName, // TLS Server Name
		}
		if cm.config.InsecureSkipVerify {
			logger.SugaredLogger.Warn("InsecureSkipVerify is enabled, this is not recommended for production")
			tlsConfig.InsecureSkipVerify = true //
		}
		if cm.config.CaFile != "" {
			caCert, err := os.ReadFile(cm.config.CaFile)
			if err != nil {
				return nil, apperror.ToGRPC(err)
			}
			caCertPool := x509.NewCertPool() //初始化证书池

			if !caCertPool.AppendCertsFromPEM(caCert) { //将 CA 证书添加到证书池中
				return nil, apperror.ToGRPC(apperror.InvalidArgument("failed to append CA certificate"))
			}

			tlsConfig.RootCAs = caCertPool
			logger.SugaredLogger.Infof("Loaded CA certificate from %q", cm.config.CaFile)
		}
		return credentials.NewTLS(tlsConfig), nil
	}

	// 单向 TLS（仅服务器认证）
	tlsConfig := &tls.Config{
		ServerName: cm.config.ServerName,
	}
	if cm.config.InsecureSkipVerify {
		logger.SugaredLogger.Warn("InsecureSkipVerify is enabled, this is not recommended for production")
		tlsConfig.InsecureSkipVerify = true
	}
	if cm.config.CaFile != "" {
		caCert, err := os.ReadFile(cm.config.CaFile)
		if err != nil {
			return nil, apperror.ToGRPC(err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, apperror.ToGRPC(apperror.InvalidArgument("failed to append CA certificate"))
		}
		tlsConfig.RootCAs = caCertPool
		logger.SugaredLogger.Infof("Loaded CA certificate from %q", cm.config.CaFile)
	}
	return credentials.NewTLS(tlsConfig), nil
	//return nil, apperror.ToGRPC(apperror.InvalidArgument("TLS configuration is invalid"))
}

// resolveService 从 Consul 解析服务地址（Round-Robin 负载均衡）
func (cm *ClientManager) resolveService(ctx context.Context, serviceName string) (string, error) {
	if ctx == nil {
		return "", apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "", apperror.ToGRPC(apperror.InvalidArgument("service name is empty"))
	}
	if cm.registry == nil {
		return "", apperror.ToGRPC(apperror.Unavailable("service registry is unavailable"))
	}
	grpcServiceName := serviceName + "-grpc"
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	entries, err := cm.registry.DiscoverGRPCService(discoveryCtx, grpcServiceName)
	if err != nil {
		return "", apperror.ToGRPC(err)
	}

	addresses := make([]string, 0, len(entries)) //初始化
	for _, entry := range entries {
		if entry == nil || entry.Service == nil || entry.Service.Port <= 0 || entry.Service.Port > 65535 {
			continue
		}

		host := strings.TrimSpace(entry.Service.Address)
		if host == "" && entry.Node != nil {
			host = strings.TrimSpace(entry.Node.Address)
		}
		if host == "" {
			continue
		}

		addresses = append(addresses, net.JoinHostPort(host, strconv.Itoa(entry.Service.Port)))
	}
	if len(addresses) == 0 {
		return "", apperror.ToGRPC(apperror.NotFound(
			fmt.Sprintf("no healthy instances found for service %q", grpcServiceName),
		))
	}

	// AddUint64 返回加一后的值，减一可让第一次选择从下标 0 开始。
	index := (atomic.AddUint64(&cm.rrCount, 1) - 1) % uint64(len(addresses))
	return addresses[index], nil
}

// CheckCertificateExpiry 检查证书有效期（健康检查）
func (cm *ClientManager) CheckCertificateExpiry(ctx context.Context, addr string) error {
	if !cm.config.UseTLS {
		return nil
	}
	// TODO: 实现证书有效期检查
	if cm.config.CertFile != "" && cm.config.KeyFile != "" {
		return nil

	}
	// 读取客户端证书
	cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
	if err != nil {
		return apperror.ToGRPC(err)
	}
	// 检查证书有效期
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0]) //解析证书
	if err != nil {
		return apperror.ToGRPC(err)
	}
	/*if time.Now().After(x509Cert.NotAfter) { //检查证书是否过期
		return apperror.ToGRPC(apperror.InvalidArgument("certificate has expired"))
	}*/
	now := time.Now()                                          //
	daysUntilExpiry := x509Cert.NotAfter.Sub(now).Hours() / 24 //计算证书有效期
	if daysUntilExpiry < 7 {                                   //如果证书有效期小于7天，则返回错误
		return apperror.ToGRPC(apperror.InvalidArgument("certificate will expire in less than 30 days"))
	}
	return nil
}

// ========== 类型安全的客户端工厂函数 ==========

// ClientFactory 通用客户端工厂函数类型
type ClientFactory[T any] func(grpc.ClientConnInterface) T

// CreateClient 通用客户端创建辅助函数（泛型函数），用于创建指定服务的客户端实例
func CreateClient[T any](manager *ClientManager, serviceName string, factory ClientFactory[T]) (T, error) {
	var zero T
	conn, err := manager.GetClient(serviceName) //获取客户端连接
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return factory(conn), nil
}

// ========== 客户端管理器 ==========
// AdminClient 获取管理服务客户端
func (cm *ClientManager) AdminClient() (pbAdmin.AdminServiceClient, error) {
	return CreateClient(cm, "admin-service", pbAdmin.NewAdminServiceClient)
}

// LlmClient 获取 AI 服务客户端
func (cm *ClientManager) LlmClient() (pbLlm.LlmServiceClient, error) {
	return CreateClient(cm, "llm-service", pbLlm.NewLlmServiceClient)
}

// Close 关闭所有 gRPC 连接
func (cm *ClientManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, conn := range cm.conns {
		if err := conn.Close(); err != nil {
			logger.SugaredLogger.Errorf("failed to close gRPC connection: %v", err)

		}
	}
	cm.conns = make(map[string]*grpc.ClientConn)
	logger.SugaredLogger.Info("all gRPC connections are closed")
}
