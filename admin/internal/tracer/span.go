package tracer

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecordError 将错误事件和错误状态写入 Span，供业务层统一复用。
// err 为 nil 时不做任何处理，调用方无需重复判断。
func RecordError(span trace.Span, err error, description string) {
	if span == nil || err == nil {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, description)
}
