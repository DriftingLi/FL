package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// gormZapLogger 把 GORM 日志路由到统一 zap 日志器，保持级别语义：
// 错误 → Error、慢查询 → Warn、正常 SQL 仅在 Info 模式下输出。
// 相比 gormlogger.New(zap.NewStdLog(...)) 的透传写法，本适配器保证
// 在 LOG_LEVEL=warn/error 时 GORM 错误不被 zap 级别过滤吞掉。
type gormZapLogger struct {
	l    *zap.Logger
	cfg  gormlogger.Config
	mode gormlogger.LogLevel
}

// newGormZapLogger 构造 GORM→zap 适配器，默认 Warn 模式（慢查询/错误）。
func newGormZapLogger(l *zap.Logger) *gormZapLogger {
	return &gormZapLogger{
		l: l,
		cfg: gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		},
		mode: gormlogger.Warn,
	}
}

// LogMode 切换 GORM 日志模式（Warn 模式只输出慢查询与错误）。
func (g *gormZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return &gormZapLogger{l: g.l, cfg: g.cfg, mode: level}
}

// Info Warn Error 输出通用消息（data 为 key/value 对）。
func (g *gormZapLogger) Info(_ context.Context, msg string, data ...any) {
	if g.mode >= gormlogger.Info {
		g.l.Info(msg, toFields(data)...)
	}
}

func (g *gormZapLogger) Warn(_ context.Context, msg string, data ...any) {
	if g.mode >= gormlogger.Warn {
		g.l.Warn(msg, toFields(data)...)
	}
}

func (g *gormZapLogger) Error(_ context.Context, msg string, data ...any) {
	if g.mode >= gormlogger.Error {
		g.l.Error(msg, toFields(data)...)
	}
}

// Trace 输出 SQL 执行信息：错误 → Error；慢查询 → Warn；正常仅在 Info 模式。
func (g *gormZapLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	switch {
	case err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && g.cfg.IgnoreRecordNotFoundError):
		g.l.Error("gorm query error",
			zap.String("sql", sql),
			zap.Error(err),
			zap.Duration("elapsed", elapsed),
		)
	case g.cfg.SlowThreshold > 0 && elapsed > g.cfg.SlowThreshold:
		g.l.Warn("gorm slow query",
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Duration("elapsed", elapsed),
		)
	case g.mode >= gormlogger.Info:
		g.l.Info("gorm query",
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Duration("elapsed", elapsed),
		)
	}
}

// toFields 把 gorm 的 key/value 对转成 zap 字段（奇数个/非法键时降级 Any）。
func toFields(data []any) []zap.Field {
	fields := make([]zap.Field, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		k, ok := data[i].(string)
		if !ok {
			fields = append(fields, zap.Any(fmt.Sprintf("arg%d", i), data[i]))
			continue
		}
		fields = append(fields, zap.Any(k, data[i+1]))
	}
	return fields
}
