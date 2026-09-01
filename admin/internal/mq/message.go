package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	OperationLogEventType     = "admin.operation_log.created" // 事件类型用于区分消息用途。
	OperationLogSchemaVersion = 1                             // 协议版本用于控制消息兼容性。
	retryCountHeader          = "x-retry-count"               // Header 中记录已经重试的次数。
)

// OperationLogEvent 是带版本的消息协议，故意与数据库 Model 分离。
// 这样修改数据库字段时，不会无意中改变已经发送到队列里的消息格式。
type OperationLogEvent struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	AdminID       uint64    `json:"admin_id"`
	AdminName     string    `json:"admin_name"`
	Module        string    `json:"module"`
	Action        string    `json:"action"`
	Description   string    `json:"description"`
	Success       bool      `json:"success"`
	ErrorCode     string    `json:"error_code,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	Method        string    `json:"method,omitempty"`
	Path          string    `json:"path,omitempty"`
	ClientIP      string    `json:"client_ip,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
}

func (e OperationLogEvent) Validate() error {
	// 消费者和发布者都执行相同校验，尽早拒绝无法正确处理的消息。
	switch {
	case strings.TrimSpace(e.EventID) == "":
		return errors.New("event_id is required")
	case e.EventType != OperationLogEventType:
		return fmt.Errorf("unsupported event_type %q", e.EventType)
	case e.SchemaVersion != OperationLogSchemaVersion:
		return fmt.Errorf("unsupported schema_version %d", e.SchemaVersion)
	case e.OccurredAt.IsZero():
		return errors.New("occurred_at is required")
	case e.AdminID == 0:
		return errors.New("admin_id is required")
	case strings.TrimSpace(e.Module) == "":
		return errors.New("module is required")
	case strings.TrimSpace(e.Action) == "":
		return errors.New("action is required")
	default:
		return nil
	}
}

func encodeOperationLog(event OperationLogEvent) ([]byte, error) {
	if err := event.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("encode operation-log event: %w", err)
	}
	return body, nil
}

func decodeOperationLog(body []byte) (OperationLogEvent, error) {
	var event OperationLogEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return event, fmt.Errorf("decode operation-log event: %w", err)
	}
	if err := event.Validate(); err != nil {
		return event, fmt.Errorf("validate operation-log event: %w", err)
	}
	return event, nil
}

type amqpHeaderCarrier amqp.Table

// amqpHeaderCarrier 把 amqp.Table 适配成 OpenTelemetry 的 TextMapCarrier。
// OpenTelemetry 会通过它读写 traceparent 和 tracestate。

func (c amqpHeaderCarrier) Get(key string) string {
	value, ok := c[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}
func (c amqpHeaderCarrier) Set(key, value string) { c[key] = value }
func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

var _ propagation.TextMapCarrier = amqpHeaderCarrier{}

func injectTraceContext(ctx context.Context, headers amqp.Table) {
	if headers != nil {
		otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))
	}
}
func extractTraceContext(ctx context.Context, headers amqp.Table) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier(headers))
}
func cloneHeaders(source amqp.Table) amqp.Table {
	// amqp.Table 本质是 map；复制后修改重试次数不会影响原始消息头。
	result := make(amqp.Table, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
func retryCount(headers amqp.Table) int {
	// AMQP 解码整数时可能得到不同宽度的整型，因此这里统一转换成 int。
	if headers == nil {
		return 0
	}
	switch value := headers[retryCountHeader].(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	default:
		return 0
	}
}
