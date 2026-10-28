// 契约测试 #2070 / ADR-0089：反馈小域（提交 / 本人历史 / 管理端列表 / 标记已处理）。
//
// 守的边界：
//   - 未登录一律 401；学员访问管理端 403；
//   - 文字必填且 ≤1000 字；图片 ≤3 张、仅本站 images/forum/ 前缀、必须是图片、文件必须真实存在；
//   - 联系方式**可空且不校验格式**（写微信号/QQ/邮箱/「晚上八点后打」都收）；
//   - 本人历史只看得到自己的；他人反馈不可见（判据是 WHERE user_id = 本人，不是前端过滤）；
//   - 管理端列表能按状态筛、非法状态值 400；每行带提交人；
//   - 标记已处理**幂等**：再点返回同一行、200、handled_at 不变；不存在 404。
//
// 图片走的是论坛图片通道（ADR-0066 决策 5：不新建通道），故本用例真的上传一张 PNG 再提交。
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/feedback"
	"forklift-training/internal/forum"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// feedbackEnv 一套跑完整的被测装配（学员 / 他人 / 管理员三枚 token + 路由）。
type feedbackEnv struct {
	r     *gin.Engine
	stu   string
	other string
	admin string
	deps  *Deps
	db    *gorm.DB
}

// pngBytes 一枚合法的最小 PNG（1x1）：内容不重要，通道按扩展名 + 大小校验。
var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// newFeedbackEnv 建装配：存储用临时目录的真 LocalStorage（图片「真实存在」那一校验要问它）。
func newFeedbackEnv(t *testing.T, db *gorm.DB) *feedbackEnv {
	t.Helper()
	testutil.SetTestGinMode()
	stuPwd, _ := core.HashPassword("student123")
	stu := seedStudent(t, db, "stuFeedback", stuPwd)
	otherStu := seedStudent(t, db, "stuFeedback2", stuPwd)
	adminPwd, _ := core.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminFeedback", adminPwd)

	cfg := &config.Config{JWTSecretKey: "feedback-secret", JWTExpiresHours: 2}
	deps := newContractDepsWithStorage(t, db, cfg, storage.NewLocalStorage(t.TempDir()))
	r := gin.New()
	apiGroup := r.Group("/api")
	// 动态角色（admin）的能力解析源：管理端路由的能力守卫要它，缺了会一律 403
	// （与走 NewRouter 的生产装配同源；forum_featured_contract_test.go 同款）。
	apiGroup.Use(middleware.AdminCapabilityResolver(deps.AdminSvc))
	// 论坛图片通道（ADR-0089 决策 5：反馈图复用它，不新建通道）
	forum.RegisterRoutes(apiGroup, deps.RouterDeps().Session, deps.ForumSvc, deps.ForumModSvc, deps.ForumImageSvc)
	feedback.RegisterRoutes(apiGroup, deps.RouterDeps().Session, deps.FeedbackSvc)

	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := sess.Issue(int(stu.ID), stu.Username, "hrwai_user")
	otherToken, _ := sess.Issue(int(otherStu.ID), otherStu.Username, "hrwai_user")
	adminToken, _ := sess.Issue(admin.AdminID, admin.Username, "admin")
	return &feedbackEnv{r: r, stu: stuToken, other: otherToken, admin: adminToken, deps: deps, db: db}
}

