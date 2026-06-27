// Package pdf 实现 PDF 报告生成
// 本文件:PDF 生成器单元测试,使用内存样例数据验证字体加载 + 3 页简洁版渲染流程
package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forklift-training/internal/valuation/model"
)

// 计数 PDF 中的页面对象 /Type /Page (排除 /Type /Pages 父节点)
func countPDFPages(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(`/Type /Page(?:[^s]|$)`)
	return len(re.FindAll(data, -1)), nil
}

// extractPDFText 解压 PDF 内 FlateDecode 后的文本流拼接
func extractPDFText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// 寻找 "stream\r?\n ... \r?\nendstream"
	streamRe := regexp.MustCompile(`(?s)stream\r?\n(.+?)\r?\nendstream`)
	matches := streamRe.FindAllSubmatch(data, -1)
	var buf bytes.Buffer
	for _, m := range matches {
		compressed := m[1]
		zr, err := zlib.NewReader(bytes.NewReader(compressed))
		if err != nil {
			// 尝试不解压直接保留
			buf.Write(compressed)
			continue
		}
		raw, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			continue
		}
		buf.Write(raw)
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

// containAny 报告中至少出现 expected 中的一项(用于容忍字体编码差异)
func containAny(haystack string, expected ...string) bool {
	for _, s := range expected {
		if strings.Contains(haystack, s) {
			return true
		}
	}
	return false
}

