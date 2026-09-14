// 契约 codegen 片五（#963）的**顶层 key 断言**：本片给 27 个端点补/纠了 data 指认、
// 给 3 个缺席端点补了完整注解块，这里把「注解声明的形状」与「handler 实际返回的 key 集合」
// 钉在一起（片一 #952 的先例：只断言顶层 key，不做实现细节断言）。
//
// 覆盖：recruit 工作区（/recruit/me、简历库列表/详情、明文联系方式）与 resume 的标量端点
// （/resume/view-stats）。其余端点已由既有契约测试守住形状（job_posting_contract_test.go、
// job_application_contract_test.go、recruiter_application_contract_test.go、
// job_card_contract_test.go、contact_contract_test.go、recruit_resume_contract_test.go、
// resume_pdf_contract_test.go 等），本片不重复断言。
package api

import (
	"encoding/json"
	"net/http"
	"sort"
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

// dataKeys 取统一信封里 data 对象的顶层 key 集合（data 不是对象时为 nil）。
func slice5DataKeys(t *testing.T, body []byte) []string {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("解析信封失败: %v body=%s", err, string(body))
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &m); err != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertSlice5Keys(t *testing.T, label string, body []byte, want ...string) {
	t.Helper()
	got := slice5DataKeys(t, body)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("%s data key 数量 = %d, 期望 %d\n实际: %v\n期望: %v\nbody=%s", label, len(got), len(want), got, want, string(body))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s data key 不一致\n实际: %v\n期望: %v\nbody=%s", label, got, want, string(body))
		}
	}
}

func assertSlice5MissingKeys(t *testing.T, label string, body []byte, absent ...string) {
	t.Helper()
	got := slice5DataKeys(t, body)
	set := map[string]bool{}
	for _, k := range got {
		set[k] = true
	}
	for _, k := range absent {
		if set[k] {
			t.Fatalf("%s 不应含 key %q（脱敏边界）: %v", label, k, got)
		}
	}
}

// assertSlice5Shape 与既有契约测试同形：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
func assertSlice5Shape(t *testing.T, db *gorm.DB) {
	t.Helper()
	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuSlice5", pwd)
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminSlice5", adminPwd)

	cfg := &config.Config{
		JWTSecretKey:    "slice5-contract-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	adminToken, _ := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(admin.AdminID, admin.Username, "admin")
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", map[string]any{
		"username": "recruitSlice5", "password": "recruit123", "company_name": "片五企业",
		"credit_code": "91110000MAslice5", "business_scope": "叉车维修", "contact_name": "王工",
		"contact_phone": "13800002222", "contact_email": "slice5@example.com",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitSlice5", "password": "recruit123"})
	var lr loginResp
	if err := json.Unmarshal(login.Body.Bytes(), &lr); err != nil {
		t.Fatalf("解析 recruit 登录失败: %v", err)
	}
	recToken := lr.Data.Token
	stuToken, _ := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(stu.ID), stu.Account, "hrwai_user")

	now := time.Now()
	card := model.JobCard{
		UserID: stu.ID, RealName: "张三丰", ContactPhone: "13800003333", Wechat: "zhang_wx",
		Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)),
		ResumeExperiences:    model.JSONB([]byte(`[{"company":"A公司","role":"维修工"}]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"credential_id":1,"cert_no":"N1","image_urls":["http://example.com/c1.jpg"]}]`)),
		Photos:               model.JSONB([]byte(`["http://example.com/w1.jpg"]`)),
		ResumeFileURL:        "/static/uploads/resumes/slice5.pdf",
		Visibility:           "open", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("创建简历卡失败: %v", err)
	}

	// ===== 1. GET /api/recruit/me：注解 data=service.RecruitMeDTO（本片新增注解）=====
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/me", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruit/me 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /recruit/me", rec.Body.Bytes(), "account", "role", "user_id")

	// ===== 2. GET /api/recruit/resumes：注解 data=service.RecruitListResult =====
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruit/resumes 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /recruit/resumes", rec.Body.Bytes(), "items", "total")

	// ===== 3. GET /api/recruit/resumes/{id}：注解 data=service.RecruitResumeCard =====
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(int(stu.ID)), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recruit/resumes/:id 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /recruit/resumes/{id}", rec.Body.Bytes(),
		"user_id", "real_name", "real_name_masked", "expected_position_extra", "expected_regions",
		"salary_negotiable", "available_in", "job_nature", "experience_years", "self_intro",
		"resume_experiences", "resume_certifications", "updated_at")
	// 脱敏边界：招聘端不可见电话/微信/现居地/PDF（注解层也未声明这些 key）
	assertSlice5MissingKeys(t, "GET /recruit/resumes/{id}", rec.Body.Bytes(),
		"contact_phone", "wechat", "region", "resume_file_url", "photos", "visibility")

	// ===== 4. POST /api/recruit/contact-requests：注解 201 data=service.ContactRequestDTO =====
	rec = doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/contact-requests",
		map[string]any{"student_user_id": stu.ID, "message": "请考虑"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发起交换申请应 201, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "POST /recruit/contact-requests", rec.Body.Bytes(),
		"id", "recruiter_id", "student_user_id", "message", "status", "created_at", "updated_at",
		"expires_at", "company_name", "contact_name", "source")
	assertSlice5MissingKeys(t, "POST /recruit/contact-requests", rec.Body.Bytes(),
		"contact_phone", "contact_email", "wechat", "decided_at")
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析交换申请失败: %v", err)
	}

	// ===== 5. GET /api/recruit/contact-requests：注解 data=service.ContactRequestListResult =====
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘方申请列表应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /recruit/contact-requests", rec.Body.Bytes(), "items", "total", "page", "page_size")

	// ===== 6. GET /api/recruit/resumes/{id}/contact：批准前 403，批准后 data=service.ContactPlainDTO =====
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(int(stu.ID))+"/contact", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("未授权读明文应 403, got %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, stuToken, http.MethodPost, "/api/resume/contact-requests/"+itoa(int(created.Data.ID))+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员同意应 200, got %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/resumes/"+itoa(int(stu.ID))+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("已授权读明文应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /recruit/resumes/{id}/contact", rec.Body.Bytes(),
		"real_name", "contact_phone", "wechat", "resume_file_url", "photos", "resume_certifications")
	if !strings.Contains(rec.Body.String(), "13800003333") {
		t.Fatalf("已授权明文应含真实电话: %s", rec.Body.String())
	}

	// ===== 7. GET /api/resume/view-stats：注解 data=object{count=integer} =====
	// 注：本用例上面刚以招聘者身份读过该学员的简历，故 count 已被留痕（≥1，按企业去重）。
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/resume/view-stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("view-stats 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /resume/view-stats", rec.Body.Bytes(), "count")
	var stats struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("解析 view-stats 失败: %v", err)
	}
	if stats.Data.Count < 1 {
		t.Fatalf("刚被招聘者查看过，count 应 ≥1, got %d body=%s", stats.Data.Count, rec.Body.String())
	}

	// ===== 8. GET /api/resume/contact-requests：学员侧同一 DTO（标量无、列表有）=====
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/resume/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员侧申请列表应 200, got %d %s", rec.Code, rec.Body.String())
	}
	assertSlice5Keys(t, "GET /resume/contact-requests", rec.Body.Bytes(), "items", "total", "page", "page_size")
}

func TestSlice5ContractShape_OnSqlite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assertSlice5Shape(t, testutil.NewMemoryDB(t))
}

func TestSlice5ContractShape_OnPostgres(t *testing.T) {
	assertSlice5Shape(t, testutil.NewPostgresDB(t))
}
