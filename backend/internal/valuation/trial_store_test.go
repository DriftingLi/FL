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
	empty, err := reader.LoadTrialSnapshot(ctx, TrialRequest{RequestID: "pg-trial-empty"})
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
	frozen, err := reader.LoadTrialSnapshot(ctx, TrialRequest{RequestID: "pg-trial-policy"})
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

func TestPostgresTrialSnapshotRecalculationUsesFrozenSnapshot(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, base+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `INSERT INTO valuation_trial_policy
		(policy_name, version, brand, vehicle_type, tonnage, max_width_ratio, usage_hour_ratios, condition_ratings, enabled)
		VALUES ('demo', 1, '林德', '电动平衡重式', 1.6, 0.4, '[0,1.7]'::jsonb, '["B"]'::jsonb, true)`); err != nil {
		t.Fatal(err)
	}
	req := TrialRequest{
		RequestID: "pg-trial-first", Policy: "demo", Brand: "林德", VehicleType: "电动平衡重式", Tonnage: 1.6,
		FactoryYear: trialPtr(2021), SaleYear: 2026, Series: trialPtr("E系列"), ConfigType: trialPtr("磷酸铁锂(LFP)"),
		MastType: trialPtr("两级门架"), MastHeightMM: trialPtr(3000), Condition: trialPtr("B"),
		Province: trialPtr("中性"), City: trialPtr("中性"), OriginalPaint: trialPtr(false),
		HasLicensePlate: trialPtr(true), HasRegistrationCertificate: trialPtr(true), HasMaintenanceRecords: trialPtr(false),
	}
	reader := NewPostgresTrialSnapshotReader(pool)
	frozen, err := reader.LoadTrialSnapshot(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	before, err := CalculateTrial(ctx, req, frozen, "development")
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != TrialStatusPriced || before.Interval == nil {
		t.Fatalf("真实迁移基线应可试算: %+v", before)
	}
	var evaluationsBefore int64
	if err := db.Raw(`SELECT COUNT(*) FROM evaluations`).Scan(&evaluationsBefore).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `UPDATE valuation_trial_policy SET max_width_ratio = 0.01`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE original_prices SET original_price = 999999 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	current, err := reader.LoadTrialSnapshot(ctx, TrialRequest{RequestID: "pg-trial-current", Policy: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if current.Hash == frozen.Hash {
		t.Fatal("当前快照必须反映策略和目录变化")
	}
	recalcReq := req
	recalcReq.RequestID = "pg-trial-recalculation"
	recalcReq.SnapshotID = frozen.Hash
	reloaded, err := reader.LoadTrialSnapshot(ctx, recalcReq)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Hash != frozen.Hash || reloaded.TrialPolicies[0].MaxWidthRatio != 0.4 {
		t.Fatalf("旧快照重算不得读取已变化的当前策略: %+v", reloaded.TrialPolicies)
	}
	after, err := CalculateTrial(ctx, recalcReq, reloaded, "development")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != TrialStatusPriced || after.Interval == nil || *after.Interval != *before.Interval ||
		after.RequestID != recalcReq.RequestID || after.SnapshotID != frozen.Hash {
		t.Fatalf("旧快照重算应复现原区间并回显标识: before=%+v after=%+v", before, after)
	}
	idempotent, err := reader.LoadTrialSnapshot(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if idempotent.Hash != frozen.Hash {
		t.Fatal("相同 request_id 的重试必须复用原快照")
	}
	conflictingReq := req
	conflictingReq.UsageHours = trialPtr(100)
	if _, err := reader.LoadTrialSnapshot(ctx, conflictingReq); err == nil {
		t.Fatal("相同 request_id 不得复用于不同请求")
	}
	if _, err := pool.Exec(ctx, `UPDATE valuation_trial_snapshots
		SET snapshot_json = jsonb_set(snapshot_json, '{trial_policies,0,max_width_ratio}', '0.01'::jsonb)
		WHERE snapshot_id = $1`, frozen.Hash); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.LoadTrialSnapshot(ctx, recalcReq); err == nil {
		t.Fatal("快照载荷与标识哈希不一致时必须拒绝")
	}
	if _, err := reader.LoadTrialSnapshot(ctx, TrialRequest{RequestID: "pg-trial-missing", SnapshotID: "missing", Policy: "demo"}); err == nil {
		t.Fatal("不存在的快照标识必须拒绝")
	}
	var evaluationsAfter int64
	if err := db.Raw(`SELECT COUNT(*) FROM evaluations`).Scan(&evaluationsAfter).Error; err != nil {
		t.Fatal(err)
	}
	if evaluationsBefore != evaluationsAfter {
		t.Fatalf("快照持久化与重算不得写 evaluations: before=%d after=%d", evaluationsBefore, evaluationsAfter)
	}
}
