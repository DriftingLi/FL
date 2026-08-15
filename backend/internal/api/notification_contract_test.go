// Ticket #228 契约测试：通知结构化 payload（N1）。
// 锁定：资料审核通过后学员通知的标题人读文案不变，且 payload.review_status === 'approved'。
// 数据经 /api/notifications 信封返回。
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

// newNotificationContractEnv 装配全路由 + 种子 admin，返回路由器、config 与 service 装配根。
func newNotificationContractEnv(t *testing.T) (*gin.Engine, *config.Config, *Deps) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecretKey: "contract-notif-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	testutil.SeedAdmin(t, db, "admin1", "hash123")
	deps := newContractDeps(t, db, cfg)
	return NewRouter(deps), cfg, deps
}

// contractToken 按角色签发 JWT（admin/student 共用）。
func contractToken(t *testing.T, cfg *config.Config, id int, account, role string) string {
	t.Helper()
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(id, account, role)
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}

// contractJSONRequest 带 JSON body 的请求 helper（token 可为空）。
func contractJSONRequest(t *testing.T, r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestNotificationContract_ApprovedPayload：学员提交昵称修改 → 管理员通过 → 学员通知列表
// payload.review_status=approved，标题人读文案不变。
func TestNotificationContract_ApprovedPayload(t *testing.T) {
	r, cfg, deps := newNotificationContractEnv(t)

	student := testutil.SeedStudent(t, deps.DB, "学员甲", "x")
	studentToken := contractToken(t, cfg, student.ID, student.Account, "student")
	adminToken := contractToken(t, cfg, 1, "admin1", "admin")

	// 1. 学员提交昵称修改（触发资料审核请求）
	w := contractJSONRequest(t, r, http.MethodPut, "/api/auth/profile", studentToken, `{"nickname":"新昵称甲"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("提交昵称修改状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}

	// 2. 管理员通过该审核请求
	var req model.ProfileChangeRequest
	if err := deps.DB.Where("user_id = ? AND status = ?", student.ID, service.ProfileStatusPending).First(&req).Error; err != nil {
		t.Fatalf("查询 pending 审核请求失败: %v", err)
	}
	w = contractJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/admin/profile-reviews/%d/approve", req.ID), adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("approve 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}

	// 3. 学员查通知列表：标题不变，payload.review_status=approved
	w = contractJSONRequest(t, r, http.MethodGet, "/api/notifications", studentToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("通知列表状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	var unpack = struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}{}
	var notifList struct {
		Items []struct {
			Title   string          `json:"title"`
			Payload json.RawMessage `json:"payload"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &unpack); err != nil {
		t.Fatalf("解析通知列表失败: %v\nbody=%s", err, w.Body.String())
	}
	if err := json.Unmarshal(unpack.Data, &notifList); err != nil {
		t.Fatalf("解析通知 data 失败: %v (raw=%s)", err, unpack.Data)
	}
	if len(notifList.Items) != 1 {
		t.Fatalf("期望 1 条通知，得到 %d", len(notifList.Items))
	}
	n := notifList.Items[0]
	if n.Title != "资料审核通过" {
		t.Errorf("标题未保持人读文案: %q", n.Title)
	}
	var payload struct {
		ReviewStatus string `json:"review_status"`
	}
	if err := json.Unmarshal(n.Payload, &payload); err != nil {
		t.Fatalf("通知 payload 解析失败: %v (raw=%s)", err, n.Payload)
	}
	if payload.ReviewStatus != "approved" {
		t.Errorf("通过审核通知 payload.review_status = %q, 期望 approved", payload.ReviewStatus)
	}
}
