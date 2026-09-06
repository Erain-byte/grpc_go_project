// Package mqhandler 放置由 MQ Consumer 调用的应用层消息处理器。
package mqhandler

import (
	"admin/internal/model"
	"admin/internal/mq"
	"admin/internal/repository"
	"context"
	"errors"
	"math"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OperationLog 把稳定的 MQ 消息协议转换成当前数据库模型。
type OperationLog struct {
	repository repository.OperationLogRepository
}

// NewOperationLog 注入操作日志 Repository，处理器本身不负责创建数据库连接。
func NewOperationLog(repository repository.OperationLogRepository) (*OperationLog, error) {
	if repository == nil {
		return nil, errors.New("operation-log repository is nil")
	}
	return &OperationLog{repository: repository}, nil
}

// Handle 校验消息并把事件字段映射为 OperationLogModel 后幂等写入数据库。
func (h *OperationLog) Handle(ctx context.Context, event mq.OperationLogEvent) error {
	ctx, span := otel.Tracer("admin/internal/mqhandler").Start(
		ctx,
		"operation_log.persist",
		trace.WithAttributes(
			attribute.String("messaging.message.id", event.EventID),
			attribute.String("operation_log.module", event.Module),
			attribute.String("operation_log.action", event.Action),
		),
	)
	defer span.End()

	if err := event.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "operation-log event validation failed")
		return mq.MarkPermanent(err)
	}
	if event.AdminID > uint64(math.MaxUint) {
		err := errors.New("admin_id exceeds uint range")
		span.RecordError(err)
		span.SetStatus(codes.Error, "operation-log event validation failed")
		return mq.MarkPermanent(err)
	}
	// 数据库使用 1/0 保存成功状态，消息协议使用更直观的 bool。
	status := int8(0)
	if event.Success {
		status = 1
	}
	err := h.repository.Create(ctx, &model.OperationLogModel{
		EventID:    event.EventID,
		AdminID:    uint(event.AdminID),
		AdminName:  event.AdminName,
		Module:     event.Module,
		OperType:   event.Action,
		OperDesc:   event.Description,
		RequestURL: event.Path,
		Method:     event.Method,
		IP:         event.ClientIP,
		UserAgent:  event.UserAgent,
		Status:     status,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "operation-log persistence failed")
	}
	return err
}
