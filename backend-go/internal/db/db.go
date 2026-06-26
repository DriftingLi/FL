// Package db 负责数据库连接初始化与连接池配置。
package db

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 使用 DATABASE_URL 初始化 PostgreSQL 连接，配置连接池。
func InitDB(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL 未配置")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	// 连接池配置（与原 Python 版 SQLAlchemy 池配置等价）
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库 ping 失败: %w", err)
	}

	slog.Info("数据库连接成功")
	return db, nil
}
