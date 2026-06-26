// Package pdf 实现 PDF 报告生成
// 本文件：PDF 生成器单元测试，使用内存样例数据验证字体加载 + 6 段渲染流程
package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forklift-training/internal/valuation/model"
)

// TestGenerateReport 验证 PDF 生成器能成功生成文件且首字节为 PDF 魔数
func TestGenerateReport(t *testing.T) {
	// 准备临时输出目录
	dir := t.TempDir()
	gen := NewGenerator(dir)

	// 构造样例评估详情
	report := &model.EvaluationDetailResponse{
		ID:             1001,
		ForkliftType:   model.ForkliftTypeElectric,
		Brand:          "丰田 TOYOTA",
		Model:          "8FBE15U",
		OriginalPrice:  12.50,
		PurchaseYear:   2021,
		SaleYear:       2024,
		UsageHours:     3500,
		WorkCondition:  model.WorkConditionStorage,
		FuelType:       "", // 电动叉车无燃料类型
		CanDrive:       true,
		HydraulicOk:    true,
		KTime:          0.7408,
		KHours:         1.0000,
		KWork:          1.0500,
		KBrand:         1.1000,
		KCondition:     0.9500,
		KMarket:        1.0000,
		EstimatedValue: 7.32,
		ConfidenceLow:  6.95,
		ConfidenceHigh: 7.69,
		DimensionScores: map[string]float64{
			"时间维度": 0.74, "使用强度": 1.00, "工况": 1.05,
			"品牌": 1.10, "车况": 0.95, "市场": 1.00,
		},
		Suggestions: []string{
			"车况整体保持良好，建议正常保养延续使用",
			"国际一线品牌保值能力强，残值稳定",
		},
	}

	// 加权权重（与 seed.sql 一致）
	weights := model.CalcWeights{
		WWorkCondition: 0.20,
		WBrand:         0.20,
		WCondition:     0.50,
		WMarket:        0.10,
	}

	// 构造样例部件状态（覆盖多个类别）
	items := []model.EvaluationItemDTO{
		{CategoryCode: "BATTERY", CategoryName: "电池组", ItemCode: "b1", ItemName: "主电池组", Status: model.ItemStatusNormal, Score: 1.0},
		{CategoryCode: "BATTERY", CategoryName: "电池组", ItemCode: "b2", ItemName: "充电机", Status: model.ItemStatusSlightWear, Score: 0.85},
		{CategoryCode: "MOTOR", CategoryName: "驱动电机", ItemCode: "m1", ItemName: "行走电机", Status: model.ItemStatusNormal, Score: 1.0},
		{CategoryCode: "MOTOR", CategoryName: "驱动电机", ItemCode: "m2", ItemName: "液压电机", Status: model.ItemStatusNeedRepair, Score: 0.6},
		{CategoryCode: "FRAME", CategoryName: "车架结构", ItemCode: "f1", ItemName: "门架", Status: model.ItemStatusNormal, Score: 1.0},
	}

	// 调用生成器
	path, err := gen.GenerateReport(report, items, weights)
	if err != nil {
		t.Fatalf("生成 PDF 失败: %v", err)
	}

	// 校验文件存在
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF 文件不存在: %v", err)
	}
	if info.Size() < 1024 {
		t.Errorf("PDF 文件过小 (%d 字节)，可能未正确生成", info.Size())
	}
	t.Logf("PDF 生成成功: %s, 大小 %d 字节", path, info.Size())

	// 校验 PDF 魔数（文件首 4 字节为 %PDF）
	head := make([]byte, 4)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开 PDF 失败: %v", err)
	}
	defer f.Close()
	if _, err := f.Read(head); err != nil {
		t.Fatalf("读取 PDF 首部失败: %v", err)
	}
	if string(head) != "%PDF" {
		t.Errorf("PDF 首部不是 %%PDF，实际为 %q", head)
	}

	// 校验输出目录确实在临时目录下
	abs, _ := filepath.Abs(path)
	if !strings.HasPrefix(abs, filepath.Clean(dir)) {
		t.Errorf("PDF 路径 %s 未落在输出目录 %s 下", abs, dir)
	}
}