// TestGenerateReport 验证 PDF 生成器能成功生成文件、首字节为 PDF 魔数,且为 3 页
func TestGenerateReport(t *testing.T) {
	// 准备临时输出目录
	dir := t.TempDir()
	gen := NewGenerator(dir)

	// 构造样例评估详情(贴近设计稿示例:合力 H2000, 28.5 万, 6 年, 6800 小时, 12.35 万残值, B 级)
	report := &model.EvaluationDetailResponse{
		ID:             1,
		ForkliftType:   model.ForkliftTypeElectric,
		Brand:          "合力 (HELI)",
		Model:          "H2000",
		OriginalPrice:  28.50,
		PurchaseYear:   2020,
		SaleYear:       2026,
		UsageHours:     6800,
		WorkCondition:  model.WorkConditionStorage,
		CanDrive:       true,
		HydraulicOk:    true,
		KTime:          0.74,
		KHours:         0.68,
		KWork:          1.10,
		KBrand:         1.05,
		KCondition:     0.82,
		KMarket:        0.95,
		EstimatedValue: 12.35,
		ConfidenceLow:  10.82,
		ConfidenceHigh: 13.88,
		DimensionScores: map[string]float64{
			"时间维度": 0.74, "使用强度": 0.68, "工况": 1.10,
			"品牌": 1.05, "车况": 0.82, "市场": 0.95,
		},
		Suggestions: []string{
			"该叉车综合状况良好,建议可作为二手设备在二手市场出售,预期可回收约 12.35 万元。",
			"液压系统和操控部件有轻微磨损,若继续自用,建议安排预防性维护。",
			"品牌市场认可度高(合力 HELI),有利于二手交易,建议优先考虑品牌官方渠道。",
			"工况温和,二手市场需求稳定。",
		},
	}

	weights := model.CalcWeights{
		WWorkCondition: 0.20,
		WBrand:         0.20,
		WCondition:     0.50,
		WMarket:        0.10,
	}

	// 样例部件状态(对应设计稿中 7 个类别)
	items := []model.EvaluationItemDTO{
		{CategoryCode: "POWER", CategoryName: "动力系统", ItemCode: "p1", ItemName: "驱动电机", Status: model.ItemStatusNormal, Score: 1.0},
		{CategoryCode: "POWER", CategoryName: "动力系统", ItemCode: "p2", ItemName: "传动系统", Status: model.ItemStatusNormal, Score: 0.95},
		{CategoryCode: "HYDR", CategoryName: "液压系统", ItemCode: "h1", ItemName: "液压泵", Status: model.ItemStatusSlightWear, Score: 0.85},
		{CategoryCode: "HYDR", CategoryName: "液压系统", ItemCode: "h2", ItemName: "液压缸", Status: model.ItemStatusSlightWear, Score: 0.85},
		{CategoryCode: "ELEC", CategoryName: "电气系统", ItemCode: "e1", ItemName: "主控板", Status: model.ItemStatusNormal, Score: 1.0},
		{CategoryCode: "ELEC", CategoryName: "电气系统", ItemCode: "e2", ItemName: "仪表盘", Status: model.ItemStatusNormal, Score: 0.95},
		{CategoryCode: "FRAME", CategoryName: "车架与结构件", ItemCode: "f1", ItemName: "门架", Status: model.ItemStatusNormal, Score: 0.95},
		{CategoryCode: "TYRE", CategoryName: "轮胎与轮组", ItemCode: "t1", ItemName: "前轮", Status: model.ItemStatusNeedRepair, Score: 0.6},
		{CategoryCode: "CTRL", CategoryName: "操控与仪表", ItemCode: "c1", ItemName: "方向盘", Status: model.ItemStatusSlightWear, Score: 0.85},
		{CategoryCode: "SAFE", CategoryName: "安全装置", ItemCode: "s1", ItemName: "警示灯", Status: model.ItemStatusNormal, Score: 1.0},
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
		t.Errorf("PDF 文件过小 (%d 字节),可能未正确生成", info.Size())
	}
	t.Logf("PDF 生成成功: %s, 大小 %d 字节", path, info.Size())

	// 校验 PDF 魔数
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
		t.Errorf("PDF 首部不是 %%PDF,实际为 %q", head)
	}

	// 校验输出目录确实在临时目录下
	abs, _ := filepath.Abs(path)
	if !strings.HasPrefix(abs, filepath.Clean(dir)) {
		t.Errorf("PDF 路径 %s 未落在输出目录 %s 下", abs, dir)
	}

	// 校验页数(简洁版应为 3 页)
	pages, err := countPDFPages(path)
	if err != nil {
		t.Fatalf("读取 PDF 失败: %v", err)
	}
	if pages != 3 {
		t.Errorf("简洁版报告应为 3 页,实际 %d 页", pages)
	}

	// 校验内容(解压后应至少包含基本 PDF 操作符,表明内容已被写入)
	stream, err := extractPDFText(path)
	if err != nil {
		t.Fatalf("解压 PDF 失败: %v", err)
	}
	// 检查关键 PDF 操作符(以下表明内容已真正写入)
	opChecks := []string{
		"BT",         // 文本块开始
		"ET",         // 文本块结束
		"re",         // 矩形(用于表格/卡片/分隔线)
		"RG",         // 描边色设置
		"rg",         // 填充色设置
		"Tf",         // 字体设置
		"Sh1", "Sh2", // 渐变 sh pattern(设计稿的渐变条)
	}
	for _, op := range opChecks {
		if !strings.Contains(stream, op) {
			t.Errorf("PDF 流中缺少关键操作符 %q", op)
		}
	}
	// 至少应有数十个 BT 文本块(标题/正文/标签)
	if c := strings.Count(stream, "BT"); c < 50 {
		t.Errorf("PDF 文本块过少(%d),可能内容未完整写入", c)
	}
}

// TestGenerateReportCombustion 内燃叉车样例
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
		HydraulicOk:    false,
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
			"车况一般,多个部件需要维修",
			"液压系统异常,建议维修后再出售",
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
	pages, _ := countPDFPages(path)
	if pages != 3 {
		t.Errorf("简洁版报告应为 3 页,实际 %d 页", pages)
	}
	t.Logf("内燃叉车 PDF 生成成功: %s", path)
}

// TestGenerateReportEmptyItems 部件列表为空时也应能生成(且仍为 3 页)
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
	pages, _ := countPDFPages(path)
	if pages != 3 {
		t.Errorf("空数据时也应为 3 页,实际 %d 页", pages)
	}
	t.Logf("空部件 PDF 生成成功: %s", path)
}

// TestFindFontFile 验证字体文件查找逻辑
func TestFindFontFile(t *testing.T) {
	_, err := findFontFile()
	if err != nil {
		// 找不到时给出明确提示,但允许跳过(CI 环境下字体可能不存在)
		t.Skipf("字体文件未找到(CI 环境可接受): %v", err)
	}
}
