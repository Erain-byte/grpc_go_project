package mq

import (
	"admin/internal/config"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OperationLogPublisher 是业务层使用的操作日志发布接口。
type OperationLogPublisher interface {
	Publish(ctx context.Context, event OperationLogEvent) error
	Close() error
}

// messagePublisher 只供消费者内部重新发布重试消息和死信消息。
type messagePublisher interface {
	Republish(ctx context.Context, exchange, routingKey string, delivery amqp.Delivery, headers amqp.Table) error
}

type Publisher struct {
	mu             sync.Mutex // AMQP Channel 不在多个发布操作之间并发共享，因此串行发布。
	client         Client
	config         config.RabbitMQConfig
	channel        *amqp.Channel
	confirmTimeout time.Duration
	closed         bool
}

var _ OperationLogPublisher = (*Publisher)(nil)
var _ messagePublisher = (*Publisher)(nil)

// NewPublisher 创建独立发布 Channel、声明拓扑并开启 Publisher Confirm。
func NewPublisher(client Client, cfg config.RabbitMQConfig) (*Publisher, error) {
	if client == nil {
		return nil, errors.New("RabbitMQ publisher client is nil")
	}
	confirmTimeout, err := time.ParseDuration(cfg.PublishConfirmTimeout)
	if err != nil || confirmTimeout <= 0 {
		return nil, fmt.Errorf("invalid publish confirm timeout %q", cfg.PublishConfirmTimeout)
	}
	p := &Publisher{client: client, config: cfg, confirmTimeout: confirmTimeout}
	if err := p.openChannelLocked(); err != nil {
		return nil, err
	}
	return p, nil
}

// Publish 校验并序列化操作日志事件，同时把当前 Trace 上下文写入消息头。
func (p *Publisher) Publish(ctx context.Context, event OperationLogEvent) error {
	if ctx == nil {
		return errors.New("RabbitMQ publish context is nil")
	}
	body, err := encodeOperationLog(event)
	if err != nil {
		return fmt.Errorf("build operation-log message: %w", err)
	}
	ctx, span := otel.Tracer("admin/internal/mq").Start(ctx, "rabbitmq publish "+p.config.OperationLog.RoutingKey,
		trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", p.config.OperationLog.Exchange),
			attribute.String("messaging.rabbitmq.destination.routing_key", p.config.OperationLog.RoutingKey),
			attribute.String("messaging.message.id", event.EventID)))
	defer span.End()
	// 注入 traceparent/tracestate，消费者可以继续同一条调用链。
	headers := make(amqp.Table)
	injectTraceContext(ctx, headers)
	message := amqp.Publishing{Headers: headers, ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: event.EventID, Timestamp: event.OccurredAt, Type: event.EventType,
		AppId: p.config.ConnectionName, Body: body}
	if err := p.publish(ctx, p.config.OperationLog.Exchange, p.config.OperationLog.RoutingKey, message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "RabbitMQ publish failed")
		return err
	}
	return nil
}

// Republish 保留原消息内容和属性，只替换目标 Exchange、RoutingKey 与 Headers。
// 消费失败时会用它把消息投递到重试队列或死信队列。
func (p *Publisher) Republish(ctx context.Context, exchange, routingKey string, delivery amqp.Delivery, headers amqp.Table) error {
	ctx, span := otel.Tracer("admin/internal/mq").Start(
		ctx,
		"rabbitmq publish "+routingKey,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", exchange),
			attribute.String("messaging.rabbitmq.destination.routing_key", routingKey),
			attribute.String("messaging.message.id", delivery.MessageId),
		),
	)
	defer span.End()

	// 用当前重新发布 Span 更新 traceparent，使后续重试消费连接到本次发布节点。
	if headers == nil {
		headers = make(amqp.Table)
	}
	injectTraceContext(ctx, headers)

	message := amqp.Publishing{Headers: headers, ContentType: delivery.ContentType, ContentEncoding: delivery.ContentEncoding,
		DeliveryMode: amqp.Persistent, Priority: delivery.Priority, CorrelationId: delivery.CorrelationId,
		ReplyTo: delivery.ReplyTo, MessageId: delivery.MessageId, Timestamp: delivery.Timestamp,
		Type: delivery.Type, UserId: delivery.UserId, AppId: delivery.AppId, Body: delivery.Body}
	if err := p.publish(ctx, exchange, routingKey, message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "RabbitMQ republish failed")
		return err
	}
	return nil
}

func (p *Publisher) publish(ctx context.Context, exchange, routingKey string, message amqp.Publishing) error {
	// 锁覆盖“发布 + 等待确认”，保证每次确认都对应当前发布的消息。
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("RabbitMQ publisher is closed")
	}
	if p.channel == nil || p.channel.IsClosed() {
		if err := p.recoverChannelLocked(ctx); err != nil {
			return err
		}
	}
	// DeferredConfirm 可以等待 RabbitMQ Broker 明确 ACK 当前消息。
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(ctx, exchange, routingKey, false, false, message)
	if err != nil {
		_ = p.closeChannelLocked()
		return fmt.Errorf("publish RabbitMQ message: %w", err)
	}
	if confirmation == nil {
		return errors.New("RabbitMQ publisher confirm is not enabled")
	}
	waitCtx, cancel := context.WithTimeout(ctx, p.confirmTimeout)
	defer cancel()
	acked, err := confirmation.WaitContext(waitCtx)
	if err != nil {
		return fmt.Errorf("wait for RabbitMQ publisher confirm: %w", err)
	}
	if !acked {
		return errors.New("RabbitMQ broker negatively acknowledged message")
	}
	return nil
}

func (p *Publisher) recoverChannelLocked(ctx context.Context) error {
	// Channel 失效不一定表示 Connection 失效，因此只在连接关闭时重连。
	_ = p.closeChannelLocked()
	if p.client.IsClosed() {
		if err := p.client.Reconnect(ctx); err != nil {
			return fmt.Errorf("reconnect RabbitMQ publisher: %w", err)
		}
	}
	return p.openChannelLocked()
}
func (p *Publisher) openChannelLocked() error {
	channel, err := p.client.OpenChannel()
	if err != nil {
		return err
	}
	if err := DeclareOperationLogTopology(channel, p.config.OperationLog); err != nil {
		_ = channel.Close()
		return err
	}
	// 开启 Publisher Confirm，避免“Publish 返回 nil 但 Broker 尚未接收”的误判。
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		return fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	p.channel = channel
	return nil
}
func (p *Publisher) closeChannelLocked() error {
	if p.channel == nil {
		return nil
	}
	channel := p.channel
	p.channel = nil
	if channel.IsClosed() {
		return nil
	}
	return channel.Close()
}
func (p *Publisher) Close() error {
	// Close 可以被重复调用，并通过互斥锁等待正在进行的发布结束。
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	return p.closeChannelLocked()
}
