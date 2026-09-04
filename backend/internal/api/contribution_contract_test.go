// #517 契约测试：资料投稿蓝图——创建/列表/详情/下载/审核/举报端点的请求形状与信封。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newContributionRouter 装配投稿蓝图测试路由器。
func newContributionRouter(t *testing.T) (*gin.Engine, *Deps, *model.HrwaiUser, *model.Credential) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewFileDB(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	deps := newContractDeps(t, db, cfg)
	r := gin.New()
	api := r.Group("/api")
	RegisterContributionRoutes(api, deps.RouterDeps(), deps.ContributionSvc)
	// 学员（已选证件）
	cred := &model.Credential{Code: "N1", Name: "叉车司机"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	cid := cred.ID
	stu := &model.HrwaiUser{Account: "acct_c", Phone: "13800000001", Username: "学员丙", Status: 1, CurrentCredentialID: &cid, CreatedAt: testutil.Now()}
	if err := db.Create(stu).Error; err != nil {
		t.Fatalf("建学员失败: %v", err)
	}
	return r, deps, stu, cred
}

// issueContributionToken 签发学员 token。
func issueContributionToken(t *testing.T, cfg *config.Config, u *model.HrwaiUser) string {
	t.Helper()
	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(u.ID), u.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return tok
}

// contributionDo 发起 JSON 请求。
func contributionDo(t *testing.T, r *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestContributionAPIContract 学员投稿 + 管理审核端到端契约。
func TestContributionAPIContract(t *testing.T) {
	r, deps, stu, cred := newContributionRouter(t)
	cfg := &config.Config{JWTSecretKey: "contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	tok := issueContributionToken(t, cfg, stu)

	// 1. 创建投稿（files 直接传虚构 URL——service 不校验文件归属）
	createBody := map[string]any{
		"credential_id": cred.ID,
		"title":         "故障排查手册",
		"intro":         "一线维修整理",
		"files": []map[string]any{{
			"file_url": "/static/uploads/contributions/x.pdf", "file_name": "x.pdf", "file_size": 1024, "content_type": "document",
		}},
	}
	w := contributionDo(t, r, tok, "POST", "/api/contributions", createBody)
	if w.Code != http.StatusOK {
		t.Fatalf("创建投稿应 200, got %d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Code int `json:"code"`
		Data struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.Status != "pending" {
		t.Fatalf("创建后应为 pending, got %s", created.Data.Status)
	}
	cID := created.Data.ID

	// 2. 无证件学员不能投稿（另建一个未选证件的）
	noCred := &model.HrwaiUser{Account: "acct_noc", Phone: "13800000002", Username: "无证件", Status: 1, CreatedAt: testutil.Now()}
	if err := deps.DB.Create(noCred).Error; err != nil {
		t.Fatalf("建无证件学员失败: %v", err)
	}
	noCredTok := issueContributionToken(t, cfg, noCred)
	w = contributionDo(t, r, noCredTok, "POST", "/api/contributions", createBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("无证件投稿应 400, got %d", w.Code)
	}

	// 3. 未认证 401
	w = contributionDo(t, r, "", "GET", "/api/contributions?credential_id="+fmt.Sprintf("%d", cred.ID), nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未认证应 401, got %d", w.Code)
	}

	// 4. 管理员审核：学员 token 访问 admin 端点应 403
	w = contributionDo(t, r, tok, "GET", "/api/admin/contributions/pending", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("学员访问审核端点应 403, got %d", w.Code)
	}

	// 5. admin 通过
	adminTok := issueContributionAdminToken(t, cfg)
	w = contributionDo(t, r, adminTok, "POST", fmt.Sprintf("/api/admin/contributions/%d/approve", cID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("approve 应 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 6. 公开广场可见（学员视角）
	w = contributionDo(t, r, tok, "GET", "/api/contributions?credential_id="+fmt.Sprintf("%d", cred.ID)+"&sort=latest", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("列表应 200, got %d", w.Code)
	}
	var list struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Data.Total != 1 || len(list.Data.Items) != 1 || list.Data.Items[0].ID != cID {
		t.Fatalf("广场应含 1 条过审稿, got total=%d items=%d", list.Data.Total, len(list.Data.Items))
	}

	// 7. 下载（另一学员）
	other := &model.HrwaiUser{Account: "acct_o", Phone: "13800000003", Username: "下载者", Status: 1, CreatedAt: testutil.Now()}
	if err := deps.DB.Create(other).Error; err != nil {
		t.Fatalf("建下载者失败: %v", err)
	}
	otherTok := issueContributionToken(t, cfg, other)
	w = contributionDo(t, r, otherTok, "POST", fmt.Sprintf("/api/contributions/%d/download", cID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("下载应 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 8. 举报 + 管理端队列 + 处置
	w = contributionDo(t, r, otherTok, "POST", fmt.Sprintf("/api/contributions/%d/report", cID), map[string]any{"reason": "piracy"})
	if w.Code != http.StatusOK {
		t.Fatalf("举报应 200, got %d body=%s", w.Code, w.Body.String())
	}
	w = contributionDo(t, r, adminTok, "GET", "/api/admin/contributions/reports?status=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("举报队列应 200, got %d", w.Code)
	}
	var reports struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &reports)
	if len(reports.Data.Items) != 1 {
		t.Fatalf("举报队列应 1 条, got %d", len(reports.Data.Items))
	}
	w = contributionDo(t, r, adminTok, "POST", fmt.Sprintf("/api/admin/contributions/reports/%d/handle", reports.Data.Items[0].ID), map[string]any{"action": "archive"})
	if w.Code != http.StatusOK {
		t.Fatalf("处置举报应 200, got %d body=%s", w.Code, w.Body.String())
	}
	// 下架后广场清空
	w = contributionDo(t, r, tok, "GET", "/api/contributions?credential_id="+fmt.Sprintf("%d", cred.ID), nil)
	var after struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &after)
	if after.Data.Total != 0 {
		t.Fatalf("下架后广场应为空, got total=%d", after.Data.Total)
	}
}

// issueContributionAdminToken 签发 admin token（契约测试）。
func issueContributionAdminToken(t *testing.T, cfg *config.Config) string {
	t.Helper()
	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(1, "admin1", "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}
	return tok
}
