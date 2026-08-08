// Package db 负责数据库连接初始化与连接池配置。
package db

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDB 使用 DATABASE_URL 初始化 PostgreSQL 连接，配置连接池。
func InitDB(dsn string, logger *zap.Logger) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL 未配置")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newGormZapLogger(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	// 连接池配置
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库 ping 失败: %w", err)
	}

	logger.Info("数据库连接成功")
	return db, nil
}

// Close 释放 GORM 底层 *sql.DB 连接池。
// 在服务优雅退出阶段调用，避免连接池资源泄漏。
// 传入 nil 时为空操作；关闭失败仅记录日志，不阻断退出流程。
func Close(db *gorm.DB, logger *zap.Logger) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Warn("获取底层 sql.DB 失败，跳过连接池关闭", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		logger.Warn("关闭数据库连接池异常", zap.Error(err))
		return
	}
	logger.Info("数据库连接池已关闭")
}
