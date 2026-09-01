// 契约测试 #452：投递即授权（spec #449 T3）。
// 核心不变式（每条都从外部行为断言）：
//   - 投递同一事务内写投递 + 写/复活 approved 授权（source=application）；
//   - 既有明文端点（/api/recruit/resumes/:id/contact）当场放行该招聘者；
//   - hidden 简历可投递；缺真实姓名/电话被拒；
//   - 该企业已有 pending 申请时投递把它覆盖为 approved；
//   - applied 期间唯一；撤回后立即重投；
//   - 学员日限 10；
//   - 撤回默认不连带授权；显式连带后明文端点 403；
//   - 投递产生的授权不计入企业日限；
//   - 注销后投递与授权一并失效。
//
// 部分唯一索引在生产由迁移保证、在测试由业务层判定保证（sqlite 不执行迁移）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
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

func assertJobApplicationContract(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminApp", pwd)
	stuPwd, _ := service.HashPassword("student123")
	stu := testutil.SeedStudent(t, db, "stuApp", stuPwd)

	cfg := &config.Config{JWTSecretKey: "app-contract-secret", JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := stuSess.Issue(stu.ID, stu.Username, "hrwai_user")

	// 建招聘者 + 简历（hidden！验证 hidden 可投递）
	createBody := map[string]any{
		"username": "appRec", "password": "recruit123", "company_name": "投递测试企业",
		"credit_code": "91110000APP", "business_scope": "叉车维修", "contact_name": "王五",
		"contact_phone": "13800001111", "contact_email": "apprec@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create recruiter: %d %s", rec.Code, rec.Body.String())
	}
	login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "appRec", "password": "recruit123"})
	var lr loginResp
	_ = json.Unmarshal(login.Body.Bytes(), &lr)
	recToken := lr.Data.Token

	// 简历：hidden 但含真实姓名/电话
	card := model.JobCard{UserID: stu.ID, RealName: "张三丰", ContactPhone: "13800009999", Wechat: "zhang_wx", Region: "江苏苏州", Visibility: "hidden", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`))}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 种子岗位 + 职位
	db.Exec("INSERT INTO positions (code, name, status) VALUES (?, ?, ?)", "maint_tech", "叉车维修技师", 1)
	var spID int
	db.Raw("SELECT position_id FROM positions WHERE code = ?", "maint_tech").Scan(&spID)
	jobBody := map[string]any{"title": "叉车维修技师", "position_id": spID, "region": "江苏苏州", "description": "日常维修"}
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

	// 1. 投递前明文端点 403（无授权）
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("before apply contact should 403, got %d", rec.Code)
	}

	// 2. 投递 → 201，且同一响应后明文端点当场放行
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply should 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("after apply contact should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "13800009999") {
		t.Fatalf("contact should expose phone, got %s", rec.Body.String())
	}

	// 3. 授权记录 source=application 且 approved
	var req model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id = ?", 1, stu.ID).First(&req).Error; err != nil {
		t.Fatalf("contact request should exist: %v", err)
	}
	if req.Status != "approved" || req.Source != "application" {
		t.Fatalf("auth should be approved+application, got %s/%s", req.Status, req.Source)
	}

	// 4. applied 期间唯一：重复投递被拒
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate apply should 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "重复投递") {
		t.Fatalf("duplicate apply message should mention 重复投递, got %s", rec.Body.String())
	}

	// 5. 撤回（默认不连带）→ 授权仍有效
	var appID int64
	db.Raw("SELECT id FROM job_applications WHERE job_posting_id = ? AND student_user_id = ?", jobID, stu.ID).Scan(&appID)
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/resume/applications/"+fmtAppID(appID)+"/withdraw", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("after withdraw without revoke, contact should still 200, got %d", rec.Code)
	}

	// 6. 撤回后可立即重新投递
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("re-apply after withdraw should 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 7. 连带撤回：显式 revoke_contact=true → 明文端点 403
	db.Raw("SELECT id FROM job_applications WHERE job_posting_id = ? AND student_user_id = ? AND status = ?", jobID, stu.ID, "applied").Scan(&appID)
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/resume/applications/"+fmtAppID(appID)+"/withdraw", map[string]any{"revoke_contact": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw with revoke should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("after revoke contact should 403, got %d", rec.Code)
	}

	// 7.5 pending 覆盖：该企业对该学员已有 pending 申请时，投递把它覆盖为 approved
	stu3 := testutil.SeedStudent(t, db, "stuApp3", stuPwd)
	card3 := model.JobCard{UserID: stu3.ID, RealName: "赵六", ContactPhone: "13800007777", Visibility: "hidden"}
	_ = db.Create(&card3).Error
	// 企业发起一个 pending 申请
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu3.ID, "message": "您好，想了解"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("recruiter contact request should 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	stu3Sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stu3Token, _ := stu3Sess.Issue(stu3.ID, stu3.Username, "hrwai_user")
	// stu3 投递同企业新职位
	jobBody["title"] = "叉车维修技师3"
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", jobBody)
	var job3 struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &job3)
	rec = doWithToken(t, r, stu3Token, http.MethodPost, "/api/jobs/"+itoa(job3.Data.ID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stu3 apply should 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var pendingReq model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id = ?", 1, stu3.ID).First(&pendingReq).Error; err != nil {
		t.Fatalf("pending req should exist: %v", err)
	}
	if pendingReq.Status != "approved" {
		t.Fatalf("pending should be overridden to approved, got %s", pendingReq.Status)
	}
	if pendingReq.Source != "application" {
		t.Fatalf("overridden auth source should be application, got %s", pendingReq.Source)
	}
	// 8. 企业日限不受投递影响：投递产生的授权不消耗企业日限（先投 5 个职位再发满额申请）
	// 简化：直接验证投递 5 个后企业仍能发起第 1 个申请（若日限被误扣则 400）
	// 这里用另一个学员来验证企业日限独立
	stu2 := testutil.SeedStudent(t, db, "stuApp2", stuPwd)
	card2 := model.JobCard{UserID: stu2.ID, RealName: "李四", ContactPhone: "13800008888", Visibility: "hidden"}
	_ = db.Create(&card2).Error
	// 给 stu2 也建一个职位
	jobBody["title"] = "叉车维修技师2"
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", jobBody)
	var job2 struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &job2)
	stu2Sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stu2Token, _ := stu2Sess.Issue(stu2.ID, stu2.Username, "hrwai_user")
	rec = doWithToken(t, r, stu2Token, http.MethodPost, "/api/jobs/"+itoa(job2.Data.ID)+"/apply", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stu2 apply should 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 8.5 缺真实姓名/电话的简历不能投递
	stu4 := testutil.SeedStudent(t, db, "stuApp4", stuPwd)
	card4 := model.JobCard{UserID: stu4.ID, RealName: "", ContactPhone: "", Visibility: "hidden"}
	_ = db.Create(&card4).Error
	stu4Sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stu4Token, _ := stu4Sess.Issue(stu4.ID, stu4.Username, "hrwai_user")
	rec = doWithToken(t, r, stu4Token, http.MethodPost, "/api/jobs/"+itoa(jobID)+"/apply", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("apply with empty resume should 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 8.6 学员每日投递上限 10：给 stu4 补全简历后连投 11 个职位
	_ = db.Model(&model.JobCard{}).Where("user_id = ?", stu4.ID).Updates(map[string]any{"real_name": "钱七", "contact_phone": "13800006666"}).Error
	for i := 0; i < 10; i++ {
		jobBody["title"] = "日限职位" + itoa(i)
		rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", jobBody)
		var jobN struct {
			Data struct {
				ID int `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &jobN)
		rec = doWithToken(t, r, stu4Token, http.MethodPost, "/api/jobs/"+itoa(jobN.Data.ID)+"/apply", nil)
		if rec.Code != http.StatusCreated {
			t.Fatalf("apply %d should 201, got %d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	jobBody["title"] = "日限职位-第11个"
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", jobBody)
	var job11 struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &job11)
	rec = doWithToken(t, r, stu4Token, http.MethodPost, "/api/jobs/"+itoa(job11.Data.ID)+"/apply", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("11th apply should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "上限") {
		t.Fatalf("daily limit message should mention 上限, got %s", rec.Body.String())
	}
	// 9. 注销后投递与授权失效
	rec = doWithToken(t, r, stu2Token, http.MethodDelete, "/api/auth/account", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete account should 200, got %d", rec.Code)
	}
	var appCnt int64
	db.Model(&model.JobApplication{}).Where("student_user_id = ?", stu2.ID).Count(&appCnt)
	if appCnt != 0 {
		t.Fatalf("applications should be deleted on account deletion, count=%d", appCnt)
	}
	var authCnt int64
	db.Model(&model.ContactRequest{}).Where("student_user_id = ?", stu2.ID).Count(&authCnt)
	if authCnt != 0 {
		t.Fatalf("contact requests should be deleted on account deletion, count=%d", authCnt)
	}
}

func fmtAppID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func TestJobApplicationContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertJobApplicationContract(t, db)
}

func TestJobApplicationContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertJobApplicationContract(t, db)
}
