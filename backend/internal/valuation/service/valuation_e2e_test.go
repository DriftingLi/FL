// 评估全流程端到端测试：内存替身下 evaluateInternal 的组合行为
// （五维公式组合、残值≤原价钳制、置信区间、原价精确→模糊降级、系数快照单次读取）。
package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// countingDict 包装 DictionaryReader，统计系数读取调用次数（断言快照单次读取）。
type countingDict struct {
	DictionaryReader
	listCalls int
	getCalls  int
}

func (c *countingDict) ListCoefficientConfigs(ctx context.Context) ([]repository.CoefficientConfig, error) {
	c.listCalls++
	return c.DictionaryReader.ListCoefficientConfigs(ctx)
}

func (c *countingDict) GetCoefficientByKey(ctx context.Context, key string) (repository.CoefficientConfig, error) {
	c.getCalls++
	return c.DictionaryReader.GetCoefficientByKey(ctx, key)
}

// failSnapshotDict 快照读取失败（模拟全表查询故障），评估应回退逐 key provider 不失败。
type failSnapshotDict struct {
	DictionaryReader
}

func (f *failSnapshotDict) ListCoefficientConfigs(context.Context) ([]repository.CoefficientConfig, error) {
	return nil, errors.New("模拟全表查询故障")
}

// newFullMemDictReader 带车型/原价/系数的完整版。
type fullMemDict struct {
	memDictReader
	vehicleTypes   map[string]repository.VehicleType
	originalPrices []repository.OriginalPrice
	coefficients   map[string]float64
}

func newFullMemDictReader() *fullMemDict {
	return &fullMemDict{
		memDictReader: *newDefaultMemDict(),
		vehicleTypes: map[string]repository.VehicleType{
			"电动叉车": {ID: 1, Name: "电动叉车", PowerType: "electric", EarliestFactoryYear: 2000},
			"内燃叉车": {ID: 2, Name: "内燃叉车", PowerType: "combustion", EarliestFactoryYear: 2000},
		},
		originalPrices: []repository.OriginalPrice{{
			ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
			ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, OriginalPrice: 100000,
		}},
		coefficients: newDefaultConfigReader().values,
	}
}

func (f *fullMemDict) GetVehicleTypeByName(_ context.Context, name string) (repository.VehicleType, error) {
	if vt, ok := f.vehicleTypes[name]; ok {
		return vt, nil
	}
	return repository.VehicleType{}, pgx.ErrNoRows
}

func (f *fullMemDict) FindOriginalPriceMatch(_ context.Context, brand, vehicleType, series string, tonnage float64, configType, mastType string, mastHeightMM int) (repository.OriginalPrice, error) {
	for _, op := range f.originalPrices {
		if op.Brand == brand && op.VehicleType == vehicleType && op.Series == series &&
			op.Tonnage == tonnage && op.ConfigType == configType && op.MastType == mastType &&
			op.MastHeightMM == mastHeightMM {
			return op, nil
		}
	}
	return repository.OriginalPrice{}, pgx.ErrNoRows
}

// FindOriginalPriceFuzzy 按 brand + vehicle_type + tonnage 匹配（series="其它" 时降级路径）。
func (f *fullMemDict) FindOriginalPriceFuzzy(_ context.Context, brand, vehicleType, _ string, tonnage float64) (repository.OriginalPrice, error) {
	for _, op := range f.originalPrices {
		if op.Brand == brand && op.VehicleType == vehicleType && op.Tonnage == tonnage {
			return op, nil
		}
	}
	return repository.OriginalPrice{}, pgx.ErrNoRows
}

func (f *fullMemDict) GetCoefficientByKey(_ context.Context, key string) (repository.CoefficientConfig, error) {
	if v, ok := f.coefficients[key]; ok {
		return repository.CoefficientConfig{Key: key, Value: v}, nil
	}
	return repository.CoefficientConfig{}, pgx.ErrNoRows
}

func (f *fullMemDict) ListCoefficientConfigs(_ context.Context) ([]repository.CoefficientConfig, error) {
	out := make([]repository.CoefficientConfig, 0, len(f.coefficients))
	for k, v := range f.coefficients {
		out = append(out, repository.CoefficientConfig{Key: k, Value: v})
	}
	return out, nil
}

// memEvalStore 内存评估存储（Persist 消费面）。
type memEvalStore struct {
	nextID int64
}

func (m *memEvalStore) CreateEvaluation(context.Context, *repository.CreateEvaluationParams) (int64, error) {
	m.nextID++
	return m.nextID, nil
}

func newTestValuationService(t *testing.T, dict DictionaryReader) *ValuationService {
	t.Helper()
	svc, err := NewValuationService(dict, &memEvalStore{})
	if err != nil {
		t.Fatalf("构造估值服务失败: %v", err)
	}
	return svc
}

func baseRequest() *model.EvaluationRequest {
	return &model.EvaluationRequest{
		Brand: "林德", VehicleType: "电动叉车", Series: "K系列",
		Tonnage: 3, ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
		FactoryYear: 2019, SaleYear: 2024, UsageHours: 1000,
		Province: "安徽省", City: "合肥市", ConditionRating: "A",
	}
}

