package entcode

import (
	"regexp"
	"strings"
	"testing"
)

// 编码要同时满足：带前缀、字符集落在 [a-z0-9_]、长度不超过各表 code 列的下限（30）。
func TestGenerate(t *testing.T) {
	shape := regexp.MustCompile(`^specialty_[0-9a-f]{16}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		code, err := Generate("specialty")
		if err != nil {
			t.Fatalf("生成失败: %v", err)
		}
		if !shape.MatchString(code) {
			t.Fatalf("编码形状不符: %q", code)
		}
		if len(code) > 30 {
			t.Fatalf("编码超过 30 字符: %q", code)
		}
		if seen[code] {
			t.Fatalf("100 次内出现重复编码: %q", code)
		}
		seen[code] = true
	}
}

// faq 分类标识的校验正则比其它表更严（小写字母 / 数字 / 下划线，2–64 位），生成值必须直接可用。
func TestGenerateMatchesFaqCodeRe(t *testing.T) {
	faqCodeRe := regexp.MustCompile(`^[a-z0-9_]{2,64}$`)
	code, err := Generate("faq")
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if !faqCodeRe.MatchString(code) {
		t.Fatalf("生成值不满足 faq 标识正则: %q", code)
	}
	if !strings.HasPrefix(code, "faq_") {
		t.Fatalf("前缀丢失: %q", code)
	}
}
