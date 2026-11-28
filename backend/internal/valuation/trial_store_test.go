package valuation

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/testutil"
)

func TestPostgresTrialSnapshotReadOnlyTransactionAndUniquePolicy(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	var schema string
	if err := db.Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	base := os.Getenv("DATABASE_URL")
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, base+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	reader := NewPostgresTrialSnapshotReader(pool)
	empty, err := reader.LoadTrialSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.TrialPolicies) != 0 || len(empty.Brands) == 0 || len(empty.Coefficients) == 0 {
		t.Fatalf("迁移必须保持策略为空并冻结完整目录/系数: policies=%d brands=%d coefficients=%d",
			len(empty.TrialPolicies), len(empty.Brands), len(empty.Coefficients))
	}
	var before int64
	if err := db.Raw(`SELECT COUNT(*) FROM evaluations`).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO valuation_trial_policy
		(policy_name, version, brand, vehicle_type, tonnage, max_width_ratio, usage_hour_ratios, condition_ratings, enabled)
		VALUES ('demo', 1, '林德', '电动叉车', 3, 0.4, '[0,0.85,1.15]'::jsonb, '["B","C"]'::jsonb, true)`)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := reader.LoadTrialSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.TrialPolicies) != 1 || frozen.TrialPolicies[0].MaxWidthRatio != 0.4 || len(frozen.TrialPolicies[0].UsageHourRatios) != 3 {
		t.Fatalf("策略 JSONB 映射失败: %+v", frozen.TrialPolicies)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO valuation_trial_policy
		(policy_name, version, brand, vehicle_type, tonnage, max_width_ratio, usage_hour_ratios, condition_ratings, enabled)
		VALUES ('demo', 2, '林德', '电动叉车', 3, 0.3, '[0]'::jsonb, '["B"]'::jsonb, true)`); err == nil {
		t.Fatal("同策略组合重复版本必须由唯一索引拒绝")
	}
	if _, err := pool.Exec(ctx, `UPDATE valuation_trial_policy SET max_width_ratio = 0.2`); err != nil {
		t.Fatal(err)
	}
	if frozen.TrialPolicies[0].MaxWidthRatio != 0.4 {
		t.Fatal("已加载快照不得随数据库更新漂移")
	}
	var after int64
	if err := db.Raw(`SELECT COUNT(*) FROM evaluations`).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("试算快照读取不得写 evaluations: before=%d after=%d", before, after)
	}
}
