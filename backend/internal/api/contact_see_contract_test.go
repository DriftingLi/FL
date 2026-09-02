// 契约测试 #487：学员可见企业联系方式——approved 透出（含投递来源）、pending/revoked 不透出、
// 招聘者微信字段 CRUD（管理员创建/编辑/列表）。
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

func TestStudentSeesCompanyContactContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuSeeContact", pwd)
	now := time.Now()
	card := model.JobCard{
		UserID: stu.ID, RealName: "张三丰", ContactPhone: "13800009999", Wechat: "zhang_wx", Region: "江苏省/苏州市",
		ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open",
		ResumeFileURL:        "/static/uploads/resumes/see.pdf",
		Photos:               model.JSONB([]byte(`["http://example.com/work1.jpg"]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"credential_id":1,"cert_no":"N1","expire_date":"2028-01-01","image_urls":["http://example.com/cert1.jpg"]}]`)),
		CreatedAt:            now, UpdatedAt: now,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
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
	admin := testutil.SeedAdmin(t, db, "adminSee", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	// 管理员创建带微信的招聘者
	body := map[string]any{
		"username": "recruitSee1", "password": "recruit123", "company_name": "测试企业-see", "credit_code": "91110000MAsee", "business_scope": "叉车维修", "contact_name": "联系人王", "contact_phone": "13800001111", "contact_email": "see@example.com", "wechat": "company_wx_001",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	if !stringsContains(rec.Body.String(), "company_wx_001") {
		t.Fatalf("创建响应应含 wechat, body=%s", rec.Body.String())
	}
	// 编辑招聘者更新微信
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/1", map[string]any{"username": "recruitSee1", "company_name": "测试企业-see", "credit_code": "91110000MAsee", "business_scope": "叉车维修", "contact_name": "联系人王", "contact_phone": "13800001111", "contact_email": "see@example.com", "wechat": "company_wx_updated"})
	if rec.Code != http.StatusOK {
		t.Fatalf("编辑招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	if !stringsContains(rec.Body.String(), "company_wx_updated") {
		t.Fatalf("编辑响应应含更新后的 wechat, body=%s", rec.Body.String())
	}

	recruiterToken := func() string {
		rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitSee1", "password": "recruit123"})
		if rec2.Code != http.StatusOK {
			t.Fatalf("recruiter login fail %d %s", rec2.Code, rec2.Body.String())
		}
		var resp loginResp
		if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse login: %v", err)
		}
		return resp.Data.Token
	}()
	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")

	// 发起申请（pending）
	rec = doWithToken(t, r, recruiterToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "想联系"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发起申请失败 %d %s", rec.Code, rec.Body.String())
	}
	var createdReq struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createdReq); err != nil {
		t.Fatalf("parse created: %v", err)
	}
	reqID := createdReq.Data.ID

	// pending：学员侧列表不透出电话/邮箱/微信
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	rawPending := rec.Body.String()
	for _, needle := range []string{"13800001111", "see@example.com", "company_wx_updated"} {
		if stringsContains(rawPending, needle) {
			t.Fatalf("pending 不应透出 %q, body=%s", needle, rawPending)
		}
	}

	// 学员同意 → approved：透出企业电话/邮箱/微信
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.FormatInt(reqID, 10)+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("同意失败 %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	rawApproved := rec.Body.String()
	for _, needle := range []string{"13800001111", "see@example.com", "company_wx_updated"} {
		if !stringsContains(rawApproved, needle) {
			t.Fatalf("approved 应透出 %q, body=%s", needle, rawApproved)
		}
	}
	if !stringsContains(rawApproved, "recruiter") {
		t.Fatalf("approved 应含 source=recruiter")
	}

	// #489：授权后 GetContact 透出上传附件/工作照/证书原图（明文面）
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID)+"/contact", nil)
	rawFull := rec.Body.String()
	for _, needle := range []string{"/static/uploads/resumes/see.pdf", "http://example.com/work1.jpg", "http://example.com/cert1.jpg"} {
		if !stringsContains(rawFull, needle) {
			t.Fatalf("授权后 GetContact 应透出 %q, body=%s", needle, rawFull)
		}
	}

	// 撤回 → revoked：联系方式消失
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.FormatInt(reqID, 10)+"/revoke", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("撤回失败 %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	rawRevoked := rec.Body.String()
	for _, needle := range []string{"13800001111", "see@example.com", "company_wx_updated"} {
		if stringsContains(rawRevoked, needle) {
			t.Fatalf("revoked 不应透出 %q, body=%s", needle, rawRevoked)
		}
	}

	// 投递产生授权（source=application）：同样透出
	var pos model.Position
	if err := db.Create(&model.Position{Code: "see_pos", Name: "叉车司机", Status: 1}).Error; err != nil {
		t.Fatalf("create pos: %v", err)
	}
	db.First(&pos, "code = ?", "see_pos")
	job := model.JobPosting{
		RecruiterID: 1, Title: "招聘叉车司机", PositionID: &pos.PositionID, Region: "江苏省/苏州市",
		Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/jobs/"+strconv.Itoa(job.ID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("投递失败 %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	rawApp := rec.Body.String()
	for _, needle := range []string{"13800001111", "see@example.com", "company_wx_updated"} {
		if !stringsContains(rawApp, needle) {
			t.Fatalf("投递来源 approved 应透出 %q, body=%s", needle, rawApp)
		}
	}
	if !stringsContains(rawApp, "application") {
		t.Fatalf("投递来源应含 source=application")
	}
}

func stringsContains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
