// 契约测试（ADR-0048 片七 #965）：admin / featured 管理端端点的**顶层 key 断言**。
//
// 背景：本片为 50 个此前不在 swagger 的端点补了注解，并把响应体定型为 service / api DTO。
// 注解里 `data=` 指认的类型与 handler 实际返回是否一致，swag 侧看不出来（它只校验类型存在），
// 故这里走真实路由 + httptest，把信封 data 的**顶层 key 集合**钉在注解产物上
// （片一先例：internal/api/*_contract_test.go；DTO 字节级保形另见 internal/service 的 shape-lock）。
//
// 覆盖：用户 / 导师 / 招聘者 / 统计 / 课程 / 章节 / AI 配置与功能绑定 / 资料审核 / 审计 /
// 精选内容管理端 / 导出路由。
//
// 有意不覆盖（附理由，均已由注解 + swagger 新鲜度锁覆盖）：
//   - POST /admin/course/generate-content 与 GET .../{task_id}：创建端点会拉起后台 goroutine
//     调 AI provider，单测里没有可用的 provider（跑起来只会靠 goroutine 超时收场，断言不稳定）；
//   - POST /admin/ai-configs/{id}/test：需要外部 AI 连通，成功路径无法离线断言。
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// newAdminContractDeps 构建装配根（含 JWT + 能力位的真实路由依赖），返回 admin token。
func newAdminContractDeps(t *testing.T) (*Deps, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "admin1", adminPwd)

	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	// 用真实本地存储装配（不是 newContractDeps 的 nil）：章节/课程删除与图片上传会走 FileStore，
	// nil storage 只会在那些路径上 panic（片七契约测试要覆盖它们）。
	deps := NewDeps(cfg, db, storage.NewLocalStorage(t.TempDir()), zap.NewNop(), stubExportStore{})

	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{
		Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure,
	})
	token, err := sess.Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}
	return deps, db, token
}

// newAdminContractEnv 装配真实路由（含 JWT + 能力位中间件），返回 admin token。
func newAdminContractEnv(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	deps, db, token := newAdminContractDeps(t)
	return NewRouter(deps), db, token
}

// stubExportStore ExportStore 的测试替身：估值记录取数在单测里没有 pgx adapter
// （newContractDeps 传 nil），给一条空结果即可让 /admin/export/evaluations 走通 CSV 分支。
type stubExportStore struct{}

func (stubExportStore) ListEvaluationExports(context.Context) ([]service.EvaluationExportRow, error) {
	return nil, nil
}

// adminData 解出信封 data 原始 JSON，并断言状态码。
func adminData(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) json.RawMessage {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("期望 HTTP %d, got %d body=%s", wantStatus, rec.Code, rec.Body.String())
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应信封失败: %v body=%s", err, rec.Body.String())
	}
	return env.Data
}

// adminObject 要求 data 是 JSON 对象，返回其顶层 key → 原始 JSON。
func adminObject(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) map[string]json.RawMessage {
	t.Helper()
	raw := adminData(t, rec, wantStatus)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("data 不是 JSON 对象: %v raw=%s", err, string(raw))
	}
	return obj
}

// assertDataNull 断言 data 为 null（NoData 登记的端点：注解保持 response.R 无 data=）。
func assertDataNull(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()
	if got := strings.TrimSpace(string(adminData(t, rec, wantStatus))); got != "null" {
		t.Fatalf("期望 data 为 null（NoData 端点）, got %s", got)
	}
}

