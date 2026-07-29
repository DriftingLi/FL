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
	"log"
	"os"

	"github.com/joho/godotenv"

	"forklift-training/internal/config"
	migratedb "forklift-training/internal/migrate"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL 未配置")
	}

	direction := "up"
	var extraArgs []string
	if len(os.Args) > 1 {
		direction = os.Args[1]
		extraArgs = os.Args[2:]
	}

	if err := migratedb.RunMigrations(dsn, direction, extraArgs...); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
	fmt.Printf("迁移 %s 完成\n", direction)
	_ = config.Load // 占位引用
}
