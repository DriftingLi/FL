// Package service 业务服务层测试：phone 占位 sentinel 单点（PlaceholderPhonePrefix）与静态扫描门禁。
// 仿 ADR-0013 §2 dict 缓存契约门禁（repository/dict_cache_keys_test.go）：用静态测试把
// 散落的裸 "email_" 字面量挡在包外，强制所有 email_ 占位判定收敛到 IsPlaceholderPhone。
package service

import (
	"os"
	"strings"
	"testing"
)

func TestPlaceholderPhonePrefix_SentinelValue(t *testing.T) {
	// 独立字面量锁定：占位 sentinel 与既有契约一致（邮箱注册 / 微信建号 / 注销哨兵）。
	if got := PlaceholderPhonePrefix; got != "email_" {
		t.Errorf("PlaceholderPhonePrefix = %q, want %q", got, "email_")
	}
	if got := PlaceholderWechatPhonePrefix; got != "wxp_" {
		t.Errorf("PlaceholderWechatPhonePrefix = %q, want %q", got, "wxp_")
	}
	if got := PlaceholderDeletedSentinel; got != "deleted__sentinel" {
		t.Errorf("PlaceholderDeletedSentinel = %q, want %q", got, "deleted__sentinel")
	}
}

func TestIsPlaceholderPhone(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"email_0123456789abcdef0123456789abcdef", true}, // 邮箱注册占位值形态
		{"email_", true}, // 前缀本身
		{"wxp_oGZUjt9xK3mQ1aB2cD3eF4gH5iJ6", true}, // 微信建号占位（openID 派生）
		{"wxp_", true},                 // 微信前缀本身
		{"deleted__sentinel", true},    // 注销哨兵（精确匹配）
		{"deleted__sentinel_x", false}, // 哨兵仅精确匹配，不按前缀
		{"13800138000", false},         // 真实手机号（符合 1[3-9] 前缀）
		{"13800000000", false},
		{"EMAIL_x", false}, // 大小写敏感（与既有 HasPrefix 语义一致）
		{"WXP_x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsPlaceholderPhone(c.in); got != c.want {
			t.Errorf("IsPlaceholderPhone(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestNoBareEmailPlaceholderLiteral 静态扫描本包全部 .go 源文件，禁止出现裸 "email_" 字面量
// 散落在业务代码里（emailPlaceholderPhone / currentUserPhone / MaskedPhone 均已改用
// PlaceholderPhonePrefix / IsPlaceholderPhone）。新增占位判定若直接书写字面量即测试失败。
//
// 单点 source-of-truth 是 placeholder_phone.go 本文件声明的常量；该文件（常量定义点）与本
// 门禁测试自身允许出现字面量，故排除二者。匹配精确到带右引号的 "email_"，避免误报验证码缓存 key
// "email_code"（KeyPrefix）等含 email_ 开头子串的合法字符串。
func TestNoBareEmailPlaceholderLiteral(t *testing.T) {
	exempt := map[string]bool{
		"placeholder_phone.go":           true, // 常量定义点
		"placeholder_phone_gate_test.go": true, // 测试自身
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取包目录失败: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || exempt[name] {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "\"email_\"") {
				t.Errorf("%s:%d 含游离的 email_ 占位字面量，应改用 PlaceholderPhonePrefix / IsPlaceholderPhone: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
