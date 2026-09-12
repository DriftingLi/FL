package config

// TRUSTED_PROXIES 解析与校验（ticket #888）。
// 格式必须在装配路由器之前就校验掉：gin 的 SetTrustedProxies 在遇到非法条目时
// 会返回「已解析的前半截 + error」，只靠它兜底会留下一个半截的可信列表。
import "testing"

func TestSplitProxies(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"空值", "", nil},
		{"仅空白", "   ", nil},
		{"仅逗号", ",,", nil},
		{"单个 IP", "127.0.0.1", []string{"127.0.0.1"}},
		{"混合并去空白", "127.0.0.1, 172.19.0.0/16 ,,::1", []string{"127.0.0.1", "172.19.0.0/16", "::1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitProxies(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitProxies(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitProxies(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestValidate_TrustedProxies(t *testing.T) {
	cases := []struct {
		name    string
		proxies []string
		wantErr bool
	}{
		{"未配置（默认不信任任何代理）", nil, false},
		{"裸 IP 与 CIDR 混合", []string{"127.0.0.1", "172.19.0.0/16", "::1", "2001:db8::/32"}, false},
		{"非法条目", []string{"127.0.0.1", "not-an-ip"}, true},
		{"掩码越界", []string{"172.19.0.1/33"}, true},
		{"空条目（不应绕过校验）", []string{""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{AppEnv: "development", TrustedProxies: tc.proxies}
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("TrustedProxies=%v 应校验失败", tc.proxies)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("TrustedProxies=%v 不应报错: %v", tc.proxies, err)
			}
		})
	}
}
