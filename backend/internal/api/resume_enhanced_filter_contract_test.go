// 契约测试 #492：简历库筛选增强——岗位同源（position_id 精确）、经验「N 年及以上」（>=）、用工性质精确。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestResumeEnhancedFilterContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	pwd, _ := service.HashPassword("pass1234")
	stuA := testutil.SeedStudent(t, db, "stuFiltA", pwd)
	stuB := testutil.SeedStudent(t, db, "stuFiltB", pwd)
	stuC := testutil.SeedStudent(t, db, "stuFiltC", pwd)
	stuD := testutil.SeedStudent(t, db, "stuFiltD", pwd)
	now := time.Now()
	posDriver := model.Position{Code: "filt_driver", Name: "叉车司机", Status: 1}
	posRepair := model.Position{Code: "filt_repair", Name: "叉车维修", Status: 1}
	if err := db.Create(&posDriver).Error; err != nil {
		t.Fatalf("create pos: %v", err)
	}
	if err := db.Create(&posRepair).Error; err != nil {
		t.Fatalf("create pos2: %v", err)
	}
	ip := func(v int) *int { return &v }
	cards := []model.JobCard{
		// A：叉车司机、4 年经验、全职
		{UserID: stuA.ID, RealName: "甲学员", ContactPhone: "13844440001", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), ExpectedPositionID: &posDriver.PositionID, ExperienceYears: 4, JobNature: "fulltime", Visibility: "open", SalaryMin: ip(5000), SalaryMax: ip(8000), CreatedAt: now, UpdatedAt: now},
		// B：叉车维修、2 年经验、兼职
		{UserID: stuB.ID, RealName: "乙学员", ContactPhone: "13844440002", Region: "江苏省/南京市", ExpectedRegions: model.JSONB([]byte(`["江苏省/南京市"]`)), ExpectedPositionID: &posRepair.PositionID, ExperienceYears: 2, JobNature: "parttime", Visibility: "open", SalaryMin: ip(5000), SalaryMax: ip(8000), CreatedAt: now, UpdatedAt: now},
		// C：叉车司机、10 年经验、合同
		{UserID: stuC.ID, RealName: "丙学员", ContactPhone: "13844440003", Region: "浙江省/杭州市", ExpectedRegions: model.JSONB([]byte(`["浙江省/杭州市"]`)), ExpectedPositionID: &posDriver.PositionID, ExperienceYears: 10, JobNature: "contract", Visibility: "open", SalaryMin: ip(5000), SalaryMax: ip(8000), CreatedAt: now, UpdatedAt: now},
		// D：无岗位、1 年经验、全职
		{UserID: stuD.ID, RealName: "丁学员", ContactPhone: "13844440004", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), ExpectedPositionID: nil, ExperienceYears: 1, JobNature: "fulltime", Visibility: "open", SalaryMin: ip(5000), SalaryMax: ip(8000), CreatedAt: now, UpdatedAt: now},
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
	admin := testutil.SeedAdmin(t, db, "adminFilt", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	body := map[string]any{"username": "recruitFilt1", "password": "recruit123", "company_name": "测试企业-filt", "credit_code": "91110000MAfilt", "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": "filt@example.com"}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitFilt1", "password": "recruit123"})
	var login loginResp
	_ = json.Unmarshal(rec2.Body.Bytes(), &login)
	token := login.Data.Token

	type listShape struct {
		Data struct {
			Items []struct {
				UserID int `json:"user_id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	check := func(qs string, want int64, wantUser int) {
		t.Helper()
		rec = doWithToken(t, r, token, http.MethodGet, "/api/recruit/resumes?"+qs, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("查询 %s 应 200, 实际 %d", qs, rec.Code)
		}
		var ls listShape
		if err := json.Unmarshal(rec.Body.Bytes(), &ls); err != nil {
			t.Fatalf("parse %s: %v", qs, err)
		}
		if ls.Data.Total != want {
			t.Fatalf("查询 %s total 应 %d, 实际 %d", qs, want, ls.Data.Total)
		}
		if want > 0 && ls.Data.Items[0].UserID != wantUser {
			t.Fatalf("查询 %s 应命中 user %d, 实际 %v", qs, wantUser, ls.Data.Items)
		}
	}

	// 1. 岗位同源：position_id 精确命中司机（A、C 两人）
	check("position_id="+strconv.Itoa(posDriver.PositionID), 2, stuA.ID)
	check("position_id="+strconv.Itoa(posRepair.PositionID), 1, stuB.ID)
	// 2. 经验「3 年及以上」：A(4)、C(10) 命中，B(2)、D(1) 不中
	check("experience_min=3", 2, stuA.ID)
	check("experience_min=10", 1, stuC.ID)
	check("experience_min=15", 0, 0)
	// 3. 用工性质精确：fulltime 命中 A、D
	check("job_nature=fulltime", 2, stuA.ID)
	check("job_nature=contract", 1, stuC.ID)
	// 4. 叠加：岗位=司机 + 经验>=5 + 合同 → C
	check("position_id="+strconv.Itoa(posDriver.PositionID)+"&experience_min=5&job_nature=contract", 1, stuC.ID)
}