// TestEvaluateFullFlow 全流程组合：Kt=e^(-λ·age)、Kt_adj 幂修正、残值公式、置信区间、维度与建议。
func TestEvaluateFullFlow(t *testing.T) {
	dict := newFullMemDictReader()
	svc := newTestValuationService(t, dict)

	req := baseRequest()
	req.OriginalPaint, req.HasMaintenanceRecords = true, true
	req.HasLicensePlate, req.HasRegistrationCertificate = true, true
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("评估失败: %v", err)
	}

	// Kt = e^(-0.12 × 5)（电动 λ=0.12，age=5；KTime 入库取整到 4 位）
	wantKt := math.Exp(-0.12 * 5)
	if math.Abs(res.KTime-wantKt) > 1e-4 {
		t.Errorf("KTime = %v, 期望 ≈ %v", res.KTime, wantKt)
	}
	// 评估时点锁定的 λ 值（ADR-0004）
	if res.LambdaElectric != 0.12 || res.LambdaCombustion != 0.10 {
		t.Errorf("λ 锁定字段错误: electric=%v combustion=%v", res.LambdaElectric, res.LambdaCombustion)
	}
	// Kt_adj = Kt^(Kh/Kb)：Kh=1.10（usage 1000/8750 < low 0.7），Kb=1.0
	// 注意：estimated 用全精度中间值计算，KTimeAdjusted 仅输出时取整到 4 位
	wantKtAdj := math.Pow(wantKt, 1.10)
	if math.Abs(res.KTimeAdjusted-math.Round(wantKtAdj*1e4)/1e4) > 1e-9 {
		t.Errorf("KTimeAdjusted = %v, 期望 ≈ %v", res.KTimeAdjusted, math.Round(wantKtAdj*1e4)/1e4)
	}
	// 残值 = 原价 × Kt_adj × Kc × Km（Kc=1.04：A 基准 1.00 + 原厂漆 0.02 + 维保 0.02；Km=1.0）
	wantEst := math.Round(100000*wantKtAdj*1.04*100) / 100
	if math.Abs(res.EstimatedValue-wantEst) > 0.01 {
		t.Errorf("EstimatedValue = %v, 期望 ≈ %v", res.EstimatedValue, wantEst)
	}
	// 置信区间 = ±10%（confidence_range 快照值）
	if math.Abs(res.ConfidenceLow-res.EstimatedValue*0.9) > 0.01 || math.Abs(res.ConfidenceHigh-res.EstimatedValue*1.1) > 0.01 {
		t.Errorf("置信区间错误: [%v, %v], 期望围绕 %v ±10%%", res.ConfidenceLow, res.ConfidenceHigh, res.EstimatedValue)
	}
	// 维度分 5 项 + 建议非空
	if len(res.DimensionScores) != 5 {
		t.Errorf("维度评分应为 5 项, got %d", len(res.DimensionScores))
	}
	if len(res.Suggestions) == 0 {
		t.Error("建议不应为空")
	}
	// 残值不超过原价（钳制）
	if res.EstimatedValue > res.OriginalPrice {
		t.Errorf("残值 %v 超过原价 %v", res.EstimatedValue, res.OriginalPrice)
	}
}

// TestEvaluateClampToOriginalPrice 车况加成使残值突破原价时钳制到原价（age=0 → Kt_adj=1，Kc=1.04）。
func TestEvaluateClampToOriginalPrice(t *testing.T) {
	dict := newFullMemDictReader()
	svc := newTestValuationService(t, dict)

	req := baseRequest()
	req.FactoryYear, req.SaleYear = 2024, 2024 // age=0
	req.OriginalPaint, req.HasMaintenanceRecords = true, true
	req.HasLicensePlate, req.HasRegistrationCertificate = true, true

	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("评估失败: %v", err)
	}
	if res.EstimatedValue != res.OriginalPrice {
		t.Errorf("残值应被钳制到原价 %v, got %v", res.OriginalPrice, res.EstimatedValue)
	}
	if res.KTimeAdjusted != 1.0 {
		t.Errorf("age=0 时 Kt_adj 应为 1.0, got %v", res.KTimeAdjusted)
	}
}

// TestEvaluateOriginalPriceFuzzyFallback 精确匹配未命中 → 模糊匹配降级（series="其它" 场景）。
func TestEvaluateOriginalPriceFuzzyFallback(t *testing.T) {
	dict := newFullMemDictReader()
	svc := newTestValuationService(t, dict)

	req := baseRequest()
	req.Series = "其它" // 精确匹配必然未命中 → 模糊匹配按 brand+车型+吨位命中

	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("评估失败: %v", err)
	}
	if res.OriginalPrice != 100000 {
		t.Errorf("模糊匹配应命中原价 100000, got %v", res.OriginalPrice)
	}
	if res.EstimatedValue <= 0 {
		t.Error("降级路径残值应为正数")
	}
}

