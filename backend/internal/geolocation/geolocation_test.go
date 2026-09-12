package geolocation

// 属地解析的口径测试。
//
// **刻意不绑定「某个 IP → 某个省」的映射**：xdb 是数据快照，更新后映射会变，
// 那样写等于给未来埋一个必红的测试（spec #887 的测试口径）。
// 这里只测三种可观察行为：占位值折空、边界地址落空、库真的能解出属地。

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// parseRegion 的输入是 xdb 实测样本。
func TestParseRegion(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		province string
		city     string
	}{
		{"中国省级市（江苏/南京）", "中国|江苏省|南京市|0|CN", "江苏省", "南京市"},
		{"中国省级市带 ISP", "中国|浙江省|杭州市|阿里|CN", "浙江省", "杭州市"},
		{"直辖市（省市同名）", "中国|上海市|上海市|电信|CN", "上海市", "上海市"},
		{"境外（市级占位 0）", "United States|California|0|Google LLC|US", "California", ""},
		{"境外（省市都有）", "Australia|Queensland|Brisbane|0|AU", "Queensland", "Brisbane"},
		{"内网 / 保留地址", "Reserved|Reserved|Reserved|0|0", "", ""},
		{"库只给到省级", "中国|广东省|0|0|CN", "广东省", ""},
		{"空串", "", "", ""},
		{"字段不足", "中国|广东省", "", ""},
		{"非记录串", "not-a-region", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRegion(tc.raw)
			if got.Province != tc.province || got.City != tc.city {
				t.Errorf("parseRegion(%q) = {%q, %q}, want {%q, %q}",
					tc.raw, got.Province, got.City, tc.province, tc.city)
			}
		})
	}
}

// 边界地址一律落空：内网、保留、非法、空、IPv6 —— 不报错、不阻断发帖（spec #887 边界与降级）。
func TestResolve_BoundariesAreEmpty(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1", "192.168.1.1", "10.0.0.1", "172.16.0.1", "169.254.1.1",
		"0.0.0.0", "255.255.255.255", "224.0.0.1",
		"not-an-ip", "", "   ",
		"2001:db8::1", "::1", "::ffff:192.168.1.1",
	} {
		if region := Resolve(ip); region != (Region{}) {
			t.Errorf("Resolve(%q) = %+v, want 空属地", ip, region)
		}
	}
}

// 库真的能用：取几个长期存在的国内公共地址，**只要求「至少一个能解出省级属地」**。
//
// 两头都不钉死：不断言「某个 IP → 某个省」（映射随 xdb 更新而变），也不把要求压在单一 IP 上
// （某条 IP 段调整不该让用例变红）。全解不出才说明 xdb 或解析器已失效。
func TestResolve_PublicAddressHasRegion(t *testing.T) {
	candidates := []string{"114.114.114.114", "223.5.5.5", "119.29.29.29"}
	var resolved Region
	var resolvedIP string
	for _, ip := range candidates {
		if region := Resolve(ip); region.Province != "" {
			resolved, resolvedIP = region, ip
			break
		}
	}
	if resolvedIP == "" {
		t.Fatalf("候选公共地址 %v 全部解析不出属地——xdb 或解析器已失效", candidates)
	}
	t.Logf("（示例）%s → %+v；本用例只断言「库可用」，不断言具体省份", resolvedIP, resolved)
}

// 版本记录来自 xdb 头，形如 ip2region_v4.xdb@<RFC3339>；格式固定但不锁具体日期。
func TestDataVersion(t *testing.T) {
	v := DataVersion()
	prefix := DataSource + "@"
	if !strings.HasPrefix(v, prefix) {
		t.Fatalf("DataVersion() = %q, want 前缀 %q", v, prefix)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(v, prefix)); err != nil {
		t.Errorf("版本记录应带可解析的生成时间: %v", err)
	}
}

// 内嵌数据文件的指纹与常量一致：人工替换 xdb 却忘了更新 DataSourceMD5 时在这里变红。
func TestEmbeddedDataFingerprint(t *testing.T) {
	sum := md5.Sum(ip2regionV4)
	if got := hex.EncodeToString(sum[:]); got != DataSourceMD5 {
		t.Errorf("内嵌 xdb 的 md5 = %s, DataSourceMD5 = %s —— 替换数据文件后请同步更新常量与文档",
			got, DataSourceMD5)
	}
}
