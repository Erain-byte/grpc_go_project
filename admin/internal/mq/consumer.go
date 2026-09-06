package mq

import (
	"admin/internal/config"
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// OperationLogHandler 由应用层实现，消费者只负责消息可靠性，不直接依赖数据库。
type OperationLogHandler interface {
	Handle(ctx context.Context, event OperationLogEvent) error
}

type Consumer struct {
	client            Client
	config            config.RabbitMQConfig
	handler           OperationLogHandler
	publisher         messagePublisher
	logger            *zap.Logger
	reconnectInterval time.Duration
}

// NewConsumer 组装消费所需的连接、处理器、重试发布器和日志组件。
func NewConsumer(client Client, cfg config.RabbitMQConfig, handler OperationLogHandler, publisher messagePublisher, logger *zap.Logger) (*Consumer, error) {
	if client == nil {
		return nil, errors.New("RabbitMQ consumer client is nil")
	}
	if handler == nil {
		return nil, errors.New("operation-log handler is nil")
	}
	if publisher == nil {
		return nil, errors.New("RabbitMQ retry publisher is nil")
	}
	if logger == nil {
		return nil, errors.New("RabbitMQ consumer logger is nil")
	}
	interval, err := time.ParseDuration(cfg.ReconnectInterval)
	if err != nil || interval <= 0 {
		return nil, fmt.Errorf("invalid RabbitMQ reconnect interval %q", cfg.ReconnectInterval)
	}
	return &Consumer{client: client, config: cfg, handler: handler, publisher: publisher, logger: logger, reconnectInterval: interval}, nil
}

// Start 持续运行消费者，直到 ctx 被取消。
// Channel 或连接异常时等待 reconnectInterval，然后重新连接并恢复消费。
func (c *Consumer) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("RabbitMQ consumer context is nil")
	}
	for {
		err := c.consume(ctx)
		if ctx.Err() != nil {
			return nil
		}
		c.logger.Error("RabbitMQ consumer interrupted; reconnecting", zap.Error(err), zap.Duration("after", c.reconnectInterval))
		if waitContext(ctx, c.reconnectInterval) != nil {
			return nil
		}
		if err := c.client.Reconnect(ctx); err != nil {
			c.logger.Error("RabbitMQ reconnect failed", zap.Error(err))
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	channel, err := c.client.OpenChannel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err := DeclareOperationLogTopology(channel, c.config.OperationLog); err != nil {
		return err
	}
	// Prefetch 限制尚未 ACK 的消息数量，防止消费者处理不过来时堆满内存。
	if err := channel.Qos(c.config.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("set RabbitMQ consumer QoS: %w", err)
	}
	// autoAck=false：只有数据库处理成功或消息被可靠转移后才手动 ACK。
	deliveries, err := channel.Consume(c.config.OperationLog.Queue, c.config.ConnectionName+"-operation-log", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start RabbitMQ consumer: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RabbitMQ delivery channel closed")
			}
			c.handleDelivery(ctx, delivery)
		}
	}
}

func (c *Consumer) handleDelivery(parent context.Context, delivery amqp.Delivery) {
	// 从 AMQP Headers 提取上游 Trace，使消费 Span 与发布 Span 属于同一条链路。
	ctx := extractTraceContext(parent, delivery.Headers)
	ctx, span := otel.Tracer("admin/internal/mq").Start(ctx, "rabbitmq consume "+c.config.OperationLog.Queue,
		trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", c.config.OperationLog.Queue),
			attribute.String("messaging.operation.name", "process"),
			attribute.String("messaging.message.id", delivery.MessageId),
			attribute.Int("messaging.message.retry_count", retryCount(delivery.Headers))))
	defer span.End()
	event, err := decodeOperationLog(delivery.Body)
	if err == nil {
		// 业务处理成功后确认原消息，RabbitMQ 才会把它从主队列移除。
		err = c.handler.Handle(ctx, event)
	}
	if err == nil {
		if ackErr := delivery.Ack(false); ackErr != nil {
			span.RecordError(ackErr)
			span.SetStatus(codes.Error, "RabbitMQ acknowledgement failed")
			c.logger.Error("ack RabbitMQ message failed", zap.Error(ackErr))
		}
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	// 消息格式错误或业务明确标记为永久错误时，重试不会解决问题，直接进入 DLQ。
	if validationErr := event.Validate(); validationErr != nil || isPermanent(err) {
		c.moveToDeadLetter(ctx, delivery, err)
		return
	}
	c.retryOrDeadLetter(ctx, delivery, err)
}

func (c *Consumer) retryOrDeadLetter(ctx context.Context, delivery amqp.Delivery, cause error) {
	count := retryCount(delivery.Headers)
	if count >= c.config.OperationLog.MaxRetries {
		c.moveToDeadLetter(ctx, delivery, cause)
		return
	}
	// 复制 Header，避免修改当前 Delivery 持有的原始 map。
	headers := cloneHeaders(delivery.Headers)
	headers[retryCountHeader] = int32(count + 1)
	if err := c.publisher.Republish(ctx, c.config.OperationLog.RetryExchange, c.config.OperationLog.RetryRoutingKey, delivery, headers); err != nil {
		c.logger.Error("publish RabbitMQ retry message failed", zap.Error(err), zap.String("message_id", delivery.MessageId))
		_ = delivery.Nack(false, true)
		return
	}
	// 只有重试消息得到 Broker Confirm 后，才能 ACK 原消息，避免消息丢失。
	if err := delivery.Ack(false); err != nil {
		c.logger.Error("ack retried RabbitMQ message failed", zap.Error(err))
	}
	c.logger.Warn("operation-log message scheduled for retry", zap.Int("retry_count", count+1), zap.Error(cause), zap.String("message_id", delivery.MessageId))
}

func (c *Consumer) moveToDeadLetter(ctx context.Context, delivery amqp.Delivery, cause error) {
	headers := cloneHeaders(delivery.Headers)
	headers["x-error"] = cause.Error()
	if err := c.publisher.Republish(ctx, c.config.OperationLog.DeadLetterExchange, c.config.OperationLog.DeadLetterRoutingKey, delivery, headers); err != nil {
		c.logger.Error("publish RabbitMQ dead-letter message failed", zap.Error(err), zap.String("message_id", delivery.MessageId))
		_ = delivery.Nack(false, true)
		return
	}
	// 死信发布成功后再 ACK 原消息，保证错误消息仍可在 DLQ 中排查和补偿。
	if err := delivery.Ack(false); err != nil {
		c.logger.Error("ack dead-lettered RabbitMQ message failed", zap.Error(err))
	}
	c.logger.Error("operation-log message moved to DLQ", zap.Error(cause), zap.String("message_id", delivery.MessageId))
}

func waitContext(ctx context.Context, duration time.Duration) error {
	// 使用可取消的定时等待，服务退出时不用等完整个重连周期。
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
