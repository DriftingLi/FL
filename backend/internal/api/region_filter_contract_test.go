// 契约测试 #486：地区市级化——简历库地区筛选精确匹配（第 2 段市级）。
// 数据契约：expected_regions 元素为两段「省/市」（直辖市一段）。
// 覆盖：筛「苏州市」命中「江苏省/苏州市」不命中「江苏省/南京市」；筛「北京市」命中一段式。
// 存量迁移三样本验证走 pg 契约测试（真实 SQL 迁移执行），本文件用 Go 层筛选行为锁定。
package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestRegionCityFilterContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	pwd, _ := service.HashPassword("pass1234")
	stuSu := testutil.SeedStudent(t, db, "stuRegionSu", pwd)
	stuNan := testutil.SeedStudent(t, db, "stuRegionNan", pwd)
	stuBj := testutil.SeedStudent(t, db, "stuRegionBj", pwd)
	now := time.Now()
	ip := func(v int) *int { return &v }
	// 新契约格式（迁移后）：两段「省/市」
	cards := []model.JobCard{
		{UserID: stuSu.ID, RealName: "苏州学员", ContactPhone: "13811110001", Region: "江苏省/苏州市",
			ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open",
			SalaryMin: ip(6000), SalaryMax: ip(9000), CreatedAt: now, UpdatedAt: now},
		{UserID: stuNan.ID, RealName: "南京学员", ContactPhone: "13811110002", Region: "江苏省/南京市",
			ExpectedRegions: model.JSONB([]byte(`["江苏省/南京市"]`)), Visibility: "open",
			SalaryMin: ip(6000), SalaryMax: ip(9000), CreatedAt: now, UpdatedAt: now},
		{UserID: stuBj.ID, RealName: "北京学员", ContactPhone: "13811110003", Region: "北京市",
			ExpectedRegions: model.JSONB([]byte(`["北京市"]`)), Visibility: "open",
			SalaryMin: ip(6000), SalaryMax: ip(9000), CreatedAt: now, UpdatedAt: now},
	}
	for _, c := range cards {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("create card: %v", err)
		}
	}

	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminRegion", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	body := map[string]any{
		"username": "recruitRegion1", "password": "recruit123", "company_name": "测试企业-region", "credit_code": "91110000MAregion", "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": "region@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitRegion1", "password": "recruit123"})
	if rec2.Code != http.StatusOK {
		t.Fatalf("recruiter login fail %d %s", rec2.Code, rec2.Body.String())
	}
	var login loginResp
	if err := json.Unmarshal(rec2.Body.Bytes(), &login); err != nil {
		t.Fatalf("parse login: %v", err)
	}
	token := login.Data.Token

	type listShape struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				UserID int `json:"user_id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}

	check := func(region string, wantUser int, wantTotal int64) {
		t.Helper()
		rec = doWithToken(t, r, token, http.MethodGet, "/api/recruit/resumes?region="+queryEscape(region), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表应 200, 实际 %d %s", rec.Code, rec.Body.String())
		}
		var ls listShape
		if err := json.Unmarshal(rec.Body.Bytes(), &ls); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if ls.Data.Total != wantTotal {
			t.Fatalf("筛 %s total 应 %d, 实际 %d", region, wantTotal, ls.Data.Total)
		}
		if wantTotal > 0 && ls.Data.Items[0].UserID != wantUser {
			t.Fatalf("筛 %s 应命中 user %d, 实际 %v", region, wantUser, ls.Data.Items)
		}
	}

	// 精确匹配命中：苏州市（全名）
	check("苏州市", stuSu.ID, 1)
	// 短名归一命中：苏州
	check("苏州", stuSu.ID, 1)
	// 直辖市：北京市
	check("北京市", stuBj.ID, 1)
	// 不命中：南京市只出南京（精确匹配非子串——苏州学员不出现）
	check("南京市", stuNan.ID, 1)
}

// queryEscape 查询参数转义（避免 URL 中文问题）。
func queryEscape(s string) string {
	out := make([]byte, 0, len(s))
	const hexs = "0123456789ABCDEF"
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			out = append(out, c)
			continue
		}
		out = append(out, '%', hexs[c>>4], hexs[c&0x0F])
	}
	return string(out)
}
