// 评估缓存失效集成测试：报告/建议回写后 per-user 详情立即见新值（#399 stale 回归）。
// 真实 Postgres + Redis 下运行（CI 服务容器提供），本地不可用时跳过。
package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
)

// evalCacheTestPrefix 缓存集成测试专用 key 前缀，避免污染业务 key。
const evalCacheTestPrefix = "fl:test-eval:"

// setupCacheTestRedis 连接测试 Redis（REDIS_ADDR 未配置时回退 localhost，连不上则跳过）。
func setupCacheTestRedis(t *testing.T) {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	cfg := config.RedisConfig{
		Addr:         addr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     5,
		MinIdleConns: 1,
		MaxRetries:   3,
		Prefix:       evalCacheTestPrefix,
		DialTimeout:  3 * time.Second, ReadTimeout: 2 * time.Second,
		WriteTimeout: 2 * time.Second, PoolTimeout: 3 * time.Second,
		IdleTimeout: 2 * time.Minute,
	}
	if _, err := cache.InitRedis(cfg, zap.NewNop()); err != nil {
		t.Skipf("Redis 不可用，跳过缓存集成测试: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cache.InvalidatePattern(ctx, "eval:*")
		cache.CloseRedis(cache.GetClient(), zap.NewNop())
	})
}

// cacheRegressionParams 一条归属用户 42 的评估记录（报告路径与建议均为空）。
func cacheRegressionParams() *CreateEvaluationParams {
	return &CreateEvaluationParams{
		Brand: "缓存回归", VehicleType: "电动叉车", Series: "S系列",
		Tonnage: 2.5, ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
		FactoryYear: 2020, SaleYear: 2024, UsageHours: 800,
		Province: "安徽省", City: "合肥市", ConditionRating: "A",
		OriginalPrice: 100000, KTime: 0.5, KHours: 1.0, KBrand: 1.1, KCondition: 1.04, KMarket: 1.0,
		EstimatedValue: 52000, ConfidenceLow: 46800, ConfidenceHigh: 57200,
		UserID: 42,
	}
}

// TestEvaluationsCache_CreateInvalidatesListAndCount 新建记录后列表/统计缓存立即失效
// （失效集缺 list/count pattern 时，预热过的列表会漏新条目）。
func TestEvaluationsCache_CreateInvalidatesListAndCount(t *testing.T) {
	pool := integrationPool(t)
	setupCacheTestRedis(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE evaluations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}
	if err := cache.InvalidatePattern(ctx, "eval:*"); err != nil {
		t.Fatalf("清场失败: %v", err)
	}

	repo := NewEvaluationRepository(pool)
	if _, err := repo.CreateEvaluation(ctx, cacheRegressionParams()); err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	// 预热列表与统计（此时 1 条）
	list, err := repo.ListEvaluations(ctx, "", 42, 10, 0)
	if err != nil {
		t.Fatalf("预热列表失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("预热列表应 1 条, got %d", len(list))
	}
	total, err := repo.CountEvaluations(ctx, "", 42)
	if err != nil {
		t.Fatalf("预热统计失败: %v", err)
	}
	if total != 1 {
		t.Fatalf("预热统计应 1, got %d", total)
	}

	// 新建第 2 条 → 列表/统计必须立见新值
	if _, err := repo.CreateEvaluation(ctx, cacheRegressionParams()); err != nil {
		t.Fatalf("二次插入失败: %v", err)
	}
	list2, err := repo.ListEvaluations(ctx, "", 42, 10, 0)
	if err != nil {
		t.Fatalf("二次列表失败: %v", err)
	}
	if len(list2) != 2 {
		t.Errorf("新建后列表应立见 2 条, got %d", len(list2))
	}
	total2, err := repo.CountEvaluations(ctx, "", 42)
	if err != nil {
		t.Fatalf("二次统计失败: %v", err)
	}
	if total2 != 2 {
		t.Errorf("新建后统计应立见 2, got %d", total2)
	}
}

// TestEvaluationsCache_ReportWriteInvalidatesPerUserDetail 报告回写/建议回填后，公开与
// per-user 两种详情读 key 形状都必须立即失效（修复前 per-user key 从不失效，
// 学生端详情 stale 最长 10 分钟——按钮态/建议回填错误）。
func TestEvaluationsCache_ReportWriteInvalidatesPerUserDetail(t *testing.T) {
	pool := integrationPool(t)
	setupCacheTestRedis(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE evaluations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}
	if err := cache.InvalidatePattern(ctx, "eval:*"); err != nil {
		t.Fatalf("清场失败: %v", err)
	}

	repo := NewEvaluationRepository(pool)
	id, err := repo.CreateEvaluation(ctx, cacheRegressionParams())
	if err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	// 预热两种详情读 key（旧值：report_pdf_path 为空、建议为空）
	if _, err := repo.GetEvaluation(ctx, id); err != nil {
		t.Fatalf("预热公开详情失败: %v", err)
	}
	if _, err := repo.GetEvaluationByUser(ctx, id, 42); err != nil {
		t.Fatalf("预热 per-user 详情失败: %v", err)
	}

	// 报告回写 → 两种形状的详情缓存必须立即失效
	const pdfPath = "/reports/eval-cache-regression.pdf"
	if err := repo.UpdateEvaluationReportPath(ctx, id, pdfPath); err != nil {
		t.Fatalf("报告回写失败: %v", err)
	}
	d, err := repo.GetEvaluationByUser(ctx, id, 42)
	if err != nil {
		t.Fatalf("读取 per-user 详情失败: %v", err)
	}
	if d.ReportPdfPath != pdfPath {
		t.Errorf("报告回写后 per-user 详情仍 stale: got %q, want %q", d.ReportPdfPath, pdfPath)
	}
	dp, err := repo.GetEvaluation(ctx, id)
	if err != nil {
		t.Fatalf("读取公开详情失败: %v", err)
	}
	if dp.ReportPdfPath != pdfPath {
		t.Errorf("报告回写后公开详情仍 stale: got %q, want %q", dp.ReportPdfPath, pdfPath)
	}

	// 建议回填 → per-user 详情同样立即失效
	if err := repo.UpdateEvaluationSuggestions(ctx, id, []string{"缓存回归建议"}); err != nil {
		t.Fatalf("建议回填失败: %v", err)
	}
	d2, err := repo.GetEvaluationByUser(ctx, id, 42)
	if err != nil {
		t.Fatalf("回填后读取 per-user 详情失败: %v", err)
	}
	if len(d2.Suggestions) != 1 || d2.Suggestions[0] != "缓存回归建议" {
		t.Errorf("建议回填后 per-user 详情仍 stale: got %v", d2.Suggestions)
	}
}
