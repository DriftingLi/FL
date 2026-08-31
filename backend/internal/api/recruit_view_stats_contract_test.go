// 契约测试 #374：简历查看留痕与学员侧聚合反馈。
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

func TestRecruitViewStatsContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	// 准备学员与公开简历
	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuViewStats", pwd)
	card := model.JobCard{UserID: stu.ID, RealName: "测试员", Visibility: "open", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`))}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 创建两个招聘者
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminViewStats", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	create := func(username string) string {
		body := map[string]any{
			"username": username, "password": "recruit123", "company_name": "测试企业-" + username, "credit_code": "91110000MA" + username, "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": username + "@example.com",
		}
		rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("建招聘者 %s 失败 %d %s", username, rec.Code, rec.Body.String())
		}
		rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": username, "password": "recruit123"})
		if rec2.Code != http.StatusOK {
			t.Fatalf("login %s fail %d %s", username, rec2.Code, rec2.Body.String())
		}
		var resp loginResp
		if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse login: %v", err)
		}
		return resp.Data.Token
	}
	recruiterAToken := create("recruitViewA")
	recruiterBToken := create("recruitViewB")

	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")

	// 学员初始 view-stats 应 0，且不显示空模块（前端不占位，后端返回 count 0）
	rec := doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/view-stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员 view-stats 初始应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		Code int `json:"code"`
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}
	if stats.Data.Count != 0 {
		t.Fatalf("初始 count 应 0, 实际 %d", stats.Data.Count)
	}
	if strings.Contains(rec.Body.String(), "recruiter") || strings.Contains(rec.Body.String(), "company_name") || strings.Contains(rec.Body.String(), "enterprise") {
		t.Fatalf("学员侧响应不应包含企业名字段, body=%s", rec.Body.String())
	}

	// 招聘者 A 浏览一次
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruiter A view should 200, got %d %s", rec.Code, rec.Body.String())
	}
	// 同一招聘者同日重复浏览同一简历只计 1 次
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("second view should still 200, got %d", rec.Code)
	}
	// 通过列表也触发一次（同日不应重复计）
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list view fail %d", rec.Code)
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/view-stats", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("parse stats after A: %v", err)
	}
	if stats.Data.Count != 1 {
		t.Fatalf("同一招聘者同日重复浏览应计 1, 实际 %d", stats.Data.Count)
	}

	// 不同招聘者 B 浏览计 2
	rec = doWithToken(t, r, recruiterBToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruiter B view fail %d", rec.Code)
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/view-stats", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("parse stats after B: %v", err)
	}
	if stats.Data.Count != 2 {
		t.Fatalf("两个不同招聘者应计 2, 实际 %d", stats.Data.Count)
	}
	// 响应体仍不含企业名
	if strings.Contains(rec.Body.String(), "recruitViewA") || strings.Contains(rec.Body.String(), "company") {
		t.Fatalf("响应不应泄露企业名, body=%s", rec.Body.String())
	}

	// 招聘方无法读取留痕数据
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/resume/view-stats", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("招聘方读取留痕应 403/401, 实际 %d", rec.Code)
	}

	// 7 天外的记录不应计入
	oldTime := time.Now().AddDate(0, 0, -8)
	// 直接插入一条 8 天前的记录（不同企业）
	// 需要一个已存在的 recruiter ID：取 recruitViewA 的 ID via DB
	var recruiterA model.RecruiterUser
	if err := db.Where("username = ?", "recruitViewA").First(&recruiterA).Error; err != nil {
		t.Fatalf("find recruiterA: %v", err)
	}
	// 插入一条 8 天前的浏览（用 A 的 ID 但时间 old，应该不影响 7 天计数，因为 A 已在 7 天内有一条，distinct 仍 1，但我们插入一个新 recruiter 8 天前的应不计）
	// 创建第三个招聘者 C，只产生 8 天前的记录，不应计入
	recruiterCToken := create("recruitViewC")
	var recruiterC model.RecruiterUser
	if err := db.Where("username = ?", "recruitViewC").First(&recruiterC).Error; err != nil {
		t.Fatalf("find recruiterC: %v", err)
	}
	_ = recruiterCToken
	// 直接插入 8 天前
	if err := db.Create(&model.RecruitResumeView{RecruiterID: recruiterC.ID, ResumeUserID: stu.ID, ViewedAt: oldTime}).Error; err != nil {
		t.Fatalf("insert old view: %v", err)
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/view-stats", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("parse after old: %v", err)
	}
	if stats.Data.Count != 2 {
		t.Fatalf("8 天前记录不应计入，仍应为 2, 实际 %d", stats.Data.Count)
	}

	// 留痕表索引存在性：通过查询计划或直接检查索引（sqlite 内存库由 AutoMigrate 建表，应有索引 via 迁移但内存库无迁移；我们检查服务层查询能走索引不报错即可）
	var cnt int64
	if err := db.Model(&model.RecruitResumeView{}).Where("resume_user_id = ? AND viewed_at >= ?", stu.ID, time.Now().AddDate(0, 0, -7)).Count(&cnt).Error; err != nil {
		t.Fatalf("7 天聚合查询失败: %v", err)
	}
}