func adminKeysOf(obj map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

// assertKeysExact 断言 data 顶层 key 集合**恰好**等于 want（多一个少一个都红）。
func assertKeysExact(t *testing.T, obj map[string]json.RawMessage, want ...string) {
	t.Helper()
	if len(obj) != len(want) {
		t.Fatalf("data 顶层 key 数 = %d, 期望 %d：got=%v want=%v", len(obj), len(want), adminKeysOf(obj), want)
	}
	for _, k := range want {
		if _, ok := obj[k]; !ok {
			t.Fatalf("data 缺 key %q：got=%v want=%v", k, adminKeysOf(obj), want)
		}
	}
}

// assertHasKeys 断言 data 顶层包含 want（用于含 omitempty 可选字段的富 DTO：
// 可选键是否出现取决于填充路径，硬钉全集会把「合法缺省」判红）。
func assertHasKeys(t *testing.T, obj map[string]json.RawMessage, want ...string) {
	t.Helper()
	var missing []string
	for _, k := range want {
		if _, ok := obj[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("data 缺 key %v：got=%v", missing, adminKeysOf(obj))
	}
}

// seedAdminContractStudent 直接落一行 hrwai_users。
// 不用 testutil.SeedStudent：它的 uid 取自包级递增计数器，而 auth_me 契约测试把首个 uid
// 硬编码成 ...001（内联快照）——本文件按文件名排在它之前，先 seed 会把那个快照串掉。
func seedAdminContractStudent(t *testing.T, db *gorm.DB, username string) *model.HrwaiUser {
	t.Helper()
	u := &model.HrwaiUser{
		UID:       1000000000000000123,
		Account:   "acct_" + username,
		Username:  username,
		Password:  "hash123",
		Phone:     "test_" + username,
		Status:    1,
		CreatedAt: testutil.Now(),
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("插入测试学员失败: %v", err)
	}
	return u
}

func jsonInt(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("期望整数, got %s", string(raw))
	}
	return v
}

// TestAdminContract_UsersTutorsRecruiters 用户 / 导师 / 招聘者三组管理端点。
func TestAdminContract_UsersTutorsRecruiters(t *testing.T) {
	r, _, token := newAdminContractEnv(t)

	// ===== HRWAI 用户 =====
	rec := doWithToken(t, r, token, http.MethodGet, "/api/admin/hrwai-users", nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "list", "page", "page_size", "total")

	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/hrwai-users",
		map[string]any{"phone": "13800000001", "password": "pass1234"})
	created := adminObject(t, rec, http.StatusCreated)
	assertKeysExact(t, created, "account", "id", "phone", "uid", "username")
	userID := jsonInt(t, created["id"])

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/hrwai-users/%d/status", userID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "status")

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/hrwai-users/%d", userID),
		map[string]any{"username": "改名后", "email": "u@example.com", "company": "单位", "status": 1})
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/hrwai-users/%d/password", userID),
		map[string]any{"password": "newpass123"})
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/hrwai-users/%d", userID), nil)
	assertDataNull(t, rec, http.StatusOK)

	// ===== 导师 =====
	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/tutors", nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "page", "total", "tutors")

	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/tutor",
		map[string]any{"username": "tutor1", "password": "pass1234", "name": "张导师"})
	tutor := adminObject(t, rec, http.StatusCreated)
	assertKeysExact(t, tutor, "name", "tutor_id", "username")
	tutorID := jsonInt(t, tutor["tutor_id"])

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/status", tutorID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "status")

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/password", tutorID),
		map[string]any{"password": "newpass123"})
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/tutor/%d", tutorID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "tutor_id")

	// ===== 企业招聘者 =====
	recruitBody := map[string]any{
		"username": "recruit1", "password": "recruit123",
		"company_name": "叉车维修有限公司", "credit_code": "91110000MA12345678",
		"business_scope": "叉车维修", "contact_name": "张三",
		"contact_phone": "13800000002", "contact_email": "zhang@example.com",
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/recruiters", recruitBody)
	recruiter := adminObject(t, rec, http.StatusCreated)
	// 创建 10 字段（含 status），见 ADR-0048 片二保留的现状差异
	assertKeysExact(t, recruiter,
		"business_scope", "company_name", "contact_email", "contact_name", "contact_phone",
		"credit_code", "id", "status", "username", "wechat")
	recruiterID := jsonInt(t, recruiter["id"])

	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/recruiters", nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "items", "page", "total")

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/recruiters/%d", recruiterID), recruitBody)
	edited := adminObject(t, rec, http.StatusOK)
	// 编辑 9 字段（比创建少一个 status）—— 片二记录的现状差异，本片按字节不变保留
	assertKeysExact(t, edited,
		"business_scope", "company_name", "contact_email", "contact_name", "contact_phone",
		"credit_code", "id", "username", "wechat")

	// data 是空对象 {}（不是 null）：RecruiterPasswordResetResult 保形
	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/recruiters/%d/password", recruiterID),
		map[string]any{"password": "newpass123"})
	assertKeysExact(t, adminObject(t, rec, http.StatusOK))

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/recruiters/%d/status", recruiterID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "status")
}

