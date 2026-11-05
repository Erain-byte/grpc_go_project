package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"web3/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Logger 是 Web3 进程共享的结构化日志器。业务对象仍通过构造函数显式接收它，
	// 避免业务代码依赖全局变量；这里的全局引用仅用于应用生命周期管理。
	Logger        *zap.Logger
	SugaredLogger *zap.SugaredLogger
)

// InitializeLogger 创建同时写入控制台和滚动文件的 Zap 日志器。
func InitializeLogger(cfg config.LoggerConfig) error {
	log, err := New(cfg)
	if err != nil {
		return err
	}
	Logger = log
	SugaredLogger = log.Sugar()
	return nil
}

// New 根据配置创建独立日志器，便于测试及后续按组件派生 Named Logger。
func New(cfg config.LoggerConfig) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format != "json" && format != "console" {
		return nil, fmt.Errorf("unsupported Web3 log format %q", cfg.Format)
	}
	if cfg.MaxSize <= 0 || cfg.MaxBackups < 0 || cfg.MaxAge < 0 {
		return nil, errors.New("Web3 log rotation configuration is invalid")
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	var encoder zapcore.Encoder
	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level),
	}
	filename := strings.TrimSpace(cfg.Filename)
	if filename != "" {
		directory := filepath.Dir(filename)
		if directory != "." {
			if err := os.MkdirAll(directory, 0o755); err != nil {
				return nil, fmt.Errorf("create Web3 log directory: %w", err)
			}
		}
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		})
		cores = append(cores, zapcore.NewCore(encoder, fileWriter, level))
	}

	return zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
	), nil
}

func parseLevel(value string) (zapcore.Level, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(value)))); err != nil {
		return zap.InfoLevel, fmt.Errorf("invalid Web3 log level %q: %w", value, err)
	}
	return level, nil
}

// Sync 在进程退出前刷新 Zap 缓冲区。
func Sync() error {
	if Logger == nil {
		return nil
	}
	return Logger.Sync()
}
