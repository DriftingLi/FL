package service

import (
	"context"
	"testing"
	"time"
)

// fakeExportStore ExportStore 的测试替身（生产为估值模块 pgx adapter）。
type fakeExportStore struct {
	rows []EvaluationExportRow
}

func (f *fakeExportStore) ListEvaluationExports(_ context.Context) ([]EvaluationExportRow, error) {
	return f.rows, nil
}

// TestExportEvaluations 评估导出行格式契约：
// 表头、NULL 系数/置信区间/报告路径的展示、bool 中文映射、时间格式——经 ExportStore seam 锁定。
func TestExportEvaluations(t *testing.T) {
	tm := time.Date(2026, 8, 8, 10, 30, 0, 0, time.UTC)
	kt := 0.8123
	var nilPtr *float64
	svc := NewExportService(nil, &fakeExportStore{rows: []EvaluationExportRow{
		{
			ID: 1, Account: "alice", Username: "张三", Brand: "Toyota", VehicleType: "FBT",
			Series: "Series-1", Tonnage: 3.5, ConfigType: "标准", MastType: "L型", MastHeightMM: 3000,
			FactoryYear: 2020, SaleYear: 2021, UsageHours: 1200, OriginalPaint: true,
			Province: "广东省", City: "广州市", HasLicensePlate: true, HasRegistrationCert: false,
			HasMaintenanceRecords: true, ConditionRating: "优", OriginalPrice: 180000,
			KTime: &kt, KHours: nilPtr, KBrand: nilPtr, KCondition: nilPtr, KMarket: nilPtr,
			EstimatedValue: 123456.78, ConfidenceLow: nilPtr, ConfidenceHigh: nilPtr,
			ReportPDFPath: "http://x/report.pdf", CreatedAt: tm,
		},
	}}, nil)

	got, err := svc.Evaluations()
	if err != nil {
		t.Fatalf("Evaluations() 失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应返回表头+1行, got %d 行", len(got))
	}
	wantHeader := []any{"ID", "账号", "昵称", "品牌", "车型", "系列", "吨位", "配置", "门架类型", "门架高度mm",
		"出厂年份", "销售年份", "工时", "原厂漆", "省份", "城市", "有牌照", "有登记证", "有维保记录",
		"车况", "原价", "Kt", "Kh", "Kb", "Kc", "Km", "评估值", "置信下限", "置信上限", "报告PDF", "创建时间"}
	for i, h := range wantHeader {
		if got[0][i] != h {
			t.Fatalf("表头第 %d 列 = %v, 期望 %v", i, got[0][i], h)
		}
	}
	row := got[1]
	if row[0] != int64(1) || row[1] != "alice" || row[2] != "张三" {
		t.Fatalf("ID/账号/昵称不符: %v", row[:3])
	}
	if row[13] != "是" || row[16] != "是" || row[17] != "否" || row[18] != "是" {
		t.Fatalf("bool 中文映射不符: %v", row[13:19])
	}
	if row[21] != "0.8123" {
		t.Fatalf("Kt 格式不符: %v", row[21])
	}
	if row[22] != "" || row[23] != "" || row[27] != "" || row[28] != "" {
		t.Fatalf("NULL 系数/置信区间应显示为空: %v", row[22:29])
	}
	if row[29] != "http://x/report.pdf" {
		t.Fatalf("报告路径不符: %v", row[29])
	}
	if row[30] != "2026-08-08T10:30:00.000000" {
		t.Fatalf("创建时间格式不符: %v", row[30])
	}
}
