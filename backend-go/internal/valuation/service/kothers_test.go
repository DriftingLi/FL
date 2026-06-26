// Package service 实现核心业务逻辑
// 本文件：工况 / 品牌 / 市场系数的单元测试
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestCalcKWork_AllConditions 覆盖全部 5 种工况
func TestCalcKWork_AllConditions(t *testing.T) {
	cases := []struct {
		cond model.WorkCondition
		want float64
	}{
		{model.WorkConditionStorage, 1.05},
		{model.WorkConditionPort, 0.95},
		{model.WorkConditionCold, 0.90},
		{model.WorkConditionSite, 0.85},
		{model.WorkConditionOther, 1.00},
	}
	for _, c := range cases {
		t.Run(string(c.cond), func(t *testing.T) {
			got, err := CalcKWork(c.cond)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("Kw for %s = %.4f, want %.4f", c.cond, got, c.want)
			}
		})
	}
}

// TestCalcKWork_Invalid 异常：未识别的工况
func TestCalcKWork_Invalid(t *testing.T) {
	_, err := CalcKWork(model.WorkCondition("未知"))
	if err != model.ErrInvalidWorkCondition {
		t.Errorf("expected ErrInvalidWorkCondition, got %v", err)
	}
}

// TestCalcKBrand_AllTiers 覆盖 4 个品牌档次
func TestCalcKBrand_AllTiers(t *testing.T) {
	loader := newTestBrandLoader()
	cases := []struct {
		brand string
		want  float64
	}{
		{"丰田", 1.10},   // tier1_intl
		{"林德", 1.10},
		{"海斯特", 1.00}, // tier2_intl
		{"合力", 0.95},   // tier1_domestic
		{"比亚迪", 0.85}, // tier2_domestic
	}
	for _, c := range cases {
		t.Run(c.brand, func(t *testing.T) {
			got, err := loader.CalcKBrand(c.brand)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("Kb for %s = %.4f, want %.4f", c.brand, got, c.want)
			}
		})
	}
}

// TestCalcKBrand_NotFound 异常：品牌未注册
func TestCalcKBrand_NotFound(t *testing.T) {
	loader := newTestBrandLoader()
	_, err := loader.CalcKBrand("不存在的品牌")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestCalcKMarket 默认返回 1.00
func TestCalcKMarket(t *testing.T) {
	loader := newTestCoefficientLoader()
	got, err := CalcKMarket(loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-1.00) > 1e-9 {
		t.Errorf("Km = %.4f, want 1.00", got)
	}
}
