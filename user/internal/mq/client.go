package mq

import (
	"context"
	"errors"
	"sync"

	"github.com/streadway/amqp"
)

// Client is the interface for the message queue client
// errors are returned as a string
// 不可重新链接异常
var ErrClientClosed = errors.New("RabbitMQ client is permanently closed")

// 可重链接异常
var ErrClientReconnect = errors.New("RabbitMQ client is reconnecting")

// 接口定义
type Client interface {
	OpenChannel() (*amqp.Channel, error)
	Reconnect(ctx context.Context) error
	Ping(ctx context.Context) error
	IsClosed() bool
	Close() error
}

// 结构体
type rabbitMQClient struct {
	//读写锁保护conn和close
	mu sync.RWMutex
	//确保同一时间只有一个goroutine在执行Reconnect
	reconnectMu sync.Mutex
}
