package api

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

// TestCellString CSV 单元格序列化表驱动（#230）：
// float 截断、nil 单元格、普通值透传（逗号/引号转义在 encoding/csv 层，见 TestEncodeCSVRoundTrip）。
func TestCellString(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil 单元格", nil, ""},
		{"空字符串", "", ""},
		{"float64 整数去尾零", float64(180000), "180000"},
		{"float64 小数", float64(3.5), "3.5"},
		{"float64 高精度", float64(123456.789), "123456.789"},
		{"float32", float32(1.25), "1.25"},
		{"整数", 123, "123"},
		{"bool", true, "true"},
		{"字符串数字", "007", "007"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cellString(c.in); got != c.want {
				t.Errorf("cellString(%v) = %q, expect %q", c.in, got, c.want)
			}
		})
	}
}

// TestEncodeCSVBasics CSV 序列化基础（#230）：UTF-8 BOM 前缀 + 表头 + float 去尾零 + nil 单元格。
func TestEncodeCSVBasics(t *testing.T) {
	rows := [][]any{{"ID", "账号"}, {float64(180000), nil}, {float64(3.5), "x"}}
	b, err := encodeCSV(rows)
	if err != nil {
		t.Fatalf("encodeCSV 失败: %v", err)
	}
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("CSV 缺少 UTF-8 BOM 前缀")
	}
	want := "ID,账号\n180000,\n3.5,x\n"
	if got := string(b[3:]); got != want {
		t.Errorf("CSV 内容 = %q, expect %q", got, want)
	}
}

// TestEncodeCSVRoundTrip 逗号/引号/换行经 encoding/csv 正确转义（#230）：
// 用 csv.Reader 反解析，断言单元格值逐字还原，证明序列化未破坏字段边界。
func TestEncodeCSVRoundTrip(t *testing.T) {
	rows := [][]any{
		{"a,b", "say \"hi\""},
		{"line1" + string(rune(10)) + "line2", ""},
		{"plain", 123},
	}
	b, err := encodeCSV(rows)
	if err != nil {
		t.Fatalf("encodeCSV 失败: %v", err)
	}
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("CSV 缺少 UTF-8 BOM 前缀")
	}
	r := csv.NewReader(strings.NewReader(string(b[3:])))
	rec1, err := r.Read()
	if err != nil {
		t.Fatalf("解析第一行失败: %v", err)
	}
	if len(rec1) != 2 || rec1[0] != "a,b" || rec1[1] != "say \"hi\"" {
		t.Errorf("逗号/引号行还原失败: %v", rec1)
	}
	rec2, err := r.Read()
	if err != nil {
		t.Fatalf("解析第二行失败: %v", err)
	}
	if len(rec2) != 2 || rec2[0] != "line1"+string(rune(10))+"line2" {
		t.Errorf("换行字段还原失败: %v", rec2)
	}
	rec3, err := r.Read()
	if err != nil {
		t.Fatalf("解析第三行失败: %v", err)
	}
	if len(rec3) != 2 || rec3[0] != "plain" || rec3[1] != "123" {
		t.Errorf("常规行还原失败: %v", rec3)
	}
}

// TestContentDisposition 四类导出文件名唯一真值为后端（#230）。
func TestContentDisposition(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"学员名单.csv", "attachment; filename=\"export.csv\"; filename*=UTF-8''" + "%E5%AD%A6%E5%91%98%E5%90%8D%E5%8D%95.csv"},
		{"成绩单.csv", "attachment; filename=\"export.csv\"; filename*=UTF-8''" + "%E6%88%90%E7%BB%A9%E5%8D%95.csv"},
		{"题库.csv", "attachment; filename=\"export.csv\"; filename*=UTF-8''" + "%E9%A2%98%E5%BA%93.csv"},
		{"评估记录.csv", "attachment; filename=\"export.csv\"; filename*=UTF-8''" + "%E8%AF%84%E4%BC%B0%E8%AE%B0%E5%BD%95.csv"},
	}
	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			if got := contentDisposition(c.filename); got != c.want {
				t.Errorf("contentDisposition(%q) = %q, expect %q", c.filename, got, c.want)
			}
		})
	}
}
