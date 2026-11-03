// ADR-0048 契约 codegen 片八（#966）：aiAssistant 域**信封端点**的顶层 key 锁。
//
// 本片把这些端点的 @Success 从「无 data 指认」改为 `data=aiassistant.Xxx`（SSE 端点除外），
// 注解从此是前端响应类型的唯一事实源 —— 本测试用真实 router + 装配根断言**实际返回**的
// data 顶层 key 与指认一致（先例：片一在 internal/api/*_contract_test.go 里断言 JSON keys）。
//
// 不在本锁范围（非统一信封，ADR-0048 决策 6）：
//   - POST /api/ai-assistant/chat —— SSE 事件流，既不是信封也没有 data；
//   - GET  /api/ai-assistant/diagnosis/ossimg/* —— 原样图片字节流（本文件只锁它「不是信封」）。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// fakeUploadStorage 只实现上传契约所需的最小存储：Save 返回稳定 URL（不落盘）。
type fakeUploadStorage struct{ lastKey string }

func (f *fakeUploadStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	f.lastKey = key
	return "https://cdn.test/" + key, nil
}
func (f *fakeUploadStorage) Delete(context.Context, string) error { return nil }
func (f *fakeUploadStorage) Exists(context.Context, string) (bool, error) {
	return false, nil
}
func (f *fakeUploadStorage) List(context.Context, string) ([]string, error) { return nil, nil }
func (f *fakeUploadStorage) ListWithInfo(context.Context, string) ([]storage.FileInfo, error) {
	return nil, nil
}
func (f *fakeUploadStorage) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not found")
}

// aiEnvelope 统一信封投影（data 保留原始 JSON，供对象 / 数组 / null 三种形态分别断言）。
type aiEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func aiOK(t *testing.T, rec *httptest.ResponseRecorder) aiEnvelope {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env aiEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析统一信封失败: %v（%s）", err, rec.Body.String())
	}
	return env
}

// aiKeys 断言 data 是对象并返回其顶层 key（排序）。
func aiKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("data 不是对象: %v（%s）", err, string(raw))
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func aiWantKeys(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("data 顶层 key = %v，期望 %v", got, want)
	}
}

// aiArray 断言 data 是数组并返回元素原始 JSON。
func aiArray(t *testing.T, raw json.RawMessage) []json.RawMessage {
	t.Helper()
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("data 不是数组: %v（%s）", err, string(raw))
	}
	return arr
}

func TestAIAssistantEnvelopeContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	stu := model.HrwaiUser{Account: "ai_contract_stu", Phone: "13800002001", Username: "AI契约", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&stu).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "ai-assistant-contract-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	st := &fakeUploadStorage{}
	r := NewRouter(NewDeps(cfg, db, st, zap.NewNop(), nil))
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(stu.ID), stu.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	// ---- GET /models（公开）：data=[]aiassistant.ModelOption ----
	env := aiOK(t, testutil.PerformRequest(r, http.MethodGet, "/api/ai-assistant/models"))
	if arr := aiArray(t, env.Data); len(arr) != 0 {
		t.Fatalf("未配置模型时 /models 的 data 应为空数组，got %s", string(env.Data))
	}

	// ---- GET /modes（公开）：data=aiassistant.AIAssistantModeModels（两个键在、值可 null） ----
	env = aiOK(t, testutil.PerformRequest(r, http.MethodGet, "/api/ai-assistant/modes"))
	aiWantKeys(t, aiKeys(t, env.Data), "expert", "normal")
	var modes map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &modes); err != nil {
		t.Fatalf("解析 modes 失败: %v", err)
	}
	for _, k := range []string{"normal", "expert"} {
		if string(modes[k]) != "null" {
			t.Fatalf("未绑定模型时 %s 应为 null（x-nullable），got %s", k, modes[k])
		}
	}

	// ---- POST /sessions（登录）：data=aiassistant.AIChatSessionDTO ----
	env = aiOK(t, doWithToken(t, r, token, http.MethodPost, "/api/ai-assistant/sessions",
		map[string]any{"title": "契约会话", "feature_key": "ai_assistant"}))
	aiWantKeys(t, aiKeys(t, env.Data), "created_at", "feature_key", "id", "model_name", "title", "updated_at")
	var created map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("解析创建会话响应失败: %v", err)
	}
	var sessionID int
	if err := json.Unmarshal(created["id"], &sessionID); err != nil || sessionID == 0 {
		t.Fatalf("创建会话未返回 id: %s", string(env.Data))
	}
	var sessionTitle string
	_ = json.Unmarshal(created["title"], &sessionTitle)
	if sessionTitle != "契约会话" {
		t.Fatalf("创建会话 title = %q，期望 契约会话", sessionTitle)
	}

	// ---- GET /sessions（登录）：data=[]aiassistant.AIChatSessionDTO ----
	env = aiOK(t, doWithToken(t, r, token, http.MethodGet, "/api/ai-assistant/sessions", nil))
	sessions := aiArray(t, env.Data)
	if len(sessions) != 1 {
		t.Fatalf("会话列表应有 1 条，got %s", string(env.Data))
	}
	aiWantKeys(t, aiKeys(t, sessions[0]), "created_at", "feature_key", "id", "model_name", "title", "updated_at")

	// ---- GET /sessions/{id}/messages（登录）：data=[]aiassistant.AIChatMessageDTO ----
	// 空 images/sources 的落库形态：键一定在、值为 null（x-nullable 的字节级事实）。
	msg := model.AIChatMessage{SessionID: sessionID, Role: "user", Content: "你好", CreatedAt: testutil.Now()}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatalf("创建会话消息失败: %v", err)
	}
	withImages := model.AIChatMessage{
		SessionID: sessionID, Role: "user", Content: "带图",
		Images: `["https://cdn.test/images/ai-assistant/a.png"]`, CreatedAt: testutil.Now().Add(time.Second),
	}
	if err := db.Create(&withImages).Error; err != nil {
		t.Fatalf("创建带图消息失败: %v", err)
	}

	path := "/api/ai-assistant/sessions/" + strconv.Itoa(sessionID) + "/messages"
	env = aiOK(t, doWithToken(t, r, token, http.MethodGet, path, nil))
	msgs := aiArray(t, env.Data)
	if len(msgs) != 2 {
		t.Fatalf("消息列表应有 2 条，got %s", string(env.Data))
	}
	aiWantKeys(t, aiKeys(t, msgs[0]), "content", "created_at", "id", "images", "role", "sources")
	var first map[string]json.RawMessage
	if err := json.Unmarshal(msgs[0], &first); err != nil {
		t.Fatalf("解析消息失败: %v", err)
	}
	if string(first["images"]) != "null" || string(first["sources"]) != "null" {
		t.Fatalf("无图/无来源消息的 images/sources 应为 null（键在、值 null），got images=%s sources=%s",
			first["images"], first["sources"])
	}
	var second map[string]json.RawMessage
	if err := json.Unmarshal(msgs[1], &second); err != nil {
		t.Fatalf("解析带图消息失败: %v", err)
	}
	if !strings.Contains(string(second["images"]), "images/ai-assistant/a.png") {
		t.Fatalf("带图消息 images 应回放 URL 数组，got %s", second["images"])
	}

	// ---- PATCH /sessions/{id}/title（登录）：data=aiassistant.AISessionRenameResultDTO ----
	env = aiOK(t, doWithToken(t, r, token, http.MethodPatch, "/api/ai-assistant/sessions/"+strconv.Itoa(sessionID)+"/title",
		map[string]any{"title": "改过的标题"}))
	aiWantKeys(t, aiKeys(t, env.Data), "message")
	var renamed map[string]string
	if err := json.Unmarshal(env.Data, &renamed); err != nil || renamed["message"] == "" {
		t.Fatalf("重命名响应 data 应含非空 message，got %s", string(env.Data))
	}

	// ---- 用户自定义模型 CRUD（登录）----
	env = aiOK(t, doWithToken(t, r, token, http.MethodGet, "/api/ai-assistant/user-models", nil))
	if arr := aiArray(t, env.Data); len(arr) != 0 {
		t.Fatalf("初始用户模型列表应为空数组，got %s", string(env.Data))
	}
	env = aiOK(t, doWithToken(t, r, token, http.MethodPost, "/api/ai-assistant/user-models", map[string]any{
		"name": "自建模型", "api_key": "sk-test", "base_url": "https://api.test", "model": "test-model",
	}))
	if string(env.Data) != "null" {
		t.Fatalf("POST /user-models 有意无载荷（data 为 null），got %s", string(env.Data))
	}
	env = aiOK(t, doWithToken(t, r, token, http.MethodGet, "/api/ai-assistant/user-models", nil))
	models := aiArray(t, env.Data)
	if len(models) != 1 {
		t.Fatalf("用户模型列表应有 1 条，got %s", string(env.Data))
	}
	aiWantKeys(t, aiKeys(t, models[0]), "api_key", "base_url", "created_at", "id", "model", "name", "updated_at")
	var userModel map[string]json.RawMessage
	if err := json.Unmarshal(models[0], &userModel); err != nil {
		t.Fatalf("解析用户模型失败: %v", err)
	}
	var modelID int
	_ = json.Unmarshal(userModel["id"], &modelID)

	// ---- POST /upload-image（可选认证）：裸 fetch 也走信封，data=aiassistant.AIImageUploadResultDTO ----
	env = aiOK(t, uploadAIImage(t, r, "chat.png"))
	aiWantKeys(t, aiKeys(t, env.Data), "url")
	var uploaded map[string]string
	if err := json.Unmarshal(env.Data, &uploaded); err != nil {
		t.Fatalf("解析上传响应失败: %v", err)
	}
	if uploaded["url"] != "https://cdn.test/"+st.lastKey || !strings.HasPrefix(st.lastKey, "images/ai-assistant/") {
		t.Fatalf("上传返回 URL 形状异常: url=%q key=%q", uploaded["url"], st.lastKey)
	}

	// ---- DELETE 两个无载荷端点 ----
	env = aiOK(t, doWithToken(t, r, token, http.MethodDelete, "/api/ai-assistant/user-models/"+strconv.Itoa(modelID), nil))
	if string(env.Data) != "null" {
		t.Fatalf("DELETE /user-models/{id} 有意无载荷（data 为 null），got %s", string(env.Data))
	}
	env = aiOK(t, doWithToken(t, r, token, http.MethodDelete, "/api/ai-assistant/sessions/"+strconv.Itoa(sessionID), nil))
	if string(env.Data) != "null" {
		t.Fatalf("DELETE /sessions/{id} 有意无载荷（data 为 null），got %s", string(env.Data))
	}
}

