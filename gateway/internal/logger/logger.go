package logger

import (
	"context"
	"gateway/internal/config"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"
	"gopkg.in/natefinch/lumberjack.v2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger        *zap.Logger        // Logger 是 zap 的核心日志记录器，提供了高性能的结构化日志记录功能。
	SugaredLogger *zap.SugaredLogger // SugaredLogger 提供了更方便的日志记录方法。

)

func InitializeLogger(cfg config.LoggerConfig) error {
	// 创建日志目录
	logDir := filepath.Dir(cfg.Filename) // 获取日志文件的目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	//配置日志级别
	level := parseLevel(cfg.Level)
	// 配置日志编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	} else {
		encoder = zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	}
	// 配置日志输出（文件 + 控制台）
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,    // MB
		MaxBackups: cfg.MaxBackups, // 保留文件数
		MaxAge:     cfg.MaxAge,     // 天数
		Compress:   cfg.Compress,   // 是否压缩
	})

	// 创建核心
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level), // 控制台输出
		zapcore.NewCore(encoder, fileWriter, level),                 // 文件输出
	)

	// 创建 logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	SugaredLogger = Logger.Sugar()
	return nil
}

// parseLevel 解析日志级别字符串
func parseLevel(level string) zapcore.Level {

	switch level {
	case "debug":
		return zap.DebugLevel // 返回 zap 的 Debug 级别
	case "info":
		return zap.InfoLevel //返回
	case "warn":
		return zap.WarnLevel //
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

// Sync 同步日志缓冲区到磁盘
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

// GetTraceID 从 context 中提取 Trace ID
func GetTraceID(ctx context.Context) string {
	spn := trace.SpanFromContext(ctx)
	if spn.SpanContext().IsValid() {
		return spn.SpanContext().TraceID().String()
	}
	return ""
}

// 为 zap.Logger 添加 Trace ID 字段
func SetTraceID(ctx context.Context) zap.Field {
	traceID := GetTraceID(ctx)

	if traceID != "" {
		return zap.String("trace_id", traceID) //返回ID
	}
	return zap.Skip() //
}

// 创建带 Trace ID 的上下文日志器
func NewContextLogger(ctx context.Context) *zap.Logger {
	if Logger == nil {
		return nil
	}
	return Logger.With(SetTraceID(ctx))
}

// 创建带 Trace ID 的糖化日志器
func NewContextSugaredLogger(ctx context.Context) *zap.SugaredLogger {
	contextLogger := NewContextLogger(ctx)
	if contextLogger == nil {
		return nil
	}
	return contextLogger.Sugar()
}