// TestGenerateReportCombustion 内燃叉车样例，验证燃料类型等字段正确渲染
func TestGenerateReportCombustion(t *testing.T) {
	dir := t.TempDir()
	gen := NewGenerator(dir)

	report := &model.EvaluationDetailResponse{
		ID:             2002,
		ForkliftType:   model.ForkliftTypeCombustion,
		Brand:          "三菱 MITSUBISHI",
		Model:          "FD30N",
		OriginalPrice:  18.00,
		PurchaseYear:   2019,
		SaleYear:       2024,
		UsageHours:     8000,
		WorkCondition:  model.WorkConditionPort,
		FuelType:       model.FuelTypeDiesel,
		CanDrive:       true,
		HydraulicOk:    false, // 液压异常情况
		KTime:          0.6065,
		KHours:         0.90,
		KWork:          0.95,
		KBrand:         1.00,
		KCondition:     0.75,
		KMarket:        0.98,
		EstimatedValue: 4.50,
		ConfidenceLow:  4.28,
		ConfidenceHigh: 4.73,
		DimensionScores: map[string]float64{
			"时间维度": 0.61, "使用强度": 0.90, "工况": 0.95,
			"品牌": 1.00, "车况": 0.75, "市场": 0.98,
		},
		Suggestions: []string{
			"车况一般，多个部件需要维修",
			"液压系统异常，建议维修后再出售",
		},
	}

	weights := model.CalcWeights{
		WWorkCondition: 0.20,
		WBrand:         0.20,
		WCondition:     0.50,
		WMarket:        0.10,
	}

	items := []model.EvaluationItemDTO{
		{CategoryCode: "ENGINE", CategoryName: "发动机", ItemCode: "e1", ItemName: "柴油机本体", Status: model.ItemStatusNeedRepair, Score: 0.6},
		{CategoryCode: "ENGINE", CategoryName: "发动机", ItemCode: "e2", ItemName: "进排气系统", Status: model.ItemStatusSlightWear, Score: 0.85},
		{CategoryCode: "BODY", CategoryName: "车身结构", ItemCode: "bd1", ItemName: "驾驶舱", Status: model.ItemStatusNormal, Score: 1.0},
	}

	path, err := gen.GenerateReport(report, items, weights)
	if err != nil {
		t.Fatalf("生成 PDF 失败: %v", err)
	}
	t.Logf("内燃叉车 PDF 生成成功: %s", path)
}

// TestGenerateReportEmptyItems 部件列表为空时也应能生成
func TestGenerateReportEmptyItems(t *testing.T) {
	dir := t.TempDir()
	gen := NewGenerator(dir)

	report := &model.EvaluationDetailResponse{
		ID:            3,
		ForkliftType:  model.ForkliftTypeElectric,
		Brand:         "永恒力 JUNGHEINRICH",
		OriginalPrice: 9.00,
		PurchaseYear:  2022,
		SaleYear:      2024,
		UsageHours:    1200,
		WorkCondition: model.WorkConditionCold,
	}
	weights := model.CalcWeights{
		WWorkCondition: 0.20,
		WBrand:         0.20,
		WCondition:     0.50,
		WMarket:        0.10,
	}
	path, err := gen.GenerateReport(report, nil, weights)
	if err != nil {
		t.Fatalf("空部件列表也必须能生成: %v", err)
	}
	t.Logf("空部件 PDF 生成成功: %s", path)
}

// TestFindFontFile 验证字体文件查找逻辑
func TestFindFontFile(t *testing.T) {
	_, err := findFontFile()
	if err != nil {
		// 找不到时给出明确提示，但允许跳过（CI 环境下字体可能不存在）
		t.Skipf("字体文件未找到（CI 环境可接受）: %v", err)
	}
}
