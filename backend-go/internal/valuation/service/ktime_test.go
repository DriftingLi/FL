// Package service 实现核心业务逻辑
// 本文件：时间衰减系数 Kt 的单元测试
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestCalcKTime_BoundaryCases 覆盖 Kt 各典型年限与两种叉车类型
// 期望值与开发方案表一致：
//   电动 λ=0.12：t=1→0.887 / t=3→0.698 / t=5→0.549 / t=8→0.382 / t=10→0.301
//   内燃 λ=0.10：t=1→0.905 / t=3→0.741 / t=5→0.607 / t=8→0.449 / t=10→0.368
func TestCalcKTime_BoundaryCases(t *testing.T) {
	loader := newTestCoefficientLoader()

	type testCase struct {
		name     string
		ft       model.ForkliftType
		years    int
		expected float64
	}
	cases := []testCase{
		// 电动
		{"electric_1y", model.ForkliftTypeElectric, 1, 0.887},
		{"electric_3y", model.ForkliftTypeElectric, 3, 0.698},
		{"electric_5y", model.ForkliftTypeElectric, 5, 0.549},
		{"electric_8y", model.ForkliftTypeElectric, 8, 0.382},
		{"electric_10y", model.ForkliftTypeElectric, 10, 0.301},
		// 内燃
		{"combustion_1y", model.ForkliftTypeCombustion, 1, 0.905},
		{"combustion_3y", model.ForkliftTypeCombustion, 3, 0.741},
		{"combustion_5y", model.ForkliftTypeCombustion, 5, 0.607},
		{"combustion_8y", model.ForkliftTypeCombustion, 8, 0.449},
		{"combustion_10y", model.ForkliftTypeCombustion, 10, 0.368},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := CalcKTime(c.ft, 2025-c.years, 2025, loader)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(res.KTime-c.expected) > 0.005 {
				t.Errorf("KTime = %.4f, want ≈ %.4f", res.KTime, c.expected)
			}
		})
	}
}

// TestCalcKTime_ZeroYear 边界：t=0 时 Kt=1
func TestCalcKTime_ZeroYear(t *testing.T) {
	loader := newTestCoefficientLoader()
	res, err := CalcKTime(model.ForkliftTypeElectric, 2025, 2025, loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KTime-1.0) > 1e-9 {
		t.Errorf("KTime at t=0 = %.6f, want 1.0", res.KTime)
	}
	if res.Years != 0 {
		t.Errorf("Years = %d, want 0", res.Years)
	}
}

// TestCalcKTime_NegativeYear 异常：成交年份早于购置年份
func TestCalcKTime_NegativeYear(t *testing.T) {
	loader := newTestCoefficientLoader()
	_, err := CalcKTime(model.ForkliftTypeElectric, 2025, 2024, loader)
	if err != model.ErrInvalidYear {
		t.Errorf("expected ErrInvalidYear, got %v", err)
	}
}

// TestCalcKTime_InvalidForkliftType 异常：叉车类型非法
func TestCalcKTime_InvalidForkliftType(t *testing.T) {
	loader := newTestCoefficientLoader()
	_, err := CalcKTime(model.ForkliftType("hybrid"), 2020, 2025, loader)
	if err != model.ErrInvalidForkliftType {
		t.Errorf("expected ErrInvalidForkliftType, got %v", err)
	}
}
