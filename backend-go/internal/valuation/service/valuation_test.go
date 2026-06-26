// Package service 实现核心业务逻辑
// 本文件：主评估公式 ValuationService 的端到端单元测试
// 覆盖多种典型手算案例，对比 Go 输出与人工计算结果
package service

import (
	"context"
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// 浮点比较容差
const eps = 0.01

// floatEq 浮点等值比较（容差 0.01）
func floatEq(a, b float64) bool {
	return math.Abs(a-b) < eps
}

// TestEvaluate_Case01_Electric_5y_FullNormal 案例 1：电动叉车 5 年全正常
// V0=10, t=5 (λ=0.12), h=8000 (Kh=1.0, 8000/8750=0.914), 仓储 (Kw=1.05),
// 丰田 (Kb=1.10), 全 normal (Kc=1.0), Km=1.0
//
// 期望：
//   Kt = e^(-0.6) ≈ 0.5488
//   Kh = 1.00
//   Σ(w·K) = 0.20×1.05 + 0.20×1.10 + 0.50×1.0 + 0.10×1.0 = 0.21+0.22+0.50+0.10 = 1.03
//   V = 10 × 0.5488 × 1.00 × 1.03 ≈ 5.65
func TestEvaluate_Case01_Electric_5y_FullNormal(t *testing.T) {
	svc := buildTestService()
	items := []model.ItemInput{}
	for _, code := range []string{"m1", "m2", "b1"} {
		items = append(items, model.ItemInput{ItemCode: code, Status: model.ItemStatusNormal})
	}
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "丰田",
		OriginalPrice: 10.0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    8000,
		WorkCondition: model.WorkConditionStorage,
		CanDrive:      true,
		HydraulicOk:   true,
		Items:         items,
	}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Kt = e^(-0.12*5) = e^(-0.6) ≈ 0.5488
	if !floatEq(res.KTime, 0.5488) {
		t.Errorf("KTime=%.4f, want 0.5488", res.KTime)
	}
	if !floatEq(res.KHours, 1.00) {
		t.Errorf("KHours=%.4f, want 1.00 (8000h/8750h≈0.91)", res.KHours)
	}
	if !floatEq(res.KWork, 1.05) {
		t.Errorf("KWork=%.4f, want 1.05", res.KWork)
	}
	if !floatEq(res.KBrand, 1.10) {
		t.Errorf("KBrand=%.4f, want 1.10", res.KBrand)
	}
	if !floatEq(res.KCondition, 1.00) {
		t.Errorf("KCondition=%.4f, want 1.00", res.KCondition)
	}
	if !floatEq(res.KMarket, 1.00) {
		t.Errorf("KMarket=%.4f, want 1.00", res.KMarket)
	}

	// V = 10 × 0.5488 × 1.00 × 1.03 = 5.65
	expectedV := 10.0 * 0.5488 * 1.00 * 1.03
	if !floatEq(res.EstimatedValue, round2(expectedV)) {
		t.Errorf("EstimatedValue=%.2f, want %.2f", res.EstimatedValue, round2(expectedV))
	}
}

