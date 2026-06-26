// Package main 是迁移运行器 CLI 入口。
// 用法: go run ./cmd/migrate [up|down|version]
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
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	if err := migratedb.RunMigrations(dsn, direction); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
	fmt.Printf("迁移 %s 完成\n", direction)
	_ = config.Load // 占位引用
}
