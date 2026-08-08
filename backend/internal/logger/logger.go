// Package logger 提供全后端统一的 zap 日志栈：
// 工厂构建（级别/格式/输出）、文件轮转、敏感信息脱敏与访问日志中间件。
// 日志器以构造注入分发，不提供全局访问器。
package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志器配置。
type Config struct {
	// Level 输出级别：debug | info | warn | error
	Level string
	// Format 编码格式：console（人类可读） | json（结构化）
	Format string
	// OutputDir 日志文件目录；为空时仅输出 stdout，非空时双写（stdout + 文件）。
	// 开发环境留空，生产环境指向挂载卷（如 /data/logs）。
	OutputDir string
	// MaxSizeMB 单文件大小上限（MB），触发轮转。
	MaxSizeMB int
	// MaxBackups 保留的旧日志文件份数。
	MaxBackups int
	// MaxAgeDays 旧日志文件最长保留天数。
	MaxAgeDays int
	// Compress 轮转后是否压缩归档。
	Compress bool
}

// New 构建 zap 日志器。
func New(cfg Config) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	encoder, err := buildEncoder(cfg.Format)
	if err != nil {
		return nil, err
	}

	// AtomicLevel 构建，支持运行时调整输出级别（生产事故排查时无需重启）。
	levelAtomic := zap.NewAtomicLevelAt(level)

	var writer zapcore.WriteSyncer
	if cfg.OutputDir == "" {
		writer = zapcore.AddSync(&lumberjackAdapter{})
	} else {
		writer = zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(&lumberjackAdapter{}),
			zapcore.AddSync(newRotatingFile(cfg)),
		)
	}

	core := zapcore.NewCore(encoder, writer, levelAtomic)
	// redactCore 在编码前统一脱敏：敏感 key 打码、错误串内嵌凭证过滤。
	return zap.New(&redactCore{core}, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// redactCore 包装底层 core，对所有经该日志器输出的字段做统一脱敏：
// key 命中敏感黑名单 → 值替换为 ***；Error 类型字段 → 错误串内嵌连接串凭证过滤。
type redactCore struct {
	core zapcore.Core
}

func (c *redactCore) Enabled(l zapcore.Level) bool { return c.core.Enabled(l) }

func (c *redactCore) With(fields []zapcore.Field) zapcore.Core {
	return &redactCore{core: c.core.With(fields)}
}

func (c *redactCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// 标准 core wrapper 模式：不再委托底层 Check，由本层声明 enabled
	// 并挂载自身，确保 Write 走脱敏路径（Enabled 已委托底层判定）。
	return ce.AddCore(e, c)
}

func (c *redactCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	for i, f := range fields {
		if isSensitiveKey(f.Key) {
			fields[i] = zap.String(f.Key, "***")
			continue
		}
		if f.Type == zapcore.ErrorType {
			if err, ok := f.Interface.(error); ok {
				fields[i] = zap.String(f.Key, RedactError(err))
			}
		}
	}
	return c.core.Write(e, fields)
}

func (c *redactCore) Sync() error { return c.core.Sync() }

// lumberjackAdapter 把 os.Stdout 包成 WriteSyncer（无缓冲，便于容器日志即时可见）。
type lumberjackAdapter struct{}

func (*lumberjackAdapter) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (*lumberjackAdapter) Sync() error                 { return nil }

// newRotatingFile 创建按大小轮转、可压缩归档、受保留策略约束的日志文件 writer。
// 默认策略：100MB 轮转、保留 7 份、30 天；压缩默认关闭，由配置显式开启
// （生产环境 config 默认 LOG_COMPRESS=true，CLI 工具无文件输出不受影响）。
func newRotatingFile(cfg Config) *lumberjack.Logger {
	maxSize := cfg.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 7
	}
	maxAge := cfg.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 30
	}
	return &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/app.log", strings.TrimSuffix(cfg.OutputDir, "/")),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   cfg.Compress,
	}
}

func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InvalidLevel, fmt.Errorf("未知的日志级别: %s", level)
	}
}

func buildEncoder(format string) (zapcore.Encoder, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.ConsoleSeparator = " "

	switch strings.ToLower(format) {
	case "json":
		return zapcore.NewJSONEncoder(encoderConfig), nil
	case "console", "":
		return zapcore.NewConsoleEncoder(encoderConfig), nil
	default:
		return nil, fmt.Errorf("未知的日志格式: %s", format)
	}
}
