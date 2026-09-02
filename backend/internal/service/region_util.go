// Package service 地区契约工具（spec #484 / 子票 #486）。
// 意向地区与现居地统一「仅精确到市级」：存储两段「省/市」中文串（直辖市一段），
// 本文件提供回显拆分、无分隔串拆分、市名精确匹配等共享逻辑。
package service

import (
	"strings"
)

// regionShortProvince 省短名（去行政后缀）→ 省全名。江苏省→江苏、内蒙古自治区→内蒙古。
// 历史数据多以短名无分隔存储（「江苏苏州」），拆分时优先按短名匹配。
var regionShortProvince = func() map[string]string {
	m := make(map[string]string, len(provinceList))
	for _, prov := range provinceList {
		m[prov] = prov
		short := stripRegionSuffix(prov)
		if short != prov {
			m[short] = prov
		}
	}
	return m
}()

// regionShortCity 市短名（去「市/盟/州」后缀）→ 市全名。苏州市→苏州、延边朝鲜族自治州→延边。
var regionShortCity = func() map[string]map[string]string {
	m := make(map[string]map[string]string, len(regionProvinceCity))
	for prov, cities := range regionProvinceCity {
		inner := make(map[string]string, len(cities))
		for _, c := range cities {
			inner[c] = c
			short := stripRegionSuffix(c)
			if short != c {
				inner[short] = c
			}
		}
		m[prov] = inner
	}
	return m
}()

// stripRegionSuffix 去掉行政后缀：省/市/自治区/特别行政区/自治州/盟/地区/区。
func stripRegionSuffix(name string) string {
	for _, suf := range []string{"特别行政区", "维吾尔自治区", "回族自治区", "壮族自治区", "自治区", "自治州", "省", "市", "盟", "地区"} {
		if strings.HasSuffix(name, suf) {
			return strings.TrimSuffix(name, suf)
		}
	}
	return name
}

// SplitRegionPath 把存储串拆成路径段（按 / 拆分；直辖市一段）。
// 用于前端级联回显（重进编辑页不丢失）与筛选比较。
func SplitRegionPath(region string) []string {
	r := strings.TrimSpace(region)
	if r == "" {
		return nil
	}
	parts := strings.Split(r, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// SplitRegionNoSeparator 把历史无分隔串（如「江苏苏州精确地址123号」「江苏苏州」）拆成 (省, 市)。
// 命中返回省/市全名两段；无法识别返回 ("", "")。
func SplitRegionNoSeparator(s string) (string, string) {
	r := strings.TrimSpace(s)
	if r == "" {
		return "", ""
	}
	// 直辖市整段：北京市/天津市/上海市/重庆市
	if regionMunicipalities[r] {
		return r, ""
	}
	// 省前缀匹配（短名与全名都试，取最长命中）
	var matchedProv string
	for short := range regionShortProvince {
		if strings.HasPrefix(r, short) && len(short) > len(matchedProv) {
			matchedProv = short
		}
	}
	if matchedProv == "" {
		return "", ""
	}
	provFull := regionShortProvince[matchedProv]
	rest := strings.TrimPrefix(r, matchedProv)
	// 直辖市：只有一段，剩余区级信息不可作为市
	if regionMunicipalities[provFull] {
		return provFull, ""
	}
	if rest == "" {
		return provFull, ""
	}
	// 市前缀匹配（短名与全名都试，取最长命中）
	var matchedCity string
	for short := range regionShortCity[provFull] {
		if strings.HasPrefix(rest, short) && len(short) > len(matchedCity) {
			matchedCity = short
		}
	}
	if matchedCity == "" {
		return "", ""
	}
	return provFull, regionShortCity[provFull][matchedCity]
}

// RegionCityName 取地区的市名（第二段；直辖市一段取自身）。
// 简历库地区筛选精确匹配第 2 段（#486）：两段数据直接取第二段；
// 历史无分隔串按字典拆分；短名（如「苏州」）尽力映射到规范全名（「苏州市」）以便与迁移后数据精确匹配。
func RegionCityName(region string) string {
	parts := SplitRegionPath(region)
	if len(parts) == 0 {
		return ""
	}
	city := parts[0]
	if len(parts) >= 2 {
		city = parts[1]
	}
	// 直辖市一段：城市名即本身
	if regionMunicipalities[city] {
		return city
	}
	// 短名 → 全名映射（在所有城市短名中找）
	if full := fullCityName(city); full != "" {
		return full
	}
	// 历史无分隔串（如「江苏苏州」）：整串尝试拆分
	if _, c := SplitRegionNoSeparator(region); c != "" {
		return c
	}
	return city
}

// fullCityName 城市短名 → 规范全名（苏州市→苏州市；苏州→苏州市）；找不到返回空。
func fullCityName(short string) string {
	for _, cities := range regionShortCity {
		if full, ok := cities[short]; ok {
			return full
		}
	}
	return ""
}

// RegionProvinceName 取地区的省名（第一段；直辖市一段取自身）。
func RegionProvinceName(region string) string {
	parts := SplitRegionPath(region)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
