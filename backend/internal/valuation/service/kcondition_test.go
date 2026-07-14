// Package service 实现核心业务逻辑
// 本文件：车况系数 Kc 的单元测试
// 注意：Kc 计算依赖 CoefficientProvider（4 个 kc_* 修正项）+ DictionaryRepository（condition_ratings）
// 测试用例需连接真实 PostgreSQL；DB 不可用时自动跳过
package service

import (
	"context"
	"math"
	"testing"

	"forklift-training/internal/valuation/repository"
)

// TestCalcKCondition_AllPresent 全证件齐全 + 原漆 + 维保
// 期望 Kc = base + paint_bonus + maintenance_bonus（无证件扣减，cert_factor=1.0）
// DB 默认值：paint=0.02, maintenance=0.02；评级 A 的 base=1.00
// 期望 KcBase = 1.00 + 0.02 + 0.02 = 1.04，Kc = 1.04 × 1.0 = 1.04
func TestCalcKCondition_AllPresent(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	res, err := CalcKCondition(ctx, "A",
		true, true, true, true, // 原漆 + 维保 + 车牌 + 登记证
		dictRepo, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// KcBase = 1.00 + 0.02 + 0.02 = 1.04，CertFactor = 1.0
	expected := 1.04
	if math.Abs(res.KCondition-expected) > 1e-6 {
		t.Errorf("KCondition = %.4f, want %.4f (KcBase=%.4f, CertFactor=%.4f)",
			res.KCondition, expected, res.KcBase, res.CertFactor)
	}
	if math.Abs(res.CertFactor-1.0) > 1e-9 {
		t.Errorf("CertFactor = %.4f, want 1.0 (无证件扣减)", res.CertFactor)
	}
}

// TestCalcKCondition_MissingLicenseOnly 仅缺车牌
// 期望 cert_factor = 1 - 0.10 = 0.90
// DB 默认值：license_pct=0.10；评级 A 的 base=1.00；无原漆无维保
// 期望 KcBase = 1.00，Kc = 1.00 × 0.90 = 0.90
func TestCalcKCondition_MissingLicenseOnly(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	res, err := CalcKCondition(ctx, "A",
		false, false, false, true, // 无原漆 + 无维保 + 无车牌 + 有登记证
		dictRepo, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 0.90
	if math.Abs(res.KCondition-expected) > 1e-6 {
		t.Errorf("KCondition = %.4f, want %.4f", res.KCondition, expected)
	}
	if math.Abs(res.CertFactor-0.90) > 1e-9 {
		t.Errorf("CertFactor = %.4f, want 0.90", res.CertFactor)
	}
}

// TestCalcKCondition_MissingBothCerts 缺双证（车牌 + 登记证）
// 期望 cert_factor = (1 - 0.10) × (1 - 0.10) = 0.81（乘性复合放大）
// DB 默认值：license_pct=0.10, reg_pct=0.10；评级 A 的 base=1.00；无原漆无维保
// 期望 KcBase = 1.00，Kc = 1.00 × 0.81 = 0.81
func TestCalcKCondition_MissingBothCerts(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	res, err := CalcKCondition(ctx, "A",
		false, false, false, false, // 全缺
		dictRepo, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 0.81
	if math.Abs(res.KCondition-expected) > 1e-6 {
		t.Errorf("KCondition = %.4f, want %.4f (缺双证应复合放大)", res.KCondition, expected)
	}
	if math.Abs(res.CertFactor-0.81) > 1e-9 {
		t.Errorf("CertFactor = %.4f, want 0.81", res.CertFactor)
	}
}

// TestCalcKCondition_ClampedToMin 全缺 + 评级 E（base=0.50）
// KcBase = 0.50 + 0 + 0 = 0.50，CertFactor = 0.81，KcRaw = 0.50 × 0.81 = 0.405
// 未触发下限 0.30，因此 Kc = 0.405
// 此用例验证乘性扣减不会让 Kc 跌穿 0.30（除非 base 极低）
func TestCalcKCondition_ClampedToMin(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	res, err := CalcKCondition(ctx, "E",
		false, false, false, false,
		dictRepo, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 0.50 × 0.81 = 0.405，未触发钳制
	expected := 0.405
	if math.Abs(res.KCondition-expected) > 1e-6 {
		t.Errorf("KCondition = %.4f, want %.4f", res.KCondition, expected)
	}
	if res.KCondition < kcMinClamp {
		t.Errorf("KCondition = %.4f 不应低于下限 %.2f", res.KCondition, kcMinClamp)
	}
}

// TestCalcKCondition_FallbackDefaults provider 查询失败时回退默认值
// 通过传入 nil provider 触发查询失败，应回退到 0.03/0.03/0.05/0.05
// 期望（评级 A，base=1.00，全缺）：
//
//	KcBase = 1.00 + 0 + 0 = 1.00
//	CertFactor = (1-0.05) × (1-0.05) = 0.9025
//	Kc = 1.00 × 0.9025 = 0.9025
func TestCalcKCondition_FallbackDefaults(t *testing.T) {
	// 此用例测试 provider 失败兜底，因此不使用 newTestProvider 返回的 provider，
	// 只借用其创建的 pool 来构造 dictRepo（评级仍需查 condition_ratings）
	_, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	// 传入 nil provider 触发兜底
	res, err := CalcKCondition(ctx, "A",
		false, false, false, false,
		dictRepo, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 兜底默认值：license_pct=0.05, reg_pct=0.05
	// CertFactor = 0.95 × 0.95 = 0.9025
	expected := 0.9025
	if math.Abs(res.KCondition-expected) > 1e-6 {
		t.Errorf("KCondition = %.4f, want %.4f (兜底默认值 0.05/0.05)", res.KCondition, expected)
	}
}

// TestCalcKCondition_UnknownRating 未知评级应报错
func TestCalcKCondition_UnknownRating(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()
	dictRepo := repository.NewDictionaryRepository(pool)

	ctx := context.Background()
	_, err := CalcKCondition(ctx, "X",
		true, true, true, true,
		dictRepo, provider)
	if err == nil {
		t.Errorf("期望未知评级返回错误，得到 nil")
	}
}
