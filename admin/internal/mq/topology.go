package mq

import (
	"admin/internal/config"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DeclareOperationLogTopology 声明操作日志的主队列、延迟重试队列和死信队列。
// RabbitMQ 的声明操作是幂等的，只要同名对象的参数保持一致即可重复执行。
func DeclareOperationLogTopology(channel *amqp.Channel, cfg config.RabbitMQOperationLog) error {
	if channel == nil {
		return errors.New("declare RabbitMQ topology: channel is nil")
	}
	retryDelay, err := time.ParseDuration(cfg.RetryDelay)
	if err != nil {
		return fmt.Errorf("parse RabbitMQ retry delay %q: %w", cfg.RetryDelay, err)
	}
	if retryDelay <= 0 {
		return errors.New("RabbitMQ retry delay must be positive")
	}
	// 主 Exchange 按业务配置类型创建；重试和死信使用精确匹配的 direct 类型。
	for _, exchange := range []struct{ name, kind string }{
		{cfg.Exchange, cfg.ExchangeType}, {cfg.RetryExchange, amqp.ExchangeDirect},
		{cfg.DeadLetterExchange, amqp.ExchangeDirect},
	} {
		if err := channel.ExchangeDeclare(exchange.name, exchange.kind, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare exchange %s: %w", exchange.name, err)
		}
	}
	// 主队列被 Reject/Nack 且不重新入队时，由 RabbitMQ 自动转入死信 Exchange。
	mainArgs := amqp.Table{"x-dead-letter-exchange": cfg.DeadLetterExchange, "x-dead-letter-routing-key": cfg.DeadLetterRoutingKey}
	if _, err := channel.QueueDeclare(cfg.Queue, true, false, false, false, mainArgs); err != nil {
		return fmt.Errorf("declare queue %s: %w", cfg.Queue, err)
	}
	if err := channel.QueueBind(cfg.Queue, cfg.RoutingKey, cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", cfg.Queue, err)
	}
	// 重试队列不直接消费；消息 TTL 到期后通过死信机制重新投回主 Exchange。
	retryArgs := amqp.Table{"x-message-ttl": retryDelay.Milliseconds(), "x-dead-letter-exchange": cfg.Exchange, "x-dead-letter-routing-key": cfg.RoutingKey}
	if _, err := channel.QueueDeclare(cfg.RetryQueue, true, false, false, false, retryArgs); err != nil {
		return fmt.Errorf("declare retry queue %s: %w", cfg.RetryQueue, err)
	}
	if err := channel.QueueBind(cfg.RetryQueue, cfg.RetryRoutingKey, cfg.RetryExchange, false, nil); err != nil {
		return fmt.Errorf("bind retry queue %s: %w", cfg.RetryQueue, err)
	}
	if _, err := channel.QueueDeclare(cfg.DeadLetterQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead-letter queue %s: %w", cfg.DeadLetterQueue, err)
	}
	if err := channel.QueueBind(cfg.DeadLetterQueue, cfg.DeadLetterRoutingKey, cfg.DeadLetterExchange, false, nil); err != nil {
		return fmt.Errorf("bind dead-letter queue %s: %w", cfg.DeadLetterQueue, err)
	}
	return nil
}