// TestAIAssistantDiagnosisImageProxy 回答配图代理（GET /diagnosis/ossimg/*）：白名单内透传字节流、
// 白名单外 404、中文目录名往返。**非信封**（ADR-0048 决策 6）。
// v1 的品牌/车型/故障码/手册四条代理随出口配图代理替换（票 2237 C 节）。
func TestAIAssistantDiagnosisImageProxy(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	stub := newDiagnosisStub(t)

	cfg := &config.Config{
		JWTSecretKey:       "ai-diagnosis-image-secret",
		DiagnosisV2OSSBase: stub.URL + "/forklift/repair_img",
	}
	r := NewRouter(NewDeps(cfg, db, &fakeUploadStorage{}, zap.NewNop(), nil))

	// ---- 白名单内：原样字节流 + Content-Type + 缓存头 ----
	rec := testutil.PerformRequest(r, http.MethodGet, "/api/ai-assistant/diagnosis/ossimg/linde/a.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("配图代理期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "OSS-PNG-BYTES" {
		t.Fatalf("配图代理应原样透传字节，got %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("配图代理应透传 Content-Type，got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Fatalf("配图代理应带一天缓存（不用 immutable——OSS 上是规范名不是内容哈希），got %q", cc)
	}
	if strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "{") {
		t.Fatal("配图代理是文件流、不是统一信封：响应体不应是 JSON 对象")
	}

	// ---- 中文目录名（客户端按段 encodeURIComponent）----
	rec = testutil.PerformRequest(r, http.MethodGet,
		"/api/ai-assistant/diagnosis/ossimg/images_all/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("中文目录配图期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "OSS-CN-BYTES" {
		t.Fatalf("中文目录配图字节不符，got %q", rec.Body.String())
	}

	// ---- 白名单外 / 非法：404，且不触发上游 ----
	for _, bad := range []string{
		"/api/ai-assistant/diagnosis/ossimg/linde/a.pdf",
		"/api/ai-assistant/diagnosis/ossimg/linde/%2e%2e/%2e%2e/etc.png",
		"/api/ai-assistant/diagnosis/ossimg/linde/a",
	} {
		rec = testutil.PerformRequest(r, http.MethodGet, bad)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s 期望 404, got %d", bad, rec.Code)
		}
	}
}

// newDiagnosisStub 外部诊断助手配图源替身：只覆盖 ossimg 代理消费的路径。
// 路径逐字匹配 ⇒ 同时钉住「客户端百分号编码 → 真实路由 → 代理解一次再逐段转义」这条链
// 上游拿到的是解码后的中文段。
func newDiagnosisStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/forklift/repair_img/linde/a.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("OSS-PNG-BYTES"))
	})
	mux.HandleFunc("/forklift/repair_img/images_all/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forklift/repair_img/images_all/制动系统.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("OSS-CN-BYTES"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// uploadAIImage 以 multipart/form-data 直发上传端点（handler 走 OptionalAuth，无需 token）。
func uploadAIImage(t *testing.T, r *gin.Engine, filename string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := fw.Write([]byte("fake-image-bytes")); err != nil {
		t.Fatalf("写入 multipart 失败: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/ai-assistant/upload-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
