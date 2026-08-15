package service

import (
	"testing"
	"time"
)

// TestEvaluationExportColumnsShapeLock 评估导出列 spec shape-lock（#229 列描述单点）。
// 断言 spec 顺序（即 repository SELECT 返回序）与既有 31 列表头契约逐字一致，
// 防止「SQL 返回序 ↔ 表头序」错位漂移。任何列增删/重排都会在此变红。
func TestEvaluationExportColumnsShapeLock(t *testing.T) {
	wantHeaders := []string{
		"ID", "账号", "昵称", "品牌", "车型", "系列", "吨位", "配置", "门架类型", "门架高度mm",
		"出厂年份", "销售年份", "工时", "原厂漆", "省份", "城市", "有牌照", "有登记证", "有维保记录",
		"车况", "原价", "Kt", "Kh", "Kb", "Kc", "Km", "评估值", "置信下限", "置信上限", "报告PDF", "创建时间",
	}
	if len(EvaluationExportColumns) != len(wantHeaders) {
		t.Fatalf("列数 = %d, 期望 %d", len(EvaluationExportColumns), len(wantHeaders))
	}
	for i, col := range EvaluationExportColumns {
		if col.Header != wantHeaders[i] {
			t.Errorf("表头第 %d 列 = %q, 期望 %q", i, col.Header, wantHeaders[i])
		}
		if col.Select == "" {
			t.Errorf("第 %d 列缺少 SELECT 表达式", i)
		}
		if col.Scan == nil || col.Value == nil {
			t.Errorf("第 %d 列缺 Scan/Value 函数", i)
		}
	}

	// spec 驱动生成的 SELECT 列序必须与既有 SQL 契约逐字一致（repository 返回序）。
	wantSelect := []string{
		"e.id", "COALESCE(u.account, '') AS account", "COALESCE(u.username, '') AS username",
		"e.brand", "e.vehicle_type", "e.series", "e.tonnage", "e.config_type", "e.mast_type",
		"e.mast_height_mm", "e.factory_year", "e.sale_year", "e.usage_hours", "e.original_paint",
		"e.province", "e.city", "e.has_license_plate", "e.has_registration_certificate AS has_registration_cert",
		"e.has_maintenance_records", "e.condition_rating", "e.original_price", "e.k_time", "e.k_hours",
		"e.k_brand", "e.k_condition", "e.k_market", "e.estimated_value", "e.confidence_low", "e.confidence_high",
		"e.report_pdf_path", "e.created_at",
	}
	gotSel := ""
	for i, s := range wantSelect {
		if i > 0 {
			gotSel += ", "
		}
		gotSel += s
	}
	if got := BuildEvalExportSelect(); got != gotSel {
		t.Errorf("BuildEvalExportSelect = %q\n期望 %q", got, gotSel)
	}
}

// feedScanArg 把模拟数据库扫描值写入 Scan 落位指针（类型与 pgx 返回保持一致）。
func feedScanArg(dest any, raw any) {
	switch d := dest.(type) {
	case *int64:
		*d = raw.(int64)
	case *string:
		*d = raw.(string)
	case *float64:
		*d = raw.(float64)
	case *int:
		*d = raw.(int)
	case *bool:
		*d = raw.(bool)
	case **float64:
		if raw == nil {
			*d = nil
		} else {
			v := raw.(float64)
			*d = &v
		}
	case **string:
		if raw == nil {
			*d = nil
		} else {
			s := raw.(string)
			*d = &s
		}
	case *time.Time:
		*d = raw.(time.Time)
	default:
		t00 := dest // 保留变量以支持类型断言失败诊断
		_ = t00
		panic("feedScanArg: 不支持的落位类型")
	}
}

// TestEvaluationExportColumnsScanFormat 绑定「Scan 落位 ↔ Value 取值」：
// 用模拟数据库扫描值依 spec 顺序喂入 Scan，再由 Value 取值，断言与既有导出单元格契约一致。
// 这是 repository position Scan 的唯一无 DB 可测替身（#229）。
func TestEvaluationExportColumnsScanFormat(t *testing.T) {
	tm := time.Date(2026, 8, 8, 10, 30, 0, 0, time.UTC)
	kt := 0.8123
	fv := func(v float64) any { return v }

	var r EvaluationExportRow
	dests, commits := ScanEvalExportDestinations(&r)

	// 模拟数据库返回序（与 spec/SELECT 顺序一致）喂入 Scan 落位。
	raws := []any{
		int64(1), "alice", "张三", "Toyota", "FBT", "Series-1", 3.5, "标准", "L型",
		3000, 2020, 2021, 1200, true, "广东省", "广州市", true, false, true,
		"优", 180000.0, fv(kt), nil, nil, nil, nil, 123456.78, nil, nil, "http://x/report.pdf", tm,
	}
	if len(raws) != len(EvaluationExportColumns) {
		t.Fatalf("模拟值列数 = %d, 期望 %d", len(raws), len(EvaluationExportColumns))
	}
	for i, dest := range dests {
		feedScanArg(dest, raws[i])
	}
	for _, c := range commits {
		if c != nil {
			c()
		}
	}

	wantHeader := []string{
		"ID", "账号", "昵称", "品牌", "车型", "系列", "吨位", "配置", "门架类型", "门架高度mm",
		"出厂年份", "销售年份", "工时", "原厂漆", "省份", "城市", "有牌照", "有登记证", "有维保记录",
		"车况", "原价", "Kt", "Kh", "Kb", "Kc", "Km", "评估值", "置信下限", "置信上限", "报告PDF", "创建时间",
	}
	got := buildEvalExportHeader()
	if len(got) != len(wantHeader) {
		t.Fatalf("表头列数 = %d, 期望 %d", len(got), len(wantHeader))
	}
	for i, h := range wantHeader {
		if got[i] != h {
			t.Errorf("表头第 %d 列 = %v, 期望 %v", i, got[i], h)
		}
	}

	wantRow := []any{int64(1), "alice", "张三", "Toyota", "FBT", "Series-1", 3.5, "标准", "L型",
		3000, 2020, 2021, 1200, "是", "广东省", "广州市", "是", "否", "是",
		"优", 180000.0, "0.8123", "", "", "", "", 123456.78, "", "", "http://x/report.pdf", "2026-08-08T10:30:00.000000"}
	gotRow := buildEvalExportRow(r)
	if len(gotRow) != len(wantRow) {
		t.Fatalf("行单元格数 = %d, 期望 %d", len(gotRow), len(wantRow))
	}
	for i, w := range wantRow {
		if gotRow[i] != w {
			t.Errorf("单元格第 %d 列 = %#v, 期望 %#v", i, gotRow[i], w)
		}
	}
}

// buildEvalExportHeader 依 spec 生成表头（供 shape-lock 测试比对）。
func buildEvalExportHeader() []any {
	hdr := make([]any, 0, len(EvaluationExportColumns))
	for _, col := range EvaluationExportColumns {
		hdr = append(hdr, col.Header)
	}
	return hdr
}

// buildEvalExportRow 依 spec 生成一行的导出单元格值（供 shape-lock 测试比对）。
func buildEvalExportRow(r EvaluationExportRow) []any {
	cell := make([]any, 0, len(EvaluationExportColumns))
	for _, col := range EvaluationExportColumns {
		cell = append(cell, col.Value(r))
	}
	return cell
}