// TestEvaluate_Case02_Combustion_3y_HalfWear 案例 2：内燃 3 年一半需维修
// V0=15, t=3 (λ=0.10), h=5250 (5250/5250=1.0 落入 [1.0,1.3) → Kh=0.95),
// 港口 (Kw=0.95), 合力 (Kb=0.95),
// e1/e2=need_repair(0.6), bd1=normal(1.0)
//
// Kc_engine = 0.5*0.6 + 0.5*0.6 = 0.6
// Kc_body = 1.0
// Kc = 0.22*0.6 + 0.78*1.0 = 0.132 + 0.78 = 0.912
//
// Σ(w·K) = 0.20×0.95 + 0.20×0.95 + 0.50×0.912 + 0.10×1.0
//        = 0.19 + 0.19 + 0.456 + 0.10 = 0.936
// V = 15 × e^(-0.3) × 0.95 × 0.936 = 15 × 0.7408 × 0.95 × 0.936 ≈ 9.88
func TestEvaluate_Case02_Combustion_3y_HalfWear(t *testing.T) {
	svc := buildTestService()
	items := []model.ItemInput{
		{ItemCode: "e1", Status: model.ItemStatusNeedRepair},
		{ItemCode: "e2", Status: model.ItemStatusNeedRepair},
		{ItemCode: "bd1", Status: model.ItemStatusNormal},
	}
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeCombustion,
		Brand:         "合力",
		OriginalPrice: 15.0,
		PurchaseYear:  2022,
		SaleYear:      2025,
		UsageHours:    5250,
		WorkCondition: model.WorkConditionPort,
		FuelType:      model.FuelTypeDiesel,
		CanDrive:      true,
		HydraulicOk:   true,
		Items:         items,
	}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !floatEq(res.KTime, 0.7408) {
		t.Errorf("KTime=%.4f, want 0.7408", res.KTime)
	}
	if !floatEq(res.KHours, 0.95) {
		t.Errorf("KHours=%.4f, want 0.95 (5250/5250=1.0)", res.KHours)
	}
	if !floatEq(res.KCondition, 0.912) {
		t.Errorf("KCondition=%.4f, want 0.912", res.KCondition)
	}
	// V = 15 × 0.7408 × 0.95 × 0.936
	expectedV := 15.0 * 0.7408 * 0.95 * 0.936
	if !floatEq(res.EstimatedValue, round2(expectedV)) {
		t.Errorf("EstimatedValue=%.2f, want %.2f", res.EstimatedValue, round2(expectedV))
	}
	// 置信区间 ±5%（95% 置信水平），容差 0.02 避免浮点精度差异
	if math.Abs(res.ConfidenceLow-round2(expectedV*0.95)) > 0.02 {
		t.Errorf("ConfidenceLow=%.2f, want %.2f", res.ConfidenceLow, round2(expectedV*0.95))
	}
	if math.Abs(res.ConfidenceHigh-round2(expectedV*1.05)) > 0.02 {
		t.Errorf("ConfidenceHigh=%.2f, want %.2f", res.ConfidenceHigh, round2(expectedV*1.05))
	}
}

// TestEvaluate_Case03_HeavyUse 案例 3：电动 5 年超高强度
// V0=10, t=5, h=15000 (Kh=0.85, 15000/8750=1.71), 工地 (Kw=0.85),
// 比亚迪 (Kb=0.85), 全 normal
//
// Kt = 0.5488
// Σ(w·K) = 0.20×0.85 + 0.20×0.85 + 0.50×1.0 + 0.10×1.0
//        = 0.17 + 0.17 + 0.50 + 0.10 = 0.94
// V = 10 × 0.5488 × 0.85 × 0.94 ≈ 4.38
func TestEvaluate_Case03_HeavyUse(t *testing.T) {
	svc := buildTestService()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "比亚迪",
		OriginalPrice: 10.0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    15000,
		WorkCondition: model.WorkConditionSite,
		CanDrive:      true,
		HydraulicOk:   true,
		Items:         items,
	}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !floatEq(res.KHours, 0.85) {
		t.Errorf("KHours=%.4f, want 0.85 (15000/8750≈1.71)", res.KHours)
	}
	if !floatEq(res.KWork, 0.85) {
		t.Errorf("KWork=%.4f, want 0.85", res.KWork)
	}
	if !floatEq(res.KBrand, 0.85) {
		t.Errorf("KBrand=%.4f, want 0.85", res.KBrand)
	}
	expectedV := 10.0 * 0.5488 * 0.85 * 0.94
	if !floatEq(res.EstimatedValue, round2(expectedV)) {
		t.Errorf("EstimatedValue=%.2f, want %.2f", res.EstimatedValue, round2(expectedV))
	}
}

// TestEvaluate_Case04_CannotDrive 案例 4：不能行驶对结果的影响
// 与案例 1 相同，但 canDrive=false
// Kc 从 1.0 → 0.7
// Σ(w·K) = 0.20×1.05 + 0.20×1.10 + 0.50×0.7 + 0.10×1.0
//        = 0.21 + 0.22 + 0.35 + 0.10 = 0.88
// V = 10 × 0.5488 × 1.0 × 0.88 ≈ 4.83
func TestEvaluate_Case04_CannotDrive(t *testing.T) {
	svc := buildTestService()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "丰田",
		OriginalPrice: 10.0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    8000,
		WorkCondition: model.WorkConditionStorage,
		CanDrive:      false,
		HydraulicOk:   true,
		Items:         items,
	}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(res.KCondition, 0.7) {
		t.Errorf("KCondition=%.4f, want 0.7", res.KCondition)
	}
	expectedV := 10.0 * 0.5488 * 1.0 * 0.88
	if !floatEq(res.EstimatedValue, round2(expectedV)) {
		t.Errorf("EstimatedValue=%.2f, want %.2f", res.EstimatedValue, round2(expectedV))
	}
}

