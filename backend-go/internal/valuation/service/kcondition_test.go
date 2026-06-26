// Package service 实现核心业务逻辑
// 本文件：车况系数 Kc 的单元测试（两级加权模型）
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestCalcKCondition_AllNormal 全正常 → Kc = 1.0
func TestCalcKCondition_AllNormal(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KCondition-1.0) > 1e-9 {
		t.Errorf("Kc = %.4f, want 1.0", res.KCondition)
	}
}

// TestCalcKCondition_AllSlightWear 全轻微磨损 → Kc = 0.85
// 验证：所有条目得 0.85，权重归一化后类别和仍为 0.85
func TestCalcKCondition_AllSlightWear(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusSlightWear},
		{ItemCode: "m2", Status: model.ItemStatusSlightWear},
		{ItemCode: "b1", Status: model.ItemStatusSlightWear},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KCondition-0.85) > 1e-9 {
		t.Errorf("Kc = %.4f, want 0.85", res.KCondition)
	}
}

// TestCalcKCondition_HalfWorn 一半条目需维修（0.6），一半轻微磨损（0.85）
// 类别 motor 权重 wc=0.20，battery wc=0.80
// Kc_motor = 0.5*0.85 + 0.5*0.6 = 0.725
// Kc_battery = 1.0*0.6 = 0.6
// Kc = 0.20*0.725 + 0.80*0.6 = 0.145 + 0.48 = 0.625
func TestCalcKCondition_HalfWorn(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusSlightWear},
		{ItemCode: "m2", Status: model.ItemStatusNeedRepair},
		{ItemCode: "b1", Status: model.ItemStatusNeedRepair},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 0.20*0.725 + 0.80*0.6
	if math.Abs(res.KCondition-expected) > 1e-9 {
		t.Errorf("Kc = %.4f, want %.4f", res.KCondition, expected)
	}
}

// TestCalcKCondition_CanDriveOff 验证"不能行驶"对 Kc 的折损
// 全正常 Kc=1.0，不能行驶再 ×0.7 = 0.7
func TestCalcKCondition_CanDriveOff(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KCondition-0.7) > 1e-9 {
		t.Errorf("Kc with canDrive=false = %.4f, want 0.7", res.KCondition)
	}
}

// TestCalcKCondition_HydraulicOff 验证"液压故障"对 Kc 的折损
// 全正常 Kc=1.0，液压故障再 ×0.8 = 0.8
func TestCalcKCondition_HydraulicOff(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KCondition-0.8) > 1e-9 {
		t.Errorf("Kc with hydraulic=false = %.4f, want 0.8", res.KCondition)
	}
}

// TestCalcKCondition_BothOff 验证两个开关同时关闭的叠加折损
// 1.0 × 0.7 × 0.8 = 0.56
func TestCalcKCondition_BothOff(t *testing.T) {
	loader := newTestPartConfigLoader()
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNormal},
		{ItemCode: "m2", Status: model.ItemStatusNormal},
		{ItemCode: "b1", Status: model.ItemStatusNormal},
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KCondition-0.56) > 1e-9 {
		t.Errorf("Kc with both off = %.4f, want 0.56", res.KCondition)
	}
}

// TestCalcKCondition_EmptyItems 异常：部件状态列表为空
func TestCalcKCondition_EmptyItems(t *testing.T) {
	loader := newTestPartConfigLoader()
	_, err := CalcKCondition(model.ForkliftTypeElectric, nil, loader, true, true)
	if err != model.ErrItemsEmpty {
		t.Errorf("expected ErrItemsEmpty, got %v", err)
	}
}

// TestCalcKCondition_MissingConfig 异常：部件配置缺失
func TestCalcKCondition_MissingConfig(t *testing.T) {
	loader := &PartConfigLoader{cache: map[model.ForkliftType][]model.PartConfigInfo{}}
	items := []model.ItemInput{{ItemCode: "x", Status: model.ItemStatusNormal}}
	_, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, true)
	if err != model.ErrPartConfigMissing {
		t.Errorf("expected ErrPartConfigMissing, got %v", err)
	}
}

// TestCalcKCondition_PartialItems 用户只提交部分条目时，未提交的视为"正常"
func TestCalcKCondition_PartialItems(t *testing.T) {
	loader := newTestPartConfigLoader()
	// 只提交 m1 = need_replace
	items := []model.ItemInput{
		{ItemCode: "m1", Status: model.ItemStatusNeedReplace}, // 0.3
	}
	res, err := CalcKCondition(model.ForkliftTypeElectric, items, loader, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// m2 默认 normal=1.0
	// Kc_motor = 0.5*0.3 + 0.5*1.0 = 0.65
	// Kc_battery = 1.0（未提交，默认 normal）
	// Kc = 0.20*0.65 + 0.80*1.0 = 0.13 + 0.80 = 0.93
	expected := 0.20*0.65 + 0.80*1.0
	if math.Abs(res.KCondition-expected) > 1e-9 {
		t.Errorf("Kc = %.4f, want %.4f", res.KCondition, expected)
	}
}
