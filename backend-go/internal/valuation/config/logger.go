// Package config 提供 zap 日志器初始化能力
package config

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger 根据配置初始化 zap 日志器
// level: debug | info | warn | error
// format: console（人类可读） | json（结构化）
// output: stdout | 文件路径
func NewLogger(cfg LogConfig) (*zap.Logger, error) {
	// 解析日志级别
	level := zapcore.InfoLevel
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn", "warning":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		return nil, fmt.Errorf("未知的日志级别: %s", cfg.Level)
	}

	// 编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder

	var encoder zapcore.Encoder
	if strings.ToLower(cfg.Format) == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 输出位置
	var writer zapcore.WriteSyncer
	if strings.ToLower(cfg.Output) == "stdout" {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		writer = zapcore.AddSync(f)
	}

	core := zapcore.NewCore(encoder, writer, level)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger, nil
}
