// 简历卡投影模块的锁（ADR-0053 §4）：
//  1. 打码规则的边界（姓名）
//  2. 持证去图的白名单**集合断言**（fail-closed：新键默认不露面）
//  3. 哨兵值行为断言：敏感字段填独特哨兵，脱敏卡与打码版 PDF 的取值都不得含它
//  4. 三处投影的字段清单（L2 卡的 JSON 键集 = 有意投影面）
package service

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"forklift-training/internal/model"
)

// TestMaskRealName_Boundaries 姓名打码边界（空 / 一字 / 两字 / 多字 / 含空白）。
func TestMaskRealName_Boundaries(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"   ":      "",
		"张":        "*",
		"张三":       "张*",
		"张三丰":      "张*丰",
		"欧阳锋大侠":    "欧***侠",
		"  张三丰  ":  "张*丰",
		"John Doe": "J******e",
	}
	for in, want := range cases {
		if got := MaskRealName(in); got != want {
			t.Errorf("MaskRealName(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestMaskCertifications_AllowListIsExact 持证去图后剩下的键集合**恰好等于**白名单。
//
// 这是 fail-closed 的结构断言：白名单外的新键（图片、备注、审核状态、原件扫描件…）
// 默认不露面；想让它露面必须显式进白名单。
func TestMaskCertifications_AllowListIsExact(t *testing.T) {
	raw := model.JSONB([]byte(`[
		{"credential_id":7,"cert_no":"N1","expire_date":"2028-01-01","image_urls":["http://x/1.jpg"],
		 "imageUrls":["http://x/2.jpg"],"remark":"内部备注","review_status":"approved","scan_file":"http://x/scan.pdf"}
	]`))

	got := maskCertifications(raw)
	if len(got) != 1 {
		t.Fatalf("条目数 = %d, 期望 1", len(got))
	}
	keys := make([]string, 0, len(got[0]))
	for k := range got[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := append([]string(nil), resumeCertSafeKeys...)
	sort.Strings(want)
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("去图后键集 = %v, 期望恰好等于白名单 %v", keys, want)
	}

	// 敏感值一个都不能留下（哨兵）
	blob, _ := json.Marshal(got)
	for _, sentinel := range []string{"image_urls", "imageUrls", "http://x/1.jpg", "http://x/2.jpg",
		"remark", "内部备注", "review_status", "scan_file", "http://x/scan.pdf"} {
		if strings.Contains(string(blob), sentinel) {
			t.Fatalf("去图结果仍含 %q: %s", sentinel, string(blob))
		}
	}
}

// TestMaskCertifications_FailClosed 脏数据不留半份：非法 JSON 返回空数组。
func TestMaskCertifications_FailClosed(t *testing.T) {
	for _, raw := range []string{``, `not-json`, `{"a":1}`, `[1,2,3]`} {
		got := maskCertifications(model.JSONB([]byte(raw)))
		if got == nil {
			t.Fatalf("入参 %q：应返回空数组而不是 nil（脱敏字段是 JSON 直出，nil 会变成 null）", raw)
		}
		if len(got) != 0 {
			t.Fatalf("入参 %q：应返回空数组，得到 %+v", raw, got)
		}
	}
	if string(maskCertificationsJSON(model.JSONB([]byte(`bad`)))) != "[]" {
		t.Fatal("JSONB 形态的失败出口应为 []")
	}
}

// sentinelCard 每个敏感字段都填独特哨兵：任何一处泄漏都能被下面的断言点名。
func sentinelCard() *model.JobCard {
	ip := func(v int) *int { return &v }
	return &model.JobCard{
		UserID:                1001,
		RealName:              "张三丰",
		ContactPhone:          "SENTINEL_PHONE_13800000001",
		Wechat:                "SENTINEL_WECHAT_zhang",
		Region:                "江苏/苏州/SENTINEL_REGION_TAIL",
		ExpectedPositionID:    ip(3),
		ExpectedPositionExtra: "叉车维修技师",
		ExpectedRegions:       model.JSONB([]byte(`["江苏苏州"]`)),
		SalaryMin:             ip(8000),
		SalaryMax:             ip(12000),
		AvailableIn:           "immediate",
		JobNature:             "fulltime",
		ExperienceYears:       5,
		SelfIntro:             "5 年叉车维修经验",
		ResumeExperiences:     model.JSONB([]byte(`[{"company":"A公司","role":"维修工"}]`)),
		ResumeCertifications: model.JSONB([]byte(
			`[{"credential_id":7,"cert_no":"N1","expire_date":"2028-01-01","image_urls":["SENTINEL_CERT_IMAGE"]}]`)),
		ResumeFileURL: "SENTINEL_RESUME_PDF",
		Photos:        model.JSONB([]byte(`["SENTINEL_PHOTO"]`)),
		Visibility:    "open",
	}
}

// TestResumeProjections_NoSensitiveSentinels 哨兵值行为断言：
// 脱敏卡与打码版 PDF 的**真实取值**都不得含明文哨兵；同时明文投影必须**保留**它们
// （否则「脱敏」会退化成「全丢」，用同一个夹具把两侧都钉住）。
func TestResumeProjections_NoSensitiveSentinels(t *testing.T) {
	card := sentinelCard()

	// 投影 1：L2 脱敏卡
	l2, err := json.Marshal(desensitize(card))
	if err != nil {
		t.Fatalf("marshal L2 卡失败: %v", err)
	}
	for _, sentinel := range []string{"SENTINEL_PHONE_13800000001", "SENTINEL_WECHAT_zhang",
		"SENTINEL_REGION", "SENTINEL_CERT_IMAGE", "SENTINEL_RESUME_PDF", "SENTINEL_PHOTO"} {
		if strings.Contains(string(l2), sentinel) {
			t.Errorf("脱敏卡泄露 %q: %s", sentinel, string(l2))
		}
	}
	if !strings.Contains(string(l2), "张*丰") {
		t.Errorf("脱敏卡姓名应打码为 张*丰: %s", string(l2))
	}

	// 投影 3：打码版 PDF 的取值（版式不在本断言面）
	view := resumePDFViewOf(card)
	viewJSON, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal PDF 取值失败: %v", err)
	}
	for _, sentinel := range []string{"SENTINEL_PHONE_13800000001", "SENTINEL_WECHAT_zhang",
		"SENTINEL_REGION_TAIL", "SENTINEL_CERT_IMAGE", "SENTINEL_RESUME_PDF", "SENTINEL_PHOTO"} {
		if strings.Contains(string(viewJSON), sentinel) {
			t.Errorf("打码版 PDF 取值泄露 %q: %s", sentinel, string(viewJSON))
		}
	}
	if view.RealNameMasked != "张*丰" {
		t.Errorf("PDF 姓名应打码为 张*丰, 得到 %q", view.RealNameMasked)
	}
	// 现居地只到市（哨兵在被截掉的尾段里）
	if view.RegionCity != "江苏/苏州" {
		t.Errorf("现居地应截断到市 江苏/苏州, 得到 %q", view.RegionCity)
	}

	// 投影 2：明文卡（本人 / 已授权方）必须**保留**全部字段——两侧一起钉，防止误改成「全丢」
	plain, err := json.Marshal(toJobCardDTO(card))
	if err != nil {
		t.Fatalf("marshal 明文卡失败: %v", err)
	}
	for _, sentinel := range []string{"SENTINEL_PHONE_13800000001", "SENTINEL_WECHAT_zhang",
		"SENTINEL_REGION", "SENTINEL_CERT_IMAGE", "SENTINEL_RESUME_PDF", "SENTINEL_PHOTO"} {
		if !strings.Contains(string(plain), sentinel) {
			t.Errorf("明文卡应保留 %q（授权门禁在 contact_authz.go，不在这里）: %s", sentinel, string(plain))
		}
	}
	if !strings.Contains(string(plain), "张三丰") {
		t.Error("明文卡姓名不打码")
	}
}

// TestDesensitize_FieldListPinned L2 脱敏卡的字段清单逐字锁定：
// 新增字段必须显式出现在这里（reviewer 一眼看到「它会不会出现在企业浏览面」）。
func TestDesensitize_FieldListPinned(t *testing.T) {
	blob, err := json.Marshal(desensitize(sentinelCard()))
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	want := []string{
		"user_id", "real_name", "real_name_masked", "expected_position_id", "expected_position_extra",
		"expected_regions", "salary_min", "salary_max", "salary_negotiable", "available_in",
		"job_nature", "experience_years", "self_intro", "resume_experiences", "resume_certifications",
		"updated_at",
	}
	if len(got) != len(want) {
		t.Fatalf("L2 卡字段数 = %d, 期望 %d；实际键：%v", len(got), len(want), keysOfRaw(got))
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("L2 卡缺字段 %q（当前：%v）", k, keysOfRaw(got))
		}
	}
	// 明确不该出现的字段（present 即泄露）
	for _, k := range []string{"contact_phone", "wechat", "region", "photos", "resume_file_url"} {
		if _, ok := got[k]; ok {
			t.Errorf("L2 卡不应含字段 %q", k)
		}
	}
}

func keysOfRaw(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestCityLevelRegion_Boundaries 现居地截断到市（PDF 口径；脱敏卡干脆不带现居地）。
func TestCityLevelRegion_Boundaries(t *testing.T) {
	cases := map[string]string{
		"":             "-",
		"江苏/苏州":        "江苏/苏州",
		"江苏/苏州/姑苏区":    "江苏/苏州",
		"北京市":          "北京市",
		"北京市/东城区":      "北京市",
		"上海市/浦东新区":     "上海市",
		"江苏苏州精确地址123号": "江苏省/苏州市",
	}
	for in, want := range cases {
		if got := cityLevelRegion(in); got != want {
			t.Errorf("cityLevelRegion(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