// TestAdminContract_StatisticsCoursesChapters 统计 / 课程 / 章节端点。
func TestAdminContract_StatisticsCoursesChapters(t *testing.T) {
	r, db, token := newAdminContractEnv(t)

	// 课程创建校验方向/等级存在性（applyCourseTrainingFields）：先落最小字典行
	if err := db.Create(&model.Specialty{SpecialtyID: 1, Code: "spec1", Name: "维修", Status: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("插入专业方向失败: %v", err)
	}
	if err := db.Create(&model.CourseLevel{LevelID: 1, Code: "lvl1", Name: "入门", Status: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("插入课程等级失败: %v", err)
	}

	rec := doWithToken(t, r, token, http.MethodGet, "/api/admin/statistics", nil)
	stats := adminObject(t, rec, http.StatusOK)
	assertKeysExact(t, stats, "overview", "course_stats")
	var overview map[string]json.RawMessage
	if err := json.Unmarshal(stats["overview"], &overview); err != nil {
		t.Fatalf("overview 不是对象: %v", err)
	}
	assertKeysExact(t, overview, "active_today", "total_courses", "total_students", "total_study_duration")

	// 建两门同方向同等级课程（供排序交换）
	courseBody := func(name string) map[string]any {
		return map[string]any{"name": name, "specialty_id": 1, "level_id": 1, "theory_hours": 4, "practice_hours": 2}
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/course", courseBody("课程A"))
	c1 := adminObject(t, rec, http.StatusCreated)
	// CourseDTO 必填面（可选键 certificate_name / credential / specialty / level / points_price /
	// student_count / chapters 取决于填充路径，这里只钉必填）
	assertHasKeys(t, c1, "course_id", "name", "status", "created_at", "cover_image", "description",
		"duration", "is_hot", "is_featured", "theory_hours", "practice_hours", "sort_order",
		"specialty_id", "level_id", "credential_id", "certificate_template_id",
		"chapter_count", "prerequisite_course_ids")
	courseID1 := jsonInt(t, c1["course_id"])

	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/course", courseBody("课程B"))
	courseID2 := jsonInt(t, adminObject(t, rec, http.StatusCreated)["course_id"])

	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/courses", nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "courses", "page", "pages", "total")

	rec = doWithToken(t, r, token, http.MethodGet, fmt.Sprintf("/api/admin/course/%d", courseID1), nil)
	detail := adminObject(t, rec, http.StatusOK)
	assertHasKeys(t, detail, "course_id", "name", "status", "chapters")
	var chapters []json.RawMessage
	if err := json.Unmarshal(detail["chapters"], &chapters); err != nil {
		t.Fatalf("chapters 不是数组: %v", err)
	}

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/course/%d", courseID1),
		map[string]any{"name": "课程A改", "specialty_id": 1, "level_id": 1})
	assertHasKeys(t, adminObject(t, rec, http.StatusOK), "course_id", "name", "status")

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/course/%d/sort", courseID1),
		map[string]any{"swap_with": courseID2})
	assertDataNull(t, rec, http.StatusOK)

	// ===== 章节 =====
	rec = doWithToken(t, r, token, http.MethodPost, fmt.Sprintf("/api/admin/course/%d/chapter", courseID1),
		map[string]any{"title": "第一章", "content": "正文", "order_num": 1})
	chapter := adminObject(t, rec, http.StatusCreated)
	assertHasKeys(t, chapter, "chapter_id", "course_id", "title", "content", "content_type",
		"created_at", "description", "duration", "file_url", "order_num")
	chapterID := jsonInt(t, chapter["chapter_id"])

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/chapter/%d", chapterID),
		map[string]any{"title": "第一章改"})
	assertHasKeys(t, adminObject(t, rec, http.StatusOK), "chapter_id", "course_id", "title")

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/chapter/%d", chapterID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "chapter_id")

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/course/%d", courseID1), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "course_id")
}

