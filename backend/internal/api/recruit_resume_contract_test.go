// 契约测试 #373：招聘端脱敏简历列表与筛选。
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

func TestRecruitResumesContract_Full(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	spec1 := model.Position{Code: "spec_forklift", Name: "叉车维修", Status: 1}
	spec2 := model.Position{Code: "spec_electric", Name: "电工", Status: 1}
	if err := db.Create(&spec1).Error; err != nil {
		t.Fatalf("create spec1: %v", err)
	}
	if err := db.Create(&spec2).Error; err != nil {
		t.Fatalf("create spec2: %v", err)
	}
	cred1 := model.Credential{Code: "cred_n1", Name: "叉车N1", Category: "special_operation", Status: 1}
	cred2 := model.Credential{Code: "cred_n2", Name: "焊工", Category: "special_operation", Status: 1}
	if err := db.Create(&cred1).Error; err != nil {
		t.Fatalf("create cred1: %v", err)
	}
	if err := db.Create(&cred2).Error; err != nil {
		t.Fatalf("create cred2: %v", err)
	}

	now := time.Now()
	pwd, _ := service.HashPassword("pass1234")
	stu1 := testutil.SeedStudent(t, db, "stuRecruit1", pwd)
	stu2 := testutil.SeedStudent(t, db, "stuRecruit2", pwd)
	stu3 := testutil.SeedStudent(t, db, "stuRecruitHidden", pwd)
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-30 * time.Minute)
	ip := func(v int) *int { return &v }
	card1 := model.JobCard{
		UserID: stu1.ID, RealName: "张三丰", ContactPhone: "13800000001", Wechat: "zhang_wx", Region: "江苏苏州精确地址123号",
		ExpectedPositionID: &spec1.PositionID, ExpectedPositionExtra: "叉车维修", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`)),
		SalaryMin: ip(8000), SalaryMax: ip(12000), SalaryNegotiable: false,
		AvailableIn: "immediate", JobNature: "fulltime", ExperienceYears: 5, SelfIntro: "5年经验自我介绍",
		ResumeExperiences:    model.JSONB([]byte(`[{"company":"A公司","role":"维修工","start_month":"2020-01","end_month":"2023-01","desc":"修叉车"}]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"credential_id":` + strconv.Itoa(cred1.ID) + `,"cert_no":"CERT001","expire_date":"2028-01-01","image_urls":["http://example.com/cert1.jpg"]}]`)),
		ResumeFileURL:        "/static/uploads/resumes/stu1.pdf", Photos: model.JSONB([]byte(`[]`)), Visibility: "open",
		CreatedAt: older, UpdatedAt: older,
	}
	card2 := model.JobCard{
		UserID: stu2.ID, RealName: "李四", ContactPhone: "13900000002", Wechat: "li_wx", Region: "浙江杭州精确地址",
		ExpectedPositionID: &spec2.PositionID, ExpectedPositionExtra: "电工", ExpectedRegions: model.JSONB([]byte(`["浙江杭州"]`)),
		SalaryMin: ip(10000), SalaryMax: ip(15000), SalaryNegotiable: false,
		AvailableIn: "1w", JobNature: "parttime", ExperienceYears: 2, SelfIntro: "2年电工经验",
		ResumeExperiences:    model.JSONB([]byte(`[{"company":"B公司","role":"电工","start_month":"2021-01","end_month":"2024-01","desc":"电气"}]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"credential_id":` + strconv.Itoa(cred2.ID) + `,"cert_no":"CERT002","expire_date":"2029-01-01","image_urls":["http://example.com/cert2.jpg"]}]`)),
		ResumeFileURL:        "/static/uploads/resumes/stu2.pdf", Photos: model.JSONB([]byte(`[]`)), Visibility: "open",
		CreatedAt: newer, UpdatedAt: newer,
	}
	cardHidden := model.JobCard{
		UserID: stu3.ID, RealName: "王五", ContactPhone: "13700000003", Wechat: "wang_wx", Region: "上海精确地址",
		ExpectedPositionID: &spec1.PositionID, ExpectedPositionExtra: "叉车维修", ExpectedRegions: model.JSONB([]byte(`["上海"]`)),
		SalaryMin: ip(9000), SalaryMax: ip(13000), AvailableIn: "immediate", ExperienceYears: 3, SelfIntro: "隐藏简历",
		ResumeExperiences: model.JSONB([]byte(`[]`)), ResumeCertifications: model.JSONB([]byte(`[]`)), Visibility: "hidden",
		CreatedAt: now, UpdatedAt: now,
	}
	for _, c := range []model.JobCard{card1, card2, cardHidden} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("create card %d: %v", c.UserID, err)
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
	admin := testutil.SeedAdmin(t, db, "adminRecruit", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	// 创建招聘者并登录
	body := map[string]any{
		"username": "recruit373a", "password": "recruit123", "company_name": "测试企业-recruit373a", "credit_code": "91110000MArecruit373a", "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": "recruit373a@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	var created recruiterCreateResp
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse create recruiter: %v", err)
	}
	recruiterToken := func() string {
		rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruit373a", "password": "recruit123"})
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
	studentToken, _ := studentSess.Issue(stu1.ID, stu1.Account, "hrwai_user")

	// 401 未登录
	rec = doWithoutToken(t, r, http.MethodGet, "/api/recruit/resumes")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401, 实际 %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithoutToken(t, r, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录详情应 401, 实际 %d", rec.Code)
	}
	// 403 学员访问招聘端
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("学员访问招聘端应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}
	// 403 招聘者访问学员侧
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/forum/topics", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("招聘者访问论坛应 403, 实际 %d", rec.Code)
	}

	// 正常列表：只返回 open 的两张，hidden 不可见
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者列表应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]json.RawMessage `json:"items"`
			Total int64                        `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if listResp.Data.Total != 2 {
		t.Fatalf("应返回 2 条 open, 实际 total=%d", listResp.Data.Total)
	}
	if len(listResp.Data.Items) != 2 {
		t.Fatalf("items len 应 2, 实际 %d", len(listResp.Data.Items))
	}
	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, strconv.Itoa(stu3.ID)) && strings.Contains(bodyStr, "王五") {
		t.Fatalf("hidden 卡不应出现在列表")
	}
	// 默认排序 updated_at DESC：stu2 (newer) 应在前
	rec2 := doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes", nil)
	var listTyped struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				UserID    int    `json:"user_id"`
				UpdatedAt string `json:"updated_at"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &listTyped); err != nil {
		t.Fatalf("parse typed: %v", err)
	}
	if listTyped.Data.Items[0].UserID != stu2.ID || listTyped.Data.Items[1].UserID != stu1.ID {
		t.Fatalf("默认排序应 updated_at DESC, 实际 order %v", listTyped.Data.Items)
	}

	// 脱敏负向断言
	rawList := rec.Body.String()
	for _, needle := range []string{"13800000001", "13900000002", "zhang_wx", "li_wx", "/static/uploads/resumes", "cert1.jpg", "cert2.jpg", "江苏苏州精确地址123号", "张三丰", "李四"} {
		if strings.Contains(rawList, needle) {
			t.Fatalf("脱敏列表不应包含 %q, 实际 body=%s", needle, rawList)
		}
	}
	if !strings.Contains(rawList, "张*") && !strings.Contains(rawList, "张*丰") {
		if !strings.Contains(rawList, "李*") {
			t.Logf("body=%s", rawList)
			t.Fatalf("脱敏列表应含打码姓名")
		}
	}
	if strings.Contains(rawList, "image_urls") || strings.Contains(rawList, "imageUrls") {
		t.Fatalf("脱敏后 cert 不应含 image_urls, body=%s", rawList)
	}

	// 详情与列表共用同一脱敏路径
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("详情应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	rawDetail := rec.Body.String()
	for _, needle := range []string{"13800000001", "zhang_wx", "/static/uploads/resumes", "cert1.jpg", "江苏苏州精确地址123号", "张三丰"} {
		if strings.Contains(rawDetail, needle) {
			t.Fatalf("脱敏详情不应包含 %q", needle)
		}
	}
	if strings.Contains(rawDetail, "image_urls") {
		t.Fatalf("详情 cert 不应含 image_urls")
	}
	// hidden 详情应 404
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu3.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden 详情应 404, 实际 %d %s", rec.Code, rec.Body.String())
	}

	// 筛选轴：region
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?region=浙江杭州", nil)
	var filt struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				UserID int `json:"user_id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse filter region: %v", err)
	}
	if filt.Data.Total != 1 || filt.Data.Items[0].UserID != stu2.ID {
		t.Fatalf("region 筛选错误 total=%d items=%v", filt.Data.Total, filt.Data.Items)
	}
	// specialty_id
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?position_id="+strconv.Itoa(spec1.PositionID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse spec filter: %v", err)
	}
	if filt.Data.Total != 1 || filt.Data.Items[0].UserID != stu1.ID {
		t.Fatalf("position 筛选错误 %v", filt.Data)
	}
	// credential_id
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?credential_id="+strconv.Itoa(cred1.ID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse cred filter: %v", err)
	}
	if filt.Data.Total != 1 || filt.Data.Items[0].UserID != stu1.ID {
		t.Fatalf("credential 筛选错误 %v", filt.Data)
	}
	// available_in
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?available_in=immediate", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse avail filter: %v", err)
	}
	if filt.Data.Total != 1 {
		t.Fatalf("available_in 筛选应 1, 实际 %d", filt.Data.Total)
	}
	// experience_years
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?experience_years=5", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse exp filter: %v", err)
	}
	if filt.Data.Total != 1 || filt.Data.Items[0].UserID != stu1.ID {
		t.Fatalf("experience_years 筛选错误 %v", filt.Data)
	}
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?salary_min=9000", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("salary_min filter 应 200, 实际 %d", rec.Code)
	}
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes?region=不存在的地区", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &filt); err != nil {
		t.Fatalf("parse empty filter: %v", err)
	}
	if filt.Data.Total != 0 {
		t.Fatalf("无匹配筛选应 0, 实际 %d", filt.Data.Total)
	}

	// 更新后读最新
	newIntro := "更新后的自我介绍-最新值"
	if err := db.Model(&model.JobCard{}).Where("user_id = ?", stu1.ID).Updates(map[string]any{"self_intro": newIntro, "updated_at": time.Now()}).Error; err != nil {
		t.Fatalf("update card: %v", err)
	}
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID), nil)
	if !strings.Contains(rec.Body.String(), newIntro) {
		t.Fatalf("更新后详情应读到新值 %q, 实际 %s", newIntro, rec.Body.String())
	}
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes", nil)
	var afterList struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				UserID int `json:"user_id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &afterList); err != nil {
		t.Fatalf("parse after update: %v", err)
	}
	if afterList.Data.Items[0].UserID != stu1.ID {
		t.Fatalf("更新后排序应 stu1 在前, 实际 %v", afterList.Data.Items)
	}

	// 审计留痕
	var viewCnt int64
	if err := db.Model(&model.RecruitResumeView{}).Count(&viewCnt).Error; err != nil {
		t.Fatalf("count views: %v", err)
	}
	if viewCnt == 0 {
		t.Fatalf("读取后应有审计记录, 实际 0")
	}
}
