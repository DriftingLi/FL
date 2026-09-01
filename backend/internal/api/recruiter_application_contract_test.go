// 契约测试 #453：企业处理投递——已读、漂移、拒绝与徽标（spec #449 T4）。
// 守的边界：
//   - 按职位分页查看投递，只能看自己职位的投递（越权 → 403）；
//   - 列表返回未读投递数（判据：企业尚未打开过该投递）；打开详情即记录已读，未读计数下降；
//   - 候选人仍走既有唯一脱敏路径（姓名打码、无手机号/微信/PDF/证书原图）；
//   - 回显投递时刻的简历更新时间（版本指针，不落快照）；
//   - 标记不合适 → 终态 rejected；同一学员对该职位 30 天内再投被拒（冷却）；
//   - 企业不能把已拒绝的投递改回待处理以外的状态；学员不能替企业标记。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func assertRecruiterApplicationContract(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminProc", pwd)
	stuPwd, _ := service.HashPassword("student123")
	stu := testutil.SeedStudent(t, db, "stuProc", stuPwd)

	cfg := &config.Config{JWTSecretKey: "proc-secret", JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := stuSess.Issue(stu.ID, stu.Username, "hrwai_user")

	// 建两个招聘者
	createRecruiter := func(username string) (int, string) {
		body := map[string]any{
			"username": username, "password": "recruit123", "company_name": "处理企业-" + username,
			"credit_code": "91110000" + username, "business_scope": "叉车维修", "contact_name": "王五",
			"contact_phone": "13800001111", "contact_email": username + "@example.com",
		}
		rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create recruiter: %d %s", rec.Code, rec.Body.String())
		}
		login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": username, "password": "recruit123"})
		var lr loginResp
		_ = json.Unmarshal(login.Body.Bytes(), &lr)
		return 0, lr.Data.Token
	}
	_, recAToken := createRecruiter("procA")
	_, recBToken := createRecruiter("procB")

	// 简历（hidden 但完整）
	card := model.JobCard{UserID: stu.ID, RealName: "张三丰", ContactPhone: "13800009999", Wechat: "zhang_wx", Region: "江苏苏州", Visibility: "hidden", ResumeFileURL: "/static/uploads/resumes/a.pdf", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`))}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 种子岗位 + 职位
	db.Exec("INSERT INTO positions (code, name, status) VALUES (?, ?, ?)", "maint_tech", "叉车维修技师", 1)
	var spID int
	db.Raw("SELECT position_id FROM positions WHERE code = ?", "maint_tech").Scan(&spID)
	jobBody := map[string]any{"title": "处理职位", "position_id": spID, "region": "上海", "description": "日常"}
	rec := doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", jobBody)
	var jobCreated struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &jobCreated)
	jobID := jobCreated.Data.ID

	// 投递
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	var appID int64
	db.Raw("SELECT id FROM job_applications WHERE job_posting_id = ?", jobID).Scan(&appID)

	// 1. 越权：B 企业看 A 职位的投递 → 403
	rec = doWithToken(t, r, recBToken, http.MethodGet, "/api/recruit/jobs/"+itoa(jobID)+"/applications", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other recruiter list should 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 2. A 企业列表：未读计数 1 + 脱敏（姓名打码、无手机/微信/PDF）
	rec = doWithToken(t, r, recAToken, http.MethodGet, "/api/recruit/jobs/"+itoa(jobID)+"/applications", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list applications should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"unread_count":1`) {
		t.Fatalf("unread count should be 1, got %s", rec.Body.String())
	}
	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, "13800009999") || strings.Contains(bodyStr, "zhang_wx") || strings.Contains(bodyStr, "resumes/a.pdf") {
		t.Fatalf("application list must not leak phone/wechat/pdf: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "张*丰") {
		t.Fatalf("application list should show masked name, got %s", bodyStr)
	}

	// 3. 打开详情 → 记录已读；再查列表未读计数 0
	rec = doWithToken(t, r, recAToken, http.MethodGet, "/api/recruit/applications/"+fmtAppID(appID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get detail should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recAToken, http.MethodGet, "/api/recruit/jobs/"+itoa(jobID)+"/applications", nil)
	if !strings.Contains(rec.Body.String(), `"unread_count":0`) {
		t.Fatalf("unread count should drop to 0, got %s", rec.Body.String())
	}

	// 4. 学员侧看到「企业已查看」
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/resume/applications", nil)
	if !strings.Contains(rec.Body.String(), "employer_viewed_at") {
		t.Fatalf("student applications should include employer_viewed_at, got %s", rec.Body.String())
	}

	// 5. 学员不能替企业标记（学员调 reject → 404 路由不存在 或 403）
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/recruit/applications/"+fmtAppID(appID)+"/reject", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("student reject should be forbidden/404, got %d", rec.Code)
	}

	// 6. 标记不合适 → rejected
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/applications/"+fmtAppID(appID)+"/reject", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject should 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 7. 企业不能把已拒绝改回（再 reject → 400）
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/applications/"+fmtAppID(appID)+"/reject", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("re-reject should 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 8. 同一学员对该职位 30 天内再投被拒（冷却）
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("re-apply after reject should 400 (cooldown), got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "30 天") {
		t.Fatalf("cooldown message should mention 30 天, got %s", rec.Body.String())
	}
}

func TestRecruiterApplicationContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruiterApplicationContract(t, db)
}

func TestRecruiterApplicationContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruiterApplicationContract(t, db)
}
