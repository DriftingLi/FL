package service

import "testing"

func TestParseGradingResponse_CleanJSON(t *testing.T) {
	r := parseGradingResponse(`{"score": 8, "comment": "回答正确但缺少细节"}`, 10)
	if r == nil {
		t.Fatal("期望非 nil")
	}
	if r.Score != 8 {
		t.Errorf("Score = %v，期望 8", r.Score)
	}
	if r.Comment != "回答正确但缺少细节" {
		t.Errorf("Comment = %q", r.Comment)
	}
}

func TestParseGradingResponse_CodeBlock(t *testing.T) {
	input := "```json\n{\"score\": 9, \"comment\": \"优秀\"}\n```"
	r := parseGradingResponse(input, 10)
	if r == nil {
		t.Fatal("代码块包裹应能解析")
	}
	if r.Score != 9 {
		t.Errorf("Score = %v，期望 9", r.Score)
	}
	if r.Comment != "优秀" {
		t.Errorf("Comment = %q", r.Comment)
	}
}

func TestParseGradingResponse_EmbeddedJSON(t *testing.T) {
	input := "评分结果：{\"score\": 7, \"comment\": \"基本正确\"}，请参考标准答案。"
	r := parseGradingResponse(input, 10)
	if r == nil {
		t.Fatal("嵌入 JSON 应能解析")
	}
	if r.Score != 7 {
		t.Errorf("Score = %v，期望 7", r.Score)
	}
}

func TestParseGradingResponse_ScoreCommentPattern(t *testing.T) {
	input := `这是评分："score": 6, "comment": "需改进"`
	r := parseGradingResponse(input, 10)
	if r == nil {
		t.Fatal("score/comment 模式应能解析")
	}
	if r.Score != 6 {
		t.Errorf("Score = %v，期望 6", r.Score)
	}
}

func TestParseGradingResponse_ScoreOutOfMax(t *testing.T) {
	r := parseGradingResponse("8/10", 10)
	if r == nil {
		t.Fatal("'数字/满分' 格式应能解析")
	}
	if r.Score != 8 {
		t.Errorf("Score = %v，期望 8", r.Score)
	}
}

func TestParseGradingResponse_ScoreWithFen(t *testing.T) {
	r := parseGradingResponse("得分 7分", 10)
	if r == nil {
		t.Fatal("'数字分' 格式应能解析")
	}
	if r.Score != 7 {
		t.Errorf("Score = %v，期望 7", r.Score)
	}
}

func TestParseGradingResponse_ClampScore(t *testing.T) {
	// 分数超过满分应被限制
	r := parseGradingResponse(`{"score": 15, "comment": "test"}`, 10)
	if r == nil {
		t.Fatal("期望非 nil")
	}
	if r.Score != 10 {
		t.Errorf("Score = %v，应被限制为 10", r.Score)
	}
}

func TestParseGradingResponse_NegativeScore(t *testing.T) {
	r := parseGradingResponse(`{"score": -5, "comment": "test"}`, 10)
	if r == nil {
		t.Fatal("期望非 nil")
	}
	if r.Score != 0 {
		t.Errorf("Score = %v，应被限制为 0", r.Score)
	}
}

func TestParseGradingResponse_Empty(t *testing.T) {
	if r := parseGradingResponse("", 10); r != nil {
		t.Error("空字符串应返回 nil")
	}
	if r := parseGradingResponse("   ", 10); r != nil {
		t.Error("纯空格应返回 nil")
	}
}

func TestParseGradingResponse_Invalid(t *testing.T) {
	if r := parseGradingResponse("无法解析的文本", 10); r != nil {
		t.Error("无法解析的文本应返回 nil")
	}
}

func TestTryParseScore(t *testing.T) {
	// 有效 JSON
	r := tryParseScore(`{"score": 5, "comment": "ok"}`, 10)
	if r == nil || r.Score != 5 || r.Comment != "ok" {
		t.Errorf("tryParseScore 有效 JSON 失败: %+v", r)
	}
	// 无效 JSON
	if r := tryParseScore("not json", 10); r != nil {
		t.Error("无效 JSON 应返回 nil")
	}
	// 缺少 score 字段
	r = tryParseScore(`{"comment": "no score"}`, 10)
	if r == nil {
		t.Fatal("缺少 score 应返回非 nil（score 默认 0）")
	}
	if r.Score != 0 {
		t.Errorf("缺少 score 时默认 0，得到 %v", r.Score)
	}
}

func TestExtractBraceJSON(t *testing.T) {
	// 前后带文本的 JSON（非嵌套）
	input := `prefix {"score": 9, "comment": "good"} suffix`
	r := extractBraceJSON(input, 10)
	if r == nil {
		t.Fatal("带文本前缀的 JSON 应能提取")
	}
	if r.Score != 9 {
		t.Errorf("Score = %v，期望 9", r.Score)
	}
	if r.Comment != "good" {
		t.Errorf("Comment = %q，期望 'good'", r.Comment)
	}
	// 不含 score 的 JSON
	if r := extractBraceJSON(`{"foo": "bar"}`, 10); r != nil {
		t.Error("不含 score 的 JSON 应返回 nil")
	}
}

func TestFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"test.pdf", "pdf"},
		{"archive.tar.gz", "gz"},
		{"noext", ""},
		{".gitignore", "gitignore"},
		{"path/to/file.PPTX", "pptx"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := fileExtension(tt.filename); got != tt.want {
			t.Errorf("fileExtension(%q) = %q，期望 %q", tt.filename, got, tt.want)
		}
	}
}