// TestAdminContract_AIConfigsBindingsReviewAudit AI 配置 / 功能绑定 / 资料审核 / 审计日志。
func TestAdminContract_AIConfigsBindingsReviewAudit(t *testing.T) {
	r, db, token := newAdminContractEnv(t)

	// ===== AI 配置 =====
	rec := doWithToken(t, r, token, http.MethodPost, "/api/admin/ai-configs",
		map[string]any{"name": "主配置", "api_key": "sk-test", "base_url": "https://api.example.com", "model": "gpt-x"})
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/ai-configs", nil)
	raw := adminData(t, rec, http.StatusOK)
	var configs []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &configs); err != nil {
		t.Fatalf("data 不是数组: %v raw=%s", err, string(raw))
	}
	if len(configs) != 1 {
		t.Fatalf("配置数 = %d, 期望 1", len(configs))
	}
	assertKeysExact(t, configs[0], "id", "name", "api_key", "base_url", "model", "description",
		"is_active", "created_at", "updated_at")
	configID := jsonInt(t, configs[0]["id"])

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/ai-configs/%d", configID),
		map[string]any{"name": "主配置改", "base_url": "https://api.example.com", "model": "gpt-x"})
	assertDataNull(t, rec, http.StatusOK)

	// ===== 功能绑定（全量展示：未绑定功能的可选键缺省） =====
	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/ai-feature-bindings", nil)
	raw = adminData(t, rec, http.StatusOK)
	var bindings []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bindings); err != nil {
		t.Fatalf("data 不是数组: %v raw=%s", err, string(raw))
	}
	if len(bindings) != len(service.AllAIFeatures) {
		t.Fatalf("绑定列表条数 = %d, 期望全部功能 %d 条", len(bindings), len(service.AllAIFeatures))
	}
	assertHasKeys(t, bindings[0], "feature_key", "feature_label")

	featureKey := service.AllAIFeatures[0]
	rec = doWithToken(t, r, token, http.MethodPut, "/api/admin/ai-feature-bindings/"+featureKey,
		map[string]any{"config_id": configID})
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodDelete,
		fmt.Sprintf("/api/admin/ai-feature-bindings/%s/configs/%d", featureKey, configID), nil)
	assertDataNull(t, rec, http.StatusOK)

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/ai-configs/%d", configID), nil)
	assertDataNull(t, rec, http.StatusOK)

	// ===== 资料审核 =====
	student := seedAdminContractStudent(t, db, "stu1")
	pending := model.ProfileChangeRequest{
		UserID: student.ID, FieldType: service.ProfileFieldNickname,
		OldValue: "旧昵称", NewValue: "新昵称", Status: service.ProfileStatusPending, CreatedAt: testutil.Now(),
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("插入待审请求失败: %v", err)
	}
	pending2 := pending
	pending2.ID = 0
	pending2.NewValue = "另一个昵称"
	if err := db.Create(&pending2).Error; err != nil {
		t.Fatalf("插入第二条待审请求失败: %v", err)
	}

	reviewKeys := []string{"id", "user_id", "username", "avatar_url", "field_type",
		"old_value", "new_value", "status", "reject_reason", "created_at"}

	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/profile-reviews", nil)
	page := adminObject(t, rec, http.StatusOK)
	assertKeysExact(t, page, "page", "pages", "requests", "total")
	var requests []map[string]json.RawMessage
	if err := json.Unmarshal(page["requests"], &requests); err != nil {
		t.Fatalf("requests 不是数组: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("待审条数 = %d, 期望 2", len(requests))
	}
	assertHasKeys(t, requests[0], reviewKeys...)

	rec = doWithToken(t, r, token, http.MethodPost,
		fmt.Sprintf("/api/admin/profile-reviews/%d/approve", pending.ID), map[string]any{})
	approved := adminObject(t, rec, http.StatusOK)
	assertHasKeys(t, approved, append(reviewKeys, "reviewed_by", "reviewed_at")...)

	rec = doWithToken(t, r, token, http.MethodPost,
		fmt.Sprintf("/api/admin/profile-reviews/%d/reject", pending2.ID), map[string]any{"reason": "头像不清晰"})
	assertHasKeys(t, adminObject(t, rec, http.StatusOK), reviewKeys...)

	// ===== 审计日志（管理端写操作已被审计中间件记录） =====
	rec = doWithToken(t, r, token, http.MethodGet, "/api/admin/audit-logs", nil)
	logs := adminObject(t, rec, http.StatusOK)
	assertKeysExact(t, logs, "items", "page", "pages", "total")
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(logs["items"], &items); err != nil {
		t.Fatalf("items 不是数组: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("审计日志为空：管理端写操作应已被中间件记录")
	}
	// detail 为 JSONB（注解侧 swaggertype:"object" + x-optional）——审计中间件恒写入，故键在
	assertKeysExact(t, items[0], "id", "actor_id", "actor_role", "actor_name", "action", "path",
		"method", "request_id", "ip", "status", "detail", "created_at")
}

// TestFeaturedContract_AdminEndpoints 精选内容管理端端点（含 201 创建与发布态流转）。
func TestFeaturedContract_AdminEndpoints(t *testing.T) {
	r, _, token := newAdminContractEnv(t)

	rec := doWithToken(t, r, token, http.MethodGet, "/api/admin/featured-contents", nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "items", "page", "pages", "total")

	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/featured-content",
		map[string]any{"title": "精选一", "category": "industry", "summary": "摘要", "content": "正文"})
	created := adminObject(t, rec, http.StatusCreated)
	// FeaturedContentAdminDetailDTO 全字段必填（published_at 可空但键在）
	assertKeysExact(t, created, "category", "category_label", "content", "content_id", "cover_image",
		"created_at", "published_at", "sort_order", "source", "status", "summary", "title",
		"updated_at", "view_count")
	contentID := jsonInt(t, created["content_id"])

	rec = doWithToken(t, r, token, http.MethodGet, fmt.Sprintf("/api/admin/featured-content/%d", contentID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "category", "category_label", "content",
		"content_id", "cover_image", "created_at", "published_at", "sort_order", "source",
		"status", "summary", "title", "updated_at", "view_count")

	rec = doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/featured-content/%d", contentID),
		map[string]any{"title": "精选一改"})
	assertHasKeys(t, adminObject(t, rec, http.StatusOK), "content_id", "title", "status")

	rec = doWithToken(t, r, token, http.MethodPost, fmt.Sprintf("/api/admin/featured-content/%d/publish", contentID), nil)
	published := adminObject(t, rec, http.StatusOK)
	assertHasKeys(t, published, "content_id", "status", "published_at")
	if got := jsonInt(t, published["status"]); got != 1 {
		t.Fatalf("发布后 status = %d, 期望 1", got)
	}

	rec = doWithToken(t, r, token, http.MethodDelete, fmt.Sprintf("/api/admin/featured-content/%d", contentID), nil)
	assertKeysExact(t, adminObject(t, rec, http.StatusOK), "content_id")
}

// TestAdminContract_ExportRoutes 导出路由（非统一信封的 CSV 附件）：
// 三条路由改由具名包装方法承载注解（swag 只认函数声明的注释块），这里钉住路由没被改坏。
func TestAdminContract_ExportRoutes(t *testing.T) {
	r, _, token := newAdminContractEnv(t)
	for _, kind := range []string{"students", "questions", "evaluations"} {
		rec := doWithToken(t, r, token, http.MethodGet, "/api/admin/export/"+kind, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /admin/export/%s 期望 200, got %d body=%s", kind, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
			t.Errorf("/admin/export/%s Content-Type = %q, 期望 text/csv", kind, ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
			t.Errorf("/admin/export/%s Content-Disposition = %q, 期望 attachment", kind, cd)
		}
		if strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "{") {
			t.Errorf("/admin/export/%s 响应不是 CSV（疑似走了统一信封）", kind)
		}
	}
}
