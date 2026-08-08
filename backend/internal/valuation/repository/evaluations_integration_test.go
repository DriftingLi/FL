// 评估事实性集成测试：真实 Postgres 下新列 SQL 与字段映射。
// CI 提供 postgres:15 服务（DATABASE_URL），本地未配置时跳过。
package repository

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	migratedb "forklift-training/internal/migrate"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL 未配置，跳过集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := migratedb.RunMigrations(dsn, "up", zap.NewNop()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestEvaluationsRepository_LockedSuggestionsMapping 持久化 → 读取往返：
// suggestions/λ 列写入并原样读回（字段映射正确性）。
func TestEvaluationsRepository_LockedSuggestionsMapping(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE evaluations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}

	repo := NewEvaluationRepository(pool)
	params := &CreateEvaluationParams{
		Brand: "集成测试", VehicleType: "电动叉车", Series: "S系列",
		Tonnage: 2.5, ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
		FactoryYear: 2020, SaleYear: 2024, UsageHours: 800,
		OriginalPaint: true, Province: "安徽省", City: "合肥市",
		HasLicensePlate: true, HasRegistrationCertificate: true, HasMaintenanceRecords: true,
		ConditionRating: "A",
		OriginalPrice:   100000, KTime: 0.5, KHours: 1.0, KBrand: 1.1, KCondition: 1.04, KMarket: 1.0,
		EstimatedValue: 52000, ConfidenceLow: 46800, ConfidenceHigh: 57200,
		UserID: 0,
		// 评估时点锁定值
		Suggestions:      []string{"建议一：车况良好", "建议二：保留原厂漆"},
		LambdaElectric:   0.12,
		LambdaCombustion: 0.10,
	}

	id, err := repo.CreateEvaluation(ctx, params)
	if err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	detail, err := repo.GetEvaluation(ctx, id)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(detail.Suggestions) != 2 || detail.Suggestions[0] != "建议一：车况良好" || detail.Suggestions[1] != "建议二：保留原厂漆" {
		t.Errorf("suggestions 映射错误: %v", detail.Suggestions)
	}
	if detail.LambdaElectric != 0.12 || detail.LambdaCombustion != 0.10 {
		t.Errorf("λ 映射错误: electric=%v combustion=%v", detail.LambdaElectric, detail.LambdaCombustion)
	}
	if detail.EstimatedValue != 52000 || detail.KTime != 0.5 {
		t.Errorf("既有字段映射回归: %+v", detail)
	}
}

// TestEvaluationsRepository_BackfillIdempotent 回填 SQL 幂等：
// 已回填记录不再被覆盖，未回填记录可写。
func TestEvaluationsRepository_BackfillIdempotent(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE evaluations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}

	repo := NewEvaluationRepository(pool)
	// 插入两条未回填记录
	for i := 0; i < 2; i++ {
		if _, err := repo.CreateEvaluation(ctx, &CreateEvaluationParams{
			Brand: "回填", VehicleType: "内燃叉车", Series: "R系列",
			Tonnage: 3, ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
			FactoryYear: 2018, SaleYear: 2023, UsageHours: 2000,
			Province: "江苏省", City: "南京市", ConditionRating: "B",
			OriginalPrice: 90000, KTime: 0.6, KHours: 1.2, KBrand: 1.0, KCondition: 0.9, KMarket: 1.0,
			EstimatedValue: 40000, ConfidenceLow: 36000, ConfidenceHigh: 44000,
		}); err != nil {
			t.Fatalf("插入失败: %v", err)
		}
	}

	rows, err := repo.ListEvaluationsForBackfill(ctx)
	if err != nil {
		t.Fatalf("列出回填记录失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("应列出 2 条, got %d", len(rows))
	}
	for _, r := range rows {
		if len(r.Suggestions) != 0 {
			t.Fatalf("新记录 suggestions 应为空, got %v", r.Suggestions)
		}
	}

	if err := repo.UpdateEvaluationSuggestions(ctx, rows[0].ID, []string{"回填建议"}); err != nil {
		t.Fatalf("回填写入失败: %v", err)
	}

	// 幂等：已回填记录带值返回
	rows2, err := repo.ListEvaluationsForBackfill(ctx)
	if err != nil {
		t.Fatalf("二次列出失败: %v", err)
	}
	byID := map[int64]EvaluationBackfillRow{}
	for _, r := range rows2 {
		byID[r.ID] = r
	}
	if got := byID[rows[0].ID].Suggestions; len(got) != 1 || got[0] != "回填建议" {
		t.Errorf("已回填记录应读到锁定建议, got %v", got)
	}
	if len(byID[rows[1].ID].Suggestions) != 0 {
		t.Errorf("未回填记录 suggestions 应为空, got %v", byID[rows[1].ID].Suggestions)
	}
}
