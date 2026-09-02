// 单测 #486 地区契约工具：无分隔串拆分、市名提取、路径拆分。
package service

import "testing"

func TestSplitRegionNoSeparator(t *testing.T) {
	cases := []struct {
		in   string
		prov string
		city string
	}{
		{"江苏苏州精确地址123号", "江苏省", "苏州市"},
		{"江苏苏州", "江苏省", "苏州市"},
		{"北京市", "北京市", ""},
		{"北京市朝阳区", "北京市", ""}, // 直辖市后跟区级：只有一段，区不可作为市
		{"上海", "上海市", ""},
		{"浙江杭州", "浙江省", "杭州市"},
		{"广东省广州市天河区", "广东省", "广州市"},
		{"内蒙古呼和浩特", "内蒙古自治区", "呼和浩特市"},
		{"新疆乌鲁木齐市", "新疆维吾尔自治区", "乌鲁木齐市"},
		{"乱七八糟不可识别", "", ""},
	}
	for _, c := range cases {
		prov, city := SplitRegionNoSeparator(c.in)
		if prov != c.prov || city != c.city {
			t.Errorf("SplitRegionNoSeparator(%q) = (%q,%q), want (%q,%q)", c.in, prov, city, c.prov, c.city)
		}
	}
}

func TestRegionCityName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"江苏省/苏州市", "苏州市"},
		{"浙江/杭州", "杭州市"},
		{"北京市", "北京市"},
		{"江苏苏州", "苏州市"},
	}
	for _, c := range cases {
		if got := RegionCityName(c.in); got != c.want {
			t.Errorf("RegionCityName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitRegionPath(t *testing.T) {
	if got := SplitRegionPath("江苏省/苏州市"); len(got) != 2 || got[0] != "江苏省" || got[1] != "苏州市" {
		t.Errorf("SplitRegionPath 两段失败: %v", got)
	}
	if got := SplitRegionPath("北京市"); len(got) != 1 || got[0] != "北京市" {
		t.Errorf("SplitRegionPath 直辖市失败: %v", got)
	}
	if got := SplitRegionPath(""); got != nil {
		t.Errorf("空串应返回 nil: %v", got)
	}
}
