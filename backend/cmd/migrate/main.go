// Package main 是迁移运行器 CLI 入口。
// 用法:
//
//	migrate up              # 执行所有待应用的迁移
//	migrate down            # 回滚所有迁移（危险操作）
//	migrate force <version> # 强制设置数据库迁移版本并清除 dirty 标志
//	                       # 用于数据库因迁移执行中断进入 dirty 状态后的修复
//	                       # 例: migrate force 14  表示将数据库标记为已应用至 v14 且干净
package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	applogger "forklift-training/internal/logger"
	migratedb "forklift-training/internal/migrate"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL 未配置")
		os.Exit(1)
	}
	logger, err := applogger.New(applogger.Config{Level: "info", Format: "console"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	direction := "up"
	var extraArgs []string
	if len(os.Args) > 1 {
		direction = os.Args[1]
		extraArgs = os.Args[2:]
	}

	if err := migratedb.RunMigrations(dsn, direction, logger, extraArgs...); err != nil {
		logger.Error("迁移失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("迁移完成", zap.String("direction", direction))
}
