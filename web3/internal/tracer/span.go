package tracer

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecordError 将错误事件和错误状态写入 Span；err 为 nil 时不做处理。
func RecordError(span trace.Span, err error, description string) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, description)
}