// TestEvaluate_Case05_AllNeedReplace 案例 5：全部需更换
// m1=m2=b1=need_replace(0.3)
// Kc_motor = 0.5*0.3 + 0.5*0.3 = 0.3
// Kc_battery = 0.3
// Kc = 0.20*0.3 + 0.80*0.3 = 0.30
//
// 假设 1 年内燃 + 仓储 + 林德 + Kh=1.0
// Kt = e^(-0.1) ≈ 0.9048
// Σ(w·K) = 0.20×1.05 + 0.20×1.10 + 0.50×0.3 + 0.10×1.0
//        = 0.21 + 0.22 + 0.15 + 0.10 = 0.68
// V = 20 × 0.9048 × 1.0 × 0.68 ≈ 12.31
func TestEvaluate_Case05_AllNeedReplace(t *testing.T) {
	svc := buildTestService()
	items := []model.ItemInput{
		{ItemCode: "e1", Status: model.ItemStatusNeedReplace},
		{ItemCode: "e2", Status: model.ItemStatusNeedReplace},
		{ItemCode: "bd1", Status: model.ItemStatusNeedReplace},
	}
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeCombustion,
		Brand:         "林德",
		OriginalPrice: 20.0,
		PurchaseYear:  2024,
		SaleYear:      2025,
		UsageHours:    1500,
		WorkCondition: model.WorkConditionStorage,
		FuelType:      model.FuelTypeDiesel,
		CanDrive:      true,
		HydraulicOk:   true,
		Items:         items,
	}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(res.KCondition, 0.3) {
		t.Errorf("KCondition=%.4f, want 0.3", res.KCondition)
	}
	expectedV := 20.0 * 0.9048 * 1.0 * 0.68
	if !floatEq(res.EstimatedValue, round2(expectedV)) {
		t.Errorf("EstimatedValue=%.2f, want %.2f", res.EstimatedValue, round2(expectedV))
	}
}

// TestEvaluate_InvalidForkliftType 异常：非法叉车类型
func TestEvaluate_InvalidForkliftType(t *testing.T) {
	svc := buildTestService()
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftType("hybrid"),
		Brand:         "丰田",
		OriginalPrice: 10.0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    5000,
		WorkCondition: model.WorkConditionStorage,
		CanDrive:      true,
		HydraulicOk:   true,
		Items: []model.ItemInput{
			{ItemCode: "m1", Status: model.ItemStatusNormal},
		},
	}
	_, err := svc.Evaluate(context.Background(), req)
	if err != model.ErrInvalidForkliftType {
		t.Errorf("expected ErrInvalidForkliftType, got %v", err)
	}
}

// TestEvaluate_BrandNotFound 异常：品牌未找到
func TestEvaluate_BrandNotFound(t *testing.T) {
	svc := buildTestService()
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "不存在的品牌",
		OriginalPrice: 10.0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    5000,
		WorkCondition: model.WorkConditionStorage,
		CanDrive:      true,
		HydraulicOk:   true,
		Items: []model.ItemInput{
			{ItemCode: "m1", Status: model.ItemStatusNormal},
		},
	}
	_, err := svc.Evaluate(context.Background(), req)
	if err == nil {
		t.Error("expected error for unknown brand, got nil")
	}
}

// TestEvaluate_InvalidPrice 异常：原始价格 ≤ 0
func TestEvaluate_InvalidPrice(t *testing.T) {
	svc := buildTestService()
	req := &model.EvaluationRequest{
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "丰田",
		OriginalPrice: 0,
		PurchaseYear:  2020,
		SaleYear:      2025,
		UsageHours:    5000,
		WorkCondition: model.WorkConditionStorage,
		CanDrive:      true,
		HydraulicOk:   true,
		Items: []model.ItemInput{
			{ItemCode: "m1", Status: model.ItemStatusNormal},
		},
	}
	_, err := svc.Evaluate(context.Background(), req)
	if err != model.ErrInvalidOriginalPrice {
		t.Errorf("expected ErrInvalidOriginalPrice, got %v", err)
	}
}

// buildTestService 构造测试用的完整服务（不依赖 DB）
func buildTestService() *ValuationService {
	return NewValuationService(
		newTestCoefficientLoader(),
		newTestBrandLoader(),
		newTestPartConfigLoader(),
	)
}

// round2 四舍五入到 2 位小数
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
