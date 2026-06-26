// Package service 实现核心业务逻辑
// 本文件：使用强度系数 Kh 的单元测试
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestCalcKHours_RangeMapping 覆盖 5 段强度区间的查表结果
// 1750 小时/年基准
//   5 年标准 = 8750 小时
func TestCalcKHours_RangeMapping(t *testing.T) {
	type testCase struct {
		name        string
		usageHours  int
		expectedKh  float64
		desc        string
	}
	cases := []testCase{
		{"low_0.6", 5250, 1.10, "5年×0.6=5250h，比值0.6<0.7"},
		{"normal_0.85", 7438, 1.00, "5年×0.85=7437.5h，比值0.85∈[0.7,1.0)"},
		{"high_1.15", 10063, 0.95, "5年×1.15=10062.5h，比值1.15∈[1.0,1.3)"},
		{"heavy_1.45", 12688, 0.90, "5年×1.45=12687.5h，比值1.45∈[1.3,1.6)"},
		{"super_1.7", 14875, 0.85, "5年×1.7=14875h，比值1.7≥1.6"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := CalcKHours(2020, 2025, c.usageHours)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(res.KHours-c.expectedKh) > 1e-9 {
				t.Errorf("KHours = %.4f, want %.4f (%s)", res.KHours, c.expectedKh, c.desc)
			}
		})
	}
}

// TestCalcKHours_BoundaryValues 测试区间临界点
func TestCalcKHours_BoundaryValues(t *testing.T) {
	// 5年标准 = 8750
	// 0.7 倍 = 6125
	// 1.0 倍 = 8750
	// 1.3 倍 = 11375
	// 1.6 倍 = 14000
	checks := []struct {
		hours    int
		expected float64
	}{
		{6124, 1.10}, // <0.7
		{6125, 1.00}, // =0.7  落入[0.7,1.0)
		{8749, 1.00}, // <1.0
		{8750, 0.95}, // =1.0  落入[1.0,1.3)
		{11374, 0.95},
		{11375, 0.90}, // =1.3
		{13999, 0.90},
		{14000, 0.85}, // =1.6
		{99999, 0.85},
	}
	for _, c := range checks {
		res, err := CalcKHours(2020, 2025, c.hours)
		if err != nil {
			t.Fatalf("hours=%d unexpected error: %v", c.hours, err)
		}
		if math.Abs(res.KHours-c.expected) > 1e-9 {
			t.Errorf("hours=%d ratio=%.4f KHours=%.4f, want %.4f",
				c.hours, res.Ratio, res.KHours, c.expected)
		}
	}
}

// TestCalcKHours_NegativeHours 异常：负数小时
func TestCalcKHours_NegativeHours(t *testing.T) {
	_, err := CalcKHours(2020, 2025, -100)
	if err != model.ErrInvalidUsageHours {
		t.Errorf("expected ErrInvalidUsageHours, got %v", err)
	}
}

// TestCalcKHours_InvalidYear 异常：年份倒置
func TestCalcKHours_InvalidYear(t *testing.T) {
	_, err := CalcKHours(2025, 2020, 1000)
	if err != model.ErrInvalidYear {
		t.Errorf("expected ErrInvalidYear, got %v", err)
	}
}

// TestCalcKHours_SameYear 同年购置与成交，years=0 按 1 年基准
func TestCalcKHours_SameYear(t *testing.T) {
	res, err := CalcKHours(2025, 2025, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 比值 = 1000/1750 ≈ 0.571 → 1.10
	if math.Abs(res.KHours-1.10) > 1e-9 {
		t.Errorf("KHours=%.4f, want 1.10", res.KHours)
	}
}
