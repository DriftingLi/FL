// 契约测试 #454：职位内容治理——举报与强制下架（spec #449 T5）。
// 守的边界：
//   - 学员举报职位；同一学员对同一职位唯一，重复举报被合并而非堆叠；
//   - 举报记录归属招聘域自己的存储（job_reports），不挂论坛举报表；
//   - 管理端只读巡检：职位列表（可按企业筛）+ 举报队列 + 处置动作进审计日志；
//   - 强制下架带原因；学员侧立即不可见（列表与详情都取不到）；
//   - 被强制下架的职位企业不能自行重新上架（哨兵错误 + 400）；
//   - 企业侧职位列表显示被下架状态与原因；
//   - 举报可标记已处理，处理后不再出现在待处理队列。
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

func assertJobReportContract(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminReport", pwd)
	stuPwd, _ := service.HashPassword("student123")
	stu := testutil.SeedStudent(t, db, "stuReport", stuPwd)

	cfg := &config.Config{JWTSecretKey: "report-secret", JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := stuSess.Issue(stu.ID, stu.Username, "hrwai_user")

	// 建招聘者 + 职位
	createBody := map[string]any{
		"username": "repRec", "password": "recruit123", "company_name": "治理测试企业",
		"credit_code": "91110000REP", "business_scope": "叉车维修", "contact_name": "王五",
		"contact_phone": "13800001111", "contact_email": "repr@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create recruiter: %d %s", rec.Code, rec.Body.String())
	}
	login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "repRec", "password": "recruit123"})
	var lr loginResp
	_ = json.Unmarshal(login.Body.Bytes(), &lr)
	recToken := lr.Data.Token

	db.Exec("INSERT INTO specialty (code, name, status) VALUES (?, ?, ?)", "maint", "叉车维修", 1)
	var spID int
	db.Raw("SELECT specialty_id FROM specialty WHERE code = ?", "maint").Scan(&spID)
	jobBody := map[string]any{"title": "可疑职位", "specialty_id": spID, "region": "上海", "description": "交培训费包分配"}
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", jobBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create job: %d %s", rec.Code, rec.Body.String())
	}
	var jobCreated struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &jobCreated)
	jobID := jobCreated.Data.ID

	// 1. 学员举报 → 201
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/report", map[string]any{"reason": "交培训费包分配，涉嫌诈骗"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("report should 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 2. 重复举报 → 合并不堆叠（仍只有一条记录，原因更新）
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/report", map[string]any{"reason": "更新后的原因"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("duplicate report should 201 (merged), got %d body=%s", rec.Code, rec.Body.String())
	}
	var repCount int64
	db.Model(&model.JobReport{}).Where("job_posting_id = ? AND student_user_id = ?", jobID, stu.ID).Count(&repCount)
	if repCount != 1 {
		t.Fatalf("duplicate report should merge into one row, count=%d", repCount)
	}
	var rep model.JobReport
	db.Where("job_posting_id = ? AND student_user_id = ?", jobID, stu.ID).First(&rep)
	if rep.Reason != "更新后的原因" {
		t.Fatalf("merged report reason should be updated, got %s", rep.Reason)
	}

	// 3. 举报记录归属招聘域存储（job_reports 表），不挂论坛举报表
	var forumRepCount int64
	db.Model(&model.ForumReport{}).Count(&forumRepCount)
	if forumRepCount != 0 {
		t.Fatalf("report must not touch forum_reports, count=%d", forumRepCount)
	}

	// 4. 管理端举报队列可见该举报
	rec = doWithToken(t, r, adminToken, http.MethodGet, "/api/admin/job-reports", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin reports list should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "可疑职位") {
		t.Fatalf("admin reports should contain job title, got %s", rec.Body.String())
	}

	// 5. 强制下架带原因 → 200；学员侧列表/详情立即不可见
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/jobs/"+itoa(jobID)+"/force-offline", map[string]any{"reason": "涉嫌虚假承诺"})
	if rec.Code != http.StatusOK {
		t.Fatalf("force offline should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "涉嫌虚假承诺") {
		t.Fatalf("force offline response should contain reason, got %s", rec.Body.String())
	}
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs", nil)
	if strings.Contains(rec.Body.String(), "可疑职位") {
		t.Fatalf("student list must not contain forced-offline job, got %s", rec.Body.String())
	}
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs/"+itoa(jobID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("student detail of forced-offline job should 404, got %d", rec.Code)
	}

	// 6. 企业不能自行重新上架（哨兵错误 + 400）
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs/"+itoa(jobID)+"/toggle-status", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("recruiter re-open forced-offline job should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "强制下架") {
		t.Fatalf("re-open message should mention 强制下架, got %s", rec.Body.String())
	}

	// 7. 企业侧职位列表显示被下架状态与原因
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruiter job list should 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "可疑职位") || !strings.Contains(rec.Body.String(), "涉嫌虚假承诺") {
		t.Fatalf("recruiter list should show job + offline reason, got %s", rec.Body.String())
	}

	// 8. 管理端职位巡检列表（含强制下架）
	rec = doWithToken(t, r, adminToken, http.MethodGet, "/api/admin/jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin jobs list should 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "可疑职位") {
		t.Fatalf("admin jobs list should include forced-offline job, got %s", rec.Body.String())
	}

	// 9. 标记举报已处理 → 不再出现在待处理队列
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/job-reports/"+fmtAppID(rep.ID)+"/handle", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("mark handled should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, adminToken, http.MethodGet, "/api/admin/job-reports", nil)
	if strings.Contains(rec.Body.String(), "可疑职位") {
		t.Fatalf("handled report should not be in pending queue, got %s", rec.Body.String())
	}
}

func TestJobReportContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertJobReportContract(t, db)
}

func TestJobReportContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertJobReportContract(t, db)
}
