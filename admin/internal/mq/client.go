package mq

import (
	"admin/internal/config"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrClientClosed 表示 RabbitMQ 客户端已被永久关闭，不能继续创建 Channel 或重连。
var ErrClientClosed = errors.New("RabbitMQ client is closed")

// Client 统一管理一条长期存在的 AMQP Connection。
// Channel 是基于 Connection 创建的短生命周期对象，使用完后由调用方关闭。
type Client interface {
	OpenChannel() (*amqp.Channel, error)
	Reconnect(ctx context.Context) error
	Ping(ctx context.Context) error
	IsClosed() bool
	Close() error
}

type rabbitMQClient struct {
	mu          sync.RWMutex // 保护 conn 和 closed，允许多个读取操作并发执行。
	reconnectMu sync.Mutex   // 保证同一时间只有一个协程执行重连或关闭。
	conn        *amqp.Connection
	config      config.RabbitMQConfig
	closed      bool
}

var _ Client = (*rabbitMQClient)(nil)

// NewClient 创建并验证一条 RabbitMQ Connection。
func NewClient(cfg config.RabbitMQConfig) (Client, error) {
	conn, err := dial(cfg)
	if err != nil {
		return nil, err
	}
	return &rabbitMQClient{conn: conn, config: cfg}, nil
}

// dial 根据配置创建新的底层 AMQP Connection。
func dial(cfg config.RabbitMQConfig) (*amqp.Connection, error) {
	heartbeat, err := time.ParseDuration(cfg.Heartbeat)
	if err != nil {
		return nil, fmt.Errorf("parse RabbitMQ heartbeat: %w", err)
	}
	dialTimeout, err := time.ParseDuration(cfg.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse RabbitMQ dial timeout: %w", err)
	}
	// 用户名、密码和虚拟主机通过 Config 单独传递，避免敏感信息出现在地址和日志中。
	endpoint := "amqp://" + cfg.MqAddress() + "/"
	conn, err := amqp.DialConfig(endpoint, amqp.Config{
		SASL:  []amqp.Authentication{&amqp.PlainAuth{Username: cfg.Username, Password: cfg.Password}},
		Vhost: cfg.VHost, Heartbeat: heartbeat, Locale: "en_US",
		Dial:       amqp.DefaultDial(dialTimeout),
		Properties: amqp.Table{"connection_name": cfg.ConnectionName},
	})
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ at %s: %w", cfg.MqAddress(), err)
	}
	return conn, nil
}

func (c *rabbitMQClient) OpenChannel() (*amqp.Channel, error) {
	// 这里只读取连接状态和连接指针，所以使用读锁。
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.conn == nil || c.conn.IsClosed() {
		return nil, ErrClientClosed
	}
	channel, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	return channel, nil
}

// Reconnect 替换已经断开的 AMQP Connection。
// reconnectMu 防止发布者和消费者同时发现断线后重复创建连接。
func (c *rabbitMQClient) Reconnect(ctx context.Context) error {
	if ctx == nil {
		return errors.New("RabbitMQ reconnect context is nil")
	}
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	if c.conn != nil && !c.conn.IsClosed() {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	conn, err := dial(c.config)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close()
		return ErrClientClosed
	}
	// 新连接创建成功后再替换旧连接，避免重连失败时丢失原有对象。
	old := c.conn
	c.conn = conn
	c.mu.Unlock()
	if old != nil && !old.IsClosed() {
		_ = old.Close()
	}
	return nil
}

func (c *rabbitMQClient) Ping(ctx context.Context) error {
	if ctx == nil {
		return errors.New("RabbitMQ ping context is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// AMQP 没有独立的 Ping 命令；能够成功创建并关闭 Channel 即说明连接可用。
	channel, err := c.OpenChannel()
	if err != nil {
		return err
	}
	return channel.Close()
}

func (c *rabbitMQClient) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed || c.conn == nil || c.conn.IsClosed()
}

func (c *rabbitMQClient) Close() error {
	// Close 与 Reconnect 互斥，避免关闭过程中又建立一条新连接。
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn == nil || conn.IsClosed() {
		return nil
	}
	return conn.Close()
}
