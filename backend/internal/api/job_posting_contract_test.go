// 契约测试 #451：职位发布与职位广场（spec #449 T2）。
// 守的边界：
//   - 企业发布/编辑/上下架职位，改别人的职位 403；
//   - 专业方向业务层必填（缺 → 400）；
//   - 单企业活跃职位上限 50（哨兵错误 + 400，禁文案比对）；
//   - 学员侧只见 open 且未强制下架；closed 企业自己仍可见；
//   - L1 延伸：无 token 访问职位列表/详情一律被拒（回归断言焊死）；
//   - 职位详情对企业只露企业名/主营/对外联系人，不露电话/邮箱/信用代码（契约断言缺席）。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
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
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func assertJobPostingContract(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminJob", pwd)
	stuPwd, _ := service.HashPassword("student123")
	stu := testutil.SeedStudent(t, db, "stuJob", stuPwd)

	cfg := &config.Config{JWTSecretKey: "job-posting-secret",
		JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := stuSess.Issue(stu.ID, stu.Username, "hrwai_user")

	createRecruiter := func(username string) (int, string) {
		body := map[string]any{
			"username": username, "password": "recruit123", "company_name": "企业-" + username,
			"credit_code": "91110000" + username, "business_scope": "叉车维修与租赁", "contact_name": "李四-" + username,
			"contact_phone": "13800009999", "contact_email": username + "@example.com",
		}
		rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create recruiter %s: %d %s", username, rec.Code, rec.Body.String())
		}
		var created map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &created)
		data, _ := created["data"].(map[string]any)
		id := int(data["id"].(float64))
		login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": username, "password": "recruit123"})
		if login.Code != http.StatusOK {
			t.Fatalf("login %s: %d %s", username, login.Code, login.Body.String())
		}
		var lr loginResp
		_ = json.Unmarshal(login.Body.Bytes(), &lr)
		return id, lr.Data.Token
	}
	recAID, recAToken := createRecruiter("jobA")
	_, recBToken := createRecruiter("jobB")

	// 种子岗位
	db.Exec("INSERT INTO positions (code, name, status) VALUES (?, ?, ?)", "maint_tech", "叉车维修技师", 1)
	var spID int
	db.Raw("SELECT position_id FROM positions WHERE code = ?", "maint_tech").Scan(&spID)

	// 1. L1 延伸：无 token 访问职位列表/详情一律被拒
	rec := doWithoutToken(t, r, http.MethodGet, "/api/jobs")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token job list should 401, got %d", rec.Code)
	}
	rec = doWithoutToken(t, r, http.MethodGet, "/api/jobs/1")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token job detail should 401, got %d", rec.Code)
	}

	// 2. 缺专业方向 → 400
	badBody := map[string]any{"title": "叉车维修技师"}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", badBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing position should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "岗位") {
		t.Fatalf("missing position message should mention 岗位, got %s", rec.Body.String())
	}

	// 3. 正常发布
	goodBody := map[string]any{
		"title": "叉车维修技师", "position_id": spID, "region": "江苏苏州", "salary_min": 6000, "salary_max": 9000,
		"salary_text": "6-9K", "experience_req": "2年", "description": "负责叉车日常维修保养",
	}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", goodBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create job should 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Code int `json:"code"`
		Data struct {
			ID           int    `json:"id"`
			Status       string `json:"status"`
			PositionName string `json:"position_name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	jobID := created.Data.ID
	if created.Data.Status != "open" {
		t.Fatalf("new job should be open, got %s", created.Data.Status)
	}
	if created.Data.PositionName != "叉车维修技师" {
		t.Fatalf("specialty_name should resolve, got %s", created.Data.PositionName)
	}

	// 4. 改别人的职位 → 403
	rec = doWithToken(t, r, recBToken, http.MethodPut, "/api/recruit/jobs/"+itoa(jobID), goodBody)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("edit others job should 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recBToken, http.MethodPost, "/api/recruit/jobs/"+itoa(jobID)+"/toggle-status", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("toggle others job should 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 5. 学员侧可见该职位，且详情不露电话/邮箱/信用代码
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("student job list should 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "叉车维修技师") {
		t.Fatalf("student list should contain job title, got %s", rec.Body.String())
	}
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs/"+itoa(jobID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("student job detail should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, "13800009999") || strings.Contains(bodyStr, "@example.com") || strings.Contains(bodyStr, "91110000jobA") {
		t.Fatalf("job detail must not leak phone/email/credit_code: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "企业-jobA") || !strings.Contains(bodyStr, "叉车维修与租赁") {
		t.Fatalf("job detail should show company name/scope, got %s", bodyStr)
	}

	// 6. 下架后学员不可见、企业自己仍可见
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs/"+itoa(jobID)+"/toggle-status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle close should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs/"+itoa(jobID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("closed job should 404 for student, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recAToken, http.MethodGet, "/api/recruit/jobs/"+itoa(jobID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("closed job should still visible to owner, got %d", rec.Code)
	}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs/"+itoa(jobID)+"/toggle-status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle reopen should 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 7. 单企业活跃职位上限 50：预置 50 个 open 后第 51 个被拒
	_ = recAID
	jobBody := map[string]any{
		"title": "批量职位", "position_id": spID, "region": "上海", "description": "批量",
	}
	for i := 0; i < 49; i++ {
		jobBody["title"] = "批量职位" + itoa(i)
		rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", jobBody)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create bulk job %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", map[string]any{
		"title": "第51个", "position_id": spID, "region": "上海", "description": "超限",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("51st active job should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "上限") {
		t.Fatalf("active limit message should mention 上限, got %s", rec.Body.String())
	}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs/"+itoa(jobID)+"/toggle-status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle close for limit test: %d", rec.Code)
	}
	rec = doWithToken(t, r, recAToken, http.MethodPost, "/api/recruit/jobs", map[string]any{
		"title": "第51个重发", "position_id": spID, "region": "上海", "description": "超限后重发",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("after closing one should create, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJobPostingContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertJobPostingContract(t, db)
}

func TestJobPostingContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertJobPostingContract(t, db)
}
