// 模拟考试总分存量回填（ADR-0068 决策 4：总分 = Σ 本题得分）。
//
// 用法：DATABASE_URL=... go run ./cmd/backfill-mock-exam-scores
//
// 幂等：重跑第二次 updated=0。只改派生值（mock_exam.score / result.total_score /
// result.details[].score），不重新判分、不动对错与 AI 原始分。
package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"forklift-training/internal/db"
	applogger "forklift-training/internal/logger"
	"forklift-training/internal/mockexam"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "缺少 DATABASE_URL 环境变量")
		os.Exit(1)
	}
	logger, err := applogger.New(applogger.Config{Level: "info", Format: "console"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	gormDB, err := db.InitDB(dsn, logger)
	if err != nil {
		logger.Error("连接数据库失败", zap.Error(err))
		os.Exit(1)
	}
	defer db.Close(gormDB, logger)

	report, err := mockexam.BackfillMockExamTotalScores(gormDB)
	if err != nil {
		logger.Error("回填失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("回填完成",
		zap.Int("scanned", report.Scanned),
		zap.Int("updated", report.Updated),
		zap.Int("no_facts", report.NoFacts))
}
