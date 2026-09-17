// Package pdf 实现 PDF 报告生成
// 本文件:电池 RUL 评估报告的最小渲染用例（#1105 判据「简历 + 两份估值报告」缺电池这条）。
package pdf

import (
	"strings"
	"testing"

	"forklift-training/internal/valuation/model"
)

// sampleBatteryEvaluation 最小电池评估记录：五段模板（封面/基本信息/结论/特征/免责）都有数据可渲染。
func sampleBatteryEvaluation() *model.BatteryEvaluation {
	return &model.BatteryEvaluation{
		ID:             123,
		BatteryType:    model.BatteryTypeLFP,
		BatteryModel:   "CATL-280Ah",
		CycleCount:     860,
		RulCycles:      2400,
		SohPercent:     91.3,
		Confidence:     0.88,
		ConfidenceLow:  2100,
		ConfidenceHigh: 2700,
		FeatureImportance: []model.FeatureImportance{
			{Index: 1, Name: "容量衰减速率", Group: "capacity", Weight: 0.32, Normalized: 0.95},
			{Index: 2, Name: "内阻增长", Group: "resistance", Weight: 0.24, Normalized: 0.71},
		},
		Suggestions: []string{"继续按现有工况使用，每季度复核一次容量"},
		CreatedAt:   "2026-09-17 10:00:00",
	}
}

// TestGenerateBatteryReport 电池报告路径的最小判据：%PDF 头 + 无错误。
// 与 TestGenerateReport（叉车）一起覆盖 #1105 的三条渲染路径（简历 / 叉车报告 / 电池报告）。
func TestGenerateBatteryReport(t *testing.T) {
	data, err := GenerateBatteryReportBytes(sampleBatteryEvaluation())
	if err != nil {
		t.Fatalf("生成电池报告失败: %v", err)
	}
	if len(data) < 1024 {
		t.Errorf("PDF 内容过小 (%d 字节),可能未正确生成", len(data))
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Errorf("PDF 首部不是 %%PDF,实际为 %q", data[:4])
	}
}

// TestGenerateBatteryReportRejectsNil 空记录必须报错（handler 的 500 分支由它守住）。
func TestGenerateBatteryReportRejectsNil(t *testing.T) {
	if data, err := GenerateBatteryReportBytes(nil); err == nil {
		t.Fatalf("nil 评估记录必须报错，实际返回 %d 字节", len(data))
	}
}