// uploadImage 走论坛图片通道真上传一张 PNG，返回可访问 URL。
func (e *feedbackEnv) uploadImage(t *testing.T, token string) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "screenshot.png")
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := fw.Write(pngBytes); err != nil {
		t.Fatalf("写 multipart 失败: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/forum/upload-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("上传论坛图片失败: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Data.URL == "" {
		t.Fatalf("上传未返回 url: %s", w.Body.String())
	}
	return env.Data.URL
}

// submit 提交一条反馈，返回状态码与解出的 DTO。
func (e *feedbackEnv) submit(t *testing.T, token string, body map[string]any) (int, feedback.FeedbackDTO) {
	t.Helper()
	rec := doWithToken(t, e.r, token, http.MethodPost, "/api/feedback", body)
	var env struct {
		Data feedback.FeedbackDTO `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Data
}

// list 拉一次列表（学员面 / 管理面），返回状态码与解出的列表。
func (e *feedbackEnv) list(t *testing.T, token, path string) (int, feedback.FeedbackListResult) {
	t.Helper()
	rec := doWithToken(t, e.r, token, http.MethodGet, path, nil)
	var env struct {
		Data feedback.FeedbackListResult `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Data
}

func assertFeedbackContract(t *testing.T, db *gorm.DB) {
	e := newFeedbackEnv(t, db)

	// ===== 1. 未登录一律 401 =====
	for _, probe := range []struct{ method, path string }{
		{http.MethodPost, "/api/feedback"},
		{http.MethodGet, "/api/feedback"},
		{http.MethodGet, "/api/admin/feedback"},
		{http.MethodPost, "/api/admin/feedback/1/handle"},
	} {
		if rec := doWithoutToken(t, e.r, probe.method, probe.path); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s 未登录应 401, got %d", probe.method, probe.path, rec.Code)
		}
	}

	// ===== 2. 文字校验：必填 + 上限 =====
	if code, _ := e.submit(t, e.stu, map[string]any{"content": "   "}); code != http.StatusBadRequest {
		t.Fatalf("空内容应 400, got %d", code)
	}
	if code, _ := e.submit(t, e.stu, map[string]any{"content": strings.Repeat("字", feedback.FeedbackContentMaxLen+1)}); code != http.StatusBadRequest {
		t.Fatalf("超长内容应 400, got %d", code)
	}

	// ===== 3. 正常提交（无图）：pending + handled_at=null + images=[] =====
	code, first := e.submit(t, e.stu, map[string]any{"content": "  第三节的视频打不开，换浏览器也一样  "})
	if code != http.StatusCreated {
		t.Fatalf("提交反馈应 201, got %d", code)
	}
	if first.Status != model.FeedbackStatusPending {
		t.Fatalf("新反馈应是 pending, got %s", first.Status)
	}
	if first.HandledAt != nil {
		t.Fatalf("未处理时 handled_at 应是 null, got %v", *first.HandledAt)
	}
	if first.Images == nil || len(first.Images) != 0 {
		t.Fatalf("无图时 images 应是空集（不是 null）, got %+v", first.Images)
	}
	if first.Content != "第三节的视频打不开，换浏览器也一样" {
		t.Fatalf("正文应去首尾空白后原样保存, got %q", first.Content)
	}

	// ===== 4. 联系方式可空且不校验格式 =====
	code, withContact := e.submit(t, e.stu, map[string]any{
		"content": "积分兑换后没到账",
		"contact": "微信 abc-123 / 晚上八点后打 138****0000",
	})
	if code != http.StatusCreated {
		t.Fatalf("带非常规联系方式应 201（不强制格式）, got %d", code)
	}
	if withContact.Contact != "微信 abc-123 / 晚上八点后打 138****0000" {
		t.Fatalf("联系方式应原样保存, got %q", withContact.Contact)
	}

	// ===== 5. 图片：真上传一张，再带 URL 提交；地址可被平台解析到 =====
	imgURL := e.uploadImage(t, e.stu)
	code, withImage := e.submit(t, e.stu, map[string]any{"content": "报错截图见附件", "images": []string{imgURL}})
	if code != http.StatusCreated {
		t.Fatalf("带合法图片应 201, got %d", code)
	}
	if len(withImage.Images) != 1 || withImage.Images[0] != imgURL {
		t.Fatalf("图片 URL 应原样回显, got %+v", withImage.Images)
	}
	if ok, err := e.deps.FileSvc.Exists(context.Background(), imgURL); err != nil || !ok {
		t.Fatalf("反馈图片在存储侧应可解析（err=%v ok=%v）", err, ok)
	}

	// ===== 6. 图片校验：外链 / 非图片 / 超数量 / 站内但不存在 =====
	if code, _ := e.submit(t, e.stu, map[string]any{"content": "外链", "images": []string{"https://evil.example.com/a.png"}}); code != http.StatusBadRequest {
		t.Fatalf("外链图片应 400, got %d", code)
	}
	if code, _ := e.submit(t, e.stu, map[string]any{
		"content": "非图片", "images": []string{"/static/uploads/images/forum/not-image.pdf"},
	}); code != http.StatusBadRequest {
		t.Fatalf("非图片扩展名应 400, got %d", code)
	}
	if code, _ := e.submit(t, e.stu, map[string]any{
		"content": "四张图", "images": []string{imgURL, imgURL, imgURL, imgURL},
	}); code != http.StatusBadRequest {
		t.Fatalf("超过 3 张应 400, got %d", code)
	}
	if code, _ := e.submit(t, e.stu, map[string]any{
		"content": "站内但没有这个文件", "images": []string{"/static/uploads/images/forum/ghost_1.png"},
	}); code != http.StatusBadRequest {
		t.Fatalf("文件不存在应 400, got %d", code)
	}

	// ===== 7. 本人历史：只看得到自己的 =====
	code, mine := e.list(t, e.stu, "/api/feedback")
	if code != http.StatusOK {
		t.Fatalf("本人历史应 200, got %d", code)
	}
	if mine.Total != 3 {
		t.Fatalf("本人应有 3 条反馈, got %d", mine.Total)
	}
	if code, others := e.list(t, e.other, "/api/feedback"); code != http.StatusOK || others.Total != 0 {
		t.Fatalf("他人反馈不可见（应 0 条）, got %d / %d", code, others.Total)
	}

	// ===== 8. 管理端：学员 403；管理员可见并可筛 =====
	if code, _ := e.list(t, e.stu, "/api/admin/feedback"); code != http.StatusForbidden {
		t.Fatalf("学员访问管理端应 403, got %d", code)
	}
	code, all := e.list(t, e.admin, "/api/admin/feedback")
	if code != http.StatusOK || all.Total != 3 {
		t.Fatalf("管理端应看到全部 3 条, got %d / %d", code, all.Total)
	}
	for _, item := range all.Items {
		if item.UserID == 0 || item.Username == "" {
			t.Fatalf("管理端列表应带提交人, got %+v", item)
		}
	}
	if code, pending := e.list(t, e.admin, "/api/admin/feedback?status=pending"); code != http.StatusOK || pending.Total != 3 {
		t.Fatalf("pending 筛选应 3 条, got %d / %d", code, pending.Total)
	}
	if code, handled := e.list(t, e.admin, "/api/admin/feedback?status=handled"); code != http.StatusOK || handled.Total != 0 {
		t.Fatalf("handled 筛选应 0 条, got %d / %d", code, handled.Total)
	}
	if code, _ := e.list(t, e.admin, "/api/admin/feedback?status=weird"); code != http.StatusBadRequest {
		t.Fatalf("非法状态筛选应 400（不许静默当全部）, got %d", code)
	}

	// ===== 9. 标记已处理：幂等 + 404 =====
	api := "/api/admin/feedback/" + itoa(int(first.ID)) + "/handle"
	rec := doWithToken(t, e.r, e.admin, http.MethodPost, api, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("标记已处理应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var handled struct {
		Data feedback.FeedbackDTO `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &handled)
	if handled.Data.Status != model.FeedbackStatusHandled || handled.Data.HandledAt == nil {
		t.Fatalf("处理后应 handed 且带时间, got %+v", handled.Data)
	}
	firstHandledAt := *handled.Data.HandledAt
	// 再点一次：仍 200、同一行、时间不变（幂等）
	rec = doWithToken(t, e.r, e.admin, http.MethodPost, api, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("重复标记应 200（幂等）, got %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &handled)
	if handled.Data.HandledAt == nil || *handled.Data.HandledAt != firstHandledAt {
		t.Fatalf("重复标记不该改写处理时间：%v vs %v", handled.Data.HandledAt, firstHandledAt)
	}
	if rec := doWithToken(t, e.r, e.admin, http.MethodPost, "/api/admin/feedback/999999/handle", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的反馈应 404, got %d", rec.Code)
	}

	// ===== 10. 处理后筛选口径跟着变 =====
	if _, handledList := e.list(t, e.admin, "/api/admin/feedback?status=handled"); handledList.Total != 1 {
		t.Fatalf("处理后 handled 筛选应 1 条, got %d", handledList.Total)
	}
	if _, pendingList := e.list(t, e.admin, "/api/admin/feedback?status=pending"); pendingList.Total != 2 {
		t.Fatalf("处理后 pending 筛选应 2 条, got %d", pendingList.Total)
	}

	// ===== 11. 学员侧历史里也看得到状态（单向消息只回状态，不回任何回复文本） =====
	_, mineAfter := e.list(t, e.stu, "/api/feedback")
	foundHandled := false
	for _, item := range mineAfter.Items {
		if item.ID == first.ID && item.Status == model.FeedbackStatusHandled {
			foundHandled = true
		}
	}
	if !foundHandled {
		t.Fatalf("学员侧历史应看到自己那条已处理, got %+v", mineAfter.Items)
	}
}

func TestFeedbackContract_OnSqlite(t *testing.T) {
	assertFeedbackContract(t, testutil.NewMemoryDB(t))
}

func TestFeedbackContract_OnPostgres(t *testing.T) {
	assertFeedbackContract(t, testutil.NewPostgresDB(t))
}
