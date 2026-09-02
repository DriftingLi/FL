// 契约测试 #489：简历库交换状态可视——列表/详情 contact_state（none/pending/approved + 来源标注）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestResumeContactStateContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	pwd, _ := service.HashPassword("pass1234")
	stuNone := testutil.SeedStudent(t, db, "stuCsNone", pwd)
	stuPending := testutil.SeedStudent(t, db, "stuCsPending", pwd)
	stuApproved := testutil.SeedStudent(t, db, "stuCsApproved", pwd)
	stuAppSource := testutil.SeedStudent(t, db, "stuCsApp", pwd)
	now := time.Now()
	cards := []model.JobCard{
		{UserID: stuNone.ID, RealName: "无状态", ContactPhone: "13833330001", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open", CreatedAt: now, UpdatedAt: now},
		{UserID: stuPending.ID, RealName: "待确认", ContactPhone: "13833330002", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open", CreatedAt: now, UpdatedAt: now},
		{UserID: stuApproved.ID, RealName: "已授权", ContactPhone: "13833330003", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open", CreatedAt: now, UpdatedAt: now},
		{UserID: stuAppSource.ID, RealName: "投递授权", ContactPhone: "13833330004", Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open", CreatedAt: now, UpdatedAt: now},
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
	admin := testutil.SeedAdmin(t, db, "adminCs", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	body := map[string]any{"username": "recruitCs1", "password": "recruit123", "company_name": "测试企业-cs", "credit_code": "91110000MAcs", "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": "cs@example.com"}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitCs1", "password": "recruit123"})
	var login loginResp
	_ = json.Unmarshal(rec2.Body.Bytes(), &login)
	token := login.Data.Token

	// pending：向 stuPending 发申请
	rec = doWithToken(t, r, token, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stuPending.ID, "message": "请考虑"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发申请失败 %d %s", rec.Code, rec.Body.String())
	}
	// approved（recruiter 来源）：向 stuApproved 发申请并同意
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stuApproved.ID, "message": "请联系"})
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	stuToken := func(id int, acct string) string {
		tk, _ := studentSess.Issue(id, acct, "hrwai_user")
		return tk
	}
	rec = doWithToken(t, r, stuToken(stuApproved.ID, stuApproved.Account), http.MethodPost, "/api/resume/contact-requests/"+strconv.FormatInt(created.Data.ID, 10)+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("同意失败 %d %s", rec.Code, rec.Body.String())
	}
	// approved（application 来源）：投递 stuAppSource
	pos := model.Position{Code: "cs_pos", Name: "叉车维修", Status: 1}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("create pos: %v", err)
	}
	var p2 model.Position
	db.First(&p2, "code = ?", "cs_pos")
	job := model.JobPosting{RecruiterID: 1, Title: "岗位", PositionID: &p2.PositionID, Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	rec = doWithToken(t, r, stuToken(stuAppSource.ID, stuAppSource.Account), http.MethodPost, "/api/jobs/"+strconv.Itoa(job.ID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("投递失败 %d %s", rec.Code, rec.Body.String())
	}

	// 列表断言四态
	rec = doWithToken(t, r, token, http.MethodGet, "/api/recruit/resumes", nil)
	var listResp struct {
		Data struct {
			Items []struct {
				UserID        int    `json:"user_id"`
				ContactState  string `json:"contact_state"`
				ContactSource string `json:"contact_source"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	st := map[int]struct{ s, src string }{}
	for _, it := range listResp.Data.Items {
		st[it.UserID] = struct{ s, src string }{it.ContactState, it.ContactSource}
	}
	if v := st[stuNone.ID]; v.s != "" {
		t.Fatalf("none 学员应无 contact_state, 实际 %+v", v)
	}
	if v := st[stuPending.ID]; v.s != "pending" {
		t.Fatalf("pending 学员应 pending, 实际 %+v", v)
	}
	if v := st[stuApproved.ID]; v.s != "approved" || v.src != "recruiter" {
		t.Fatalf("approved 学员应 approved/recruiter, 实际 %+v", v)
	}
	if v := st[stuAppSource.ID]; v.s != "approved" || v.src != "application" {
		t.Fatalf("投递来源应 approved/application, 实际 %+v", v)
	}

	// 详情也带状态
	rec = doWithToken(t, r, token, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stuPending.ID), nil)
	if !strings.Contains(rec.Body.String(), `"contact_state":"pending"`) && !strings.Contains(rec.Body.String(), `"contact_state": "pending"`) {
		t.Fatalf("详情应含 contact_state=pending, body=%s", rec.Body.String())
	}
}
