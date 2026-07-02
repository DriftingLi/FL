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

// sampleDetail 构造电动叉车样例 EvaluationDetail(贴近设计稿示例)
// 各 K 系数与维度评分一一对应,保证测试数据自洽
func sampleDetail() *model.EvaluationDetail {
	return &model.EvaluationDetail{
		ID:                         1,
		Brand:                      "合力 (HELI)",
		VehicleType:                "电动叉车",
		Series:                     "H2000",
		Tonnage:                    2.0,
		ConfigType:                 "标准配置",
		MastType:                   "标准门架",
		MastHeightMM:               3000,
		FactoryYear:                2020,
		SaleYear:                   2026,
		UsageHours:                 6800,
		OriginalPaint:              true,
		Province:                   "江苏",
		City:                       "苏州",
		HasLicensePlate:            true,
		HasRegistrationCertificate: true,
		HasMaintenanceRecords:      true,
		ConditionRating:            "B",
		OriginalPrice:              28.50,
		KTime:                      0.74,
		KHours:                     0.68,
		KBrand:                     1.05,
		KCondition:                 0.82,
		KMarket:                    0.95,
		EstimatedValue:             12.35,
		ConfidenceLow:              10.82,
		ConfidenceHigh:             13.88,
	}
}

// sampleDimensionScores 5 维评分(与雷达图顺序一致)
func sampleDimensionScores() map[string]float64 {
	return map[string]float64{
		"出厂时间": 0.74,
		"使用强度": 0.90,
		"品牌价值": 1.00,
		"市场需求": 0.95,
		"车辆情况": 0.82,
	}
}

// sampleSuggestions 8 类建议中可能命中的若干条
func sampleSuggestions() []string {
	return []string{
		"车况良好,残值稳定,可作为二手设备出售",
		"原厂漆完整且有维保记录,加成 6%,对保值有利",
		"品牌力较好,残值具备一定支撑",
		"残值率较高,建议按当前车况正常出售",
	}
}

// TestGenerateReport 验证 PDF 生成器能成功生成文件、首字节为 PDF 魔数,且为 3 页
func TestGenerateReport(t *testing.T) {
	// 准备临时输出目录
	dir := t.TempDir()
	gen := NewGenerator(dir)

	detail := sampleDetail()
	dimScores := sampleDimensionScores()
	suggestions := sampleSuggestions()

	// 调用生成器
	path, err := gen.GenerateReport(detail, dimScores, suggestions)
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

// TestGenerateReportCombustion 内燃叉车样例(无电池类型,无原厂漆加成)
func TestGenerateReportCombustion(t *testing.T) {
	dir := t.TempDir()
	gen := NewGenerator(dir)

	detail := &model.EvaluationDetail{
		ID:                         2002,
		Brand:                      "三菱 MITSUBISHI",
		VehicleType:                "内燃叉车",
		Series:                     "FD30N",
		Tonnage:                    3.0,
		ConfigType:                 "高配置",
		MastType:                   "三级门架",
		MastHeightMM:               4500,
		FactoryYear:                2019,
		SaleYear:                   2024,
		UsageHours:                 8000,
		OriginalPaint:              false,
		Province:                   "上海",
		City:                       "上海",
		HasLicensePlate:            true,
		HasRegistrationCertificate: false,
		HasMaintenanceRecords:      false,
		ConditionRating:            "C",
		OriginalPrice:              18.00,
		KTime:                      0.61,
		KHours:                     0.90,
		KBrand:                     1.00,
		KCondition:                 0.75,
		KMarket:                    0.98,
		EstimatedValue:             4.50,
		ConfidenceLow:              4.28,
		ConfidenceHigh:             4.73,
	}
	dimScores := map[string]float64{
		"时间维度": 0.61,
		"使用强度": 0.90,
		"品牌":   1.00,
		"车况":   0.75,
		"市场":   0.98,
	}
	suggestions := []string{
		"车况一般,多个维度有折损,建议折价处理",
		"缺少登记证,残值扣减 5%,过户需提供登记证",
		"品牌力较好,残值具备一定支撑",
	}

	path, err := gen.GenerateReport(detail, dimScores, suggestions)
	if err != nil {
		t.Fatalf("生成 PDF 失败: %v", err)
	}
	pages, _ := countPDFPages(path)
	if pages != 3 {
		t.Errorf("简洁版报告应为 3 页,实际 %d 页", pages)
	}
	t.Logf("内燃叉车 PDF 生成成功: %s", path)
}

// TestGenerateReportEmptySuggestions 建议列表为空时也应能生成(且仍为 3 页)
func TestGenerateReportEmptySuggestions(t *testing.T) {
	dir := t.TempDir()
	gen := NewGenerator(dir)

	detail := &model.EvaluationDetail{
		ID:                         3,
		Brand:                      "永恒力 JUNGHEINRICH",
		VehicleType:                "电动叉车",
		Series:                     "EFG",
		Tonnage:                    1.5,
		ConfigType:                 "标准配置",
		MastType:                   "标准门架",
		MastHeightMM:               2500,
		FactoryYear:                2022,
		SaleYear:                   2024,
		UsageHours:                 1200,
		OriginalPaint:              true,
		Province:                   "北京",
		City:                       "北京",
		HasLicensePlate:            true,
		HasRegistrationCertificate: true,
		HasMaintenanceRecords:      true,
		ConditionRating:            "A",
		OriginalPrice:              9.00,
		KTime:                      0.85,
		KHours:                     1.10,
		KBrand:                     1.10,
		KCondition:                 1.10,
		KMarket:                    1.00,
		EstimatedValue:             10.36,
		ConfidenceLow:              9.32,
		ConfidenceHigh:             11.40,
	}
	dimScores := map[string]float64{
		"时间维度": 0.85,
		"使用强度": 1.10,
		"品牌":   1.10,
		"车况":   1.10,
		"市场":   1.00,
	}

	path, err := gen.GenerateReport(detail, dimScores, nil)
	if err != nil {
		t.Fatalf("空建议列表也必须能生成: %v", err)
	}
	pages, _ := countPDFPages(path)
	if pages != 3 {
		t.Errorf("空数据时也应为 3 页,实际 %d 页", pages)
	}
	t.Logf("空建议 PDF 生成成功: %s", path)
}

// TestEmbeddedFont 验证内嵌字体字节已通过 //go:embed 编译进二进制
func TestEmbeddedFont(t *testing.T) {
	if len(embeddedFont) == 0 {
		t.Fatal("内嵌字体字节为空，//go:embed 未生效")
	}
	if len(embeddedFont) < 1_000_000 {
		t.Errorf("内嵌字体过小 (%d 字节)，可能不是完整的 TTF 文件", len(embeddedFont))
	}
	t.Logf("内嵌字体大小: %d 字节", len(embeddedFont))
}
