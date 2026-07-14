// Package service 实现核心业务逻辑
// 本文件：valuation.go 中纯函数的单元测试（不依赖数据库）
// 覆盖 inferPowerType / BuildDimensionScores / roundTo2 / roundTo4 / clamp01
// 这些函数此前无测试覆盖，属于评估结果装配的关键路径
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestInferPowerType 覆盖动力类型推断的各类输入
// 规则：车型名含"内燃"→combustion，其他→electric（电动为仓储车主流派系）
func TestInferPowerType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want model.PowerType
	}{
		{"explicit_electric", "电动叉车", model.PowerTypeElectric},
		{"battery_type", "蓄电池叉车", model.PowerTypeElectric},
		{"empty_string", "", model.PowerTypeElectric},
		{"combustion_explicit", "内燃叉车", model.PowerTypeCombustion},
		{"combustion_balanced", "平衡重内燃叉车", model.PowerTypeCombustion},
		{"combustion_keyword_only", "内燃", model.PowerTypeCombustion},
		{"unknown_defaults_electric", "前移式叉车", model.PowerTypeElectric},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inferPowerType(c.in)
			if got != c.want {
				t.Errorf("inferPowerType(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestClamp01 覆盖钳制函数的边界
func TestClamp01(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.3, 0.3},
		{1, 1},
		{1.5, 1},
		{math.NaN(), 0}, // NaN 兜底为 0（防雷达图非法值）
	}
	for _, c := range cases {
		got := clamp01(c.in)
		if math.IsNaN(c.in) {
			if got != 0 {
				t.Errorf("clamp01(NaN) = %v, want 0", got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("clamp01(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestRoundTo2 覆盖金额精度（2 位小数），避开浮点半边界用例
func TestRoundTo2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{3.14159, 3.14},
		{2.71828, 2.72},
		{1.2349, 1.23},
		{1.2351, 1.24},
		{0, 0},
		{100.005, 100.01}, // math.Round 半远离零
	}
	for _, c := range cases {
		got := roundTo2(c.in)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("roundTo2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestRoundTo4 覆盖系数精度（4 位小数）
func TestRoundTo4(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0.123456, 0.1235},
		{0.987654, 0.9877},
		{0.5, 0.5},
		{1.00001, 1.0},
		{0, 0},
	}
	for _, c := range cases {
		got := roundTo4(c.in)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("roundTo4(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestBuildDimensionScores 覆盖 5 维度评分装配
// 验证：维度数量、标签顺序、值钳制到 [0,1]、4 位小数精度
//
// 注意：函数签名参数顺序为 (kTime, kHours, kBrand, kCondition, kMarket)，
// 但输出标签顺序为 出厂时间/使用强度/品牌价值/市场需求/车辆情况，
// 即 kCondition(车辆情况) 与 kMarket(市场需求) 在签名与输出中位置互换。
// 本测试显式按标签断言值，锁定该映射关系，防止后续重构误改。
func TestBuildDimensionScores(t *testing.T) {
	// 参数顺序：kTime, kHours, kBrand, kCondition, kMarket
	// 注入越界值验证 clamp：kTime=1.5→1, kHours=-0.2→0
	scores := BuildDimensionScores(1.5, -0.2, 0.7, 1.2, 0.95)

	if len(scores) != 5 {
		t.Fatalf("expected 5 dimension scores, got %d", len(scores))
	}

	// 按标签断言（标签顺序与雷达图一致）
	wantByLabel := map[string]float64{
		"出厂时间": 1.0,  // kTime=1.5 clamp→1.0
		"使用强度": 0.0,  // kHours=-0.2 clamp→0.0
		"品牌价值": 0.7,  // kBrand=0.7
		"市场需求": 0.95, // kMarket=0.95（签名第 5 参，输出第 4 位）
		"车辆情况": 1.0,  // kCondition=1.2 clamp→1.0（签名第 4 参，输出第 5 位）
	}
	wantOrder := []string{"出厂时间", "使用强度", "品牌价值", "市场需求", "车辆情况"}
	for i, s := range scores {
		if s.Label != wantOrder[i] {
			t.Errorf("scores[%d].Label = %q, want %q", i, s.Label, wantOrder[i])
		}
		if math.Abs(s.Value-wantByLabel[s.Label]) > 1e-9 {
			t.Errorf("scores[%d] (%s).Value = %v, want %v", i, s.Label, s.Value, wantByLabel[s.Label])
		}
		if s.Value < 0 || s.Value > 1 {
			t.Errorf("dimension %s = %v out of [0,1]", s.Label, s.Value)
		}
	}
}

// TestBuildDimensionScores_Order 验证维度输出顺序与雷达图一致
// 顺序：出厂时间(Kt) / 使用强度(Kh) / 品牌价值(Kb) / 市场需求(Km) / 车辆情况(Kc)
// 注意签名参序与输出序的差异（见 TestBuildDimensionScores 注释）
func TestBuildDimensionScores_Order(t *testing.T) {
	// 参数顺序：kTime, kHours, kBrand, kCondition, kMarket
	scores := BuildDimensionScores(0.11, 0.22, 0.33, 0.44, 0.55)
	// 输出顺序：Kt, Kh, Kb, Km, Kc → 0.11, 0.22, 0.33, 0.55, 0.44
	want := []float64{0.11, 0.22, 0.33, 0.55, 0.44}
	for i, s := range scores {
		if math.Abs(s.Value-want[i]) > 1e-9 {
			t.Errorf("order mismatch at index %d (%s): got %v, want %v", i, s.Label, s.Value, want[i])
		}
	}
}

// TestNewValuationService_NilGuards 验证构造函数的空值校验返回 error（替代旧 panic）
// 这是 #5 错误处理修复的回归测试：确保 nil 依赖不会 panic，而是返回可处理的 error
func TestNewValuationService_NilGuards(t *testing.T) {
	// 三个依赖任一为 nil 都应返回 error，且不 panic
	if _, err := NewValuationService(nil, nil, nil); err == nil {
		t.Error("expected error when pool is nil")
	}
}
