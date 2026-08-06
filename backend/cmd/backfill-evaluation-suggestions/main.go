// 评估建议历史回填（ADR-0004 评估事实性）。
//
// 用法：DATABASE_URL=... go run ./cmd/backfill-evaluation-suggestions
// 为缺失建议的存量评估记录用当前系数配置重算并写入（幂等：已有建议的记录跳过）。
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	vconfig "forklift-training/internal/valuation/config"
	vrepo "forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("缺少 DATABASE_URL 环境变量")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := vconfig.NewPostgresPool(ctx, dsn, 5, 5, 1800)
	if err != nil {
		slog.Error("连接数据库失败", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	dictRepo := vrepo.NewDictionaryRepository(pool)
	evalRepo := vrepo.NewEvaluationRepository(pool)

	updated, err := vservice.BackfillEvaluationSuggestions(ctx, dictRepo, evalRepo)
	if err != nil {
		slog.Error("回填失败", "error", err)
		os.Exit(1)
	}
	slog.Info("回填完成", "updated", updated)
}