// TestEvaluateCoefficientSnapshot_SingleBulkRead 一次评估只做 1 次全表读取、0 次逐 key 读取。
func TestEvaluateCoefficientSnapshot_SingleBulkRead(t *testing.T) {
	base := newFullMemDictReader()
	counting := &countingDict{DictionaryReader: base}
	svc := newTestValuationService(t, counting)

	if _, err := svc.Evaluate(context.Background(), baseRequest()); err != nil {
		t.Fatalf("评估失败: %v", err)
	}
	if counting.listCalls != 1 {
		t.Errorf("系数全表读取应恰好 1 次, got %d", counting.listCalls)
	}
	if counting.getCalls != 0 {
		t.Errorf("逐 key 读取应为 0 次（快照接管）, got %d", counting.getCalls)
	}
}

// TestEvaluateSnapshotFailureFallback 快照读取失败时回退逐 key provider，评估不中断。
func TestEvaluateSnapshotFailureFallback(t *testing.T) {
	base := newFullMemDictReader()
	failing := &failSnapshotDict{DictionaryReader: base}
	counting := &countingDict{DictionaryReader: failing}
	svc := newTestValuationService(t, counting)

	res, err := svc.Evaluate(context.Background(), baseRequest())
	if err != nil {
		t.Fatalf("快照失败应回退 provider 不中断, got %v", err)
	}
	if res.EstimatedValue <= 0 {
		t.Error("回退路径残值应为正数")
	}
	if counting.getCalls == 0 {
		t.Error("回退路径应发生逐 key 读取")
	}
}

// =====================================================
// 建议回填（ADR-0004）：幂等性
// =====================================================

// memBackfillStore 内存回填存储（与生产仓储同形：返回完整评估详情）。
type memBackfillStore struct {
	rows    []model.EvaluationDetail
	updates map[int64][]string
}

func (m *memBackfillStore) ListEvaluationsForBackfill(context.Context) ([]model.EvaluationDetail, error) {
	out := make([]model.EvaluationDetail, len(m.rows))
	copy(out, m.rows)
	for i := range out {
		if s, ok := m.updates[out[i].ID]; ok {
			out[i].Suggestions = append([]string(nil), s...)
		}
	}
	return out, nil
}

func (m *memBackfillStore) UpdateEvaluationSuggestions(_ context.Context, id int64, s []string) error {
	m.updates[id] = append([]string(nil), s...)
	return nil
}

// TestBackfillEvaluationSuggestions_Idempotent 回填幂等：
// 已有建议的记录跳过；同一批数据跑两遍结果一致。
func TestBackfillEvaluationSuggestions_Idempotent(t *testing.T) {
	dict := newFullMemDictReader()
	store := &memBackfillStore{
		rows: []model.EvaluationDetail{
			{ID: 1, KCondition: 1.0, KHours: 1.0, KBrand: 1.0, KTime: 0.8, KMarket: 1.0, OriginalPrice: 100000, EstimatedValue: 60000},
			{ID: 2, KCondition: 0.9, KHours: 1.1, KBrand: 1.0, KTime: 0.7, KMarket: 1.0, OriginalPrice: 80000, EstimatedValue: 40000, Suggestions: []string{"已锁定"}},
			{ID: 3, KCondition: 1.0, KHours: 1.0, KBrand: 1.0, KTime: 0.8, KMarket: 1.0, OriginalPrice: 100000, EstimatedValue: 60000},
		},
		updates: map[int64][]string{},
	}

	first, err := BackfillEvaluationSuggestions(context.Background(), dict, store)
	if err != nil {
		t.Fatalf("首次回填失败: %v", err)
	}
	if first != 2 {
		t.Errorf("首次回填应更新 2 条（id=2 已有建议跳过）, got %d", first)
	}
	if _, ok := store.updates[2]; ok {
		t.Error("已有建议的记录不应被回填")
	}

	second, err := BackfillEvaluationSuggestions(context.Background(), dict, store)
	if err != nil {
		t.Fatalf("二次回填失败: %v", err)
	}
	if second != 0 {
		t.Errorf("二次回填应为 0（全部已锁定）, got %d", second)
	}
	for _, id := range []int64{1, 3} {
		if len(store.updates[id]) == 0 {
			t.Errorf("id=%d 应已回填建议", id)
		}
	}
}

// TestBackfillEvaluationSuggestions_SnapshotFailure 快照读取失败时回填报错（与评估流程的容错不同：
// 回填是离线运维命令，宁可失败暴露问题也不静默跳过）。
func TestBackfillEvaluationSuggestions_SnapshotFailure(t *testing.T) {
	dict := &failSnapshotDict{DictionaryReader: newFullMemDictReader()}
	store := &memBackfillStore{rows: []model.EvaluationDetail{
		{ID: 1, KCondition: 1.0, KHours: 1.0, KBrand: 1.0, KTime: 0.8, KMarket: 1.0, OriginalPrice: 100000, EstimatedValue: 60000},
	}, updates: map[int64][]string{}}

	if _, err := BackfillEvaluationSuggestions(context.Background(), dict, store); err == nil {
		t.Fatal("快照读取失败时应返回错误")
	}
}
