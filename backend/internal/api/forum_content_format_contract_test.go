// 契约测试 #877（ADR-0044）：正文格式声明 content_format。
//
// 守四件事：
//  1. 发帖与回复**两条写入路径同口径**：缺省一律 text（不是空串）。
//  2. 非法值 400，不静默归一——静默归一会让客户端以为自己设置生效了。
//  3. DTO 读写往返：创建响应 / 详情 / 列表三处都回传且值一致。
//  4. **编辑帖子不改变格式**：更新路径是全量替换语义（四字段都必填都显式写），
//     但 content_format 是新增字段，既有客户端（移动端）不带它。若更新路径也按
//     「缺省 = text」处理，移动端编辑一次 markdown 帖就会把格式**静默重置**成纯文本。
//     故更新路径不写该字段（保持原值），本测试钉住这条不变式。
//
// 注意：迁移里给两表加的 CHECK (content_format IN (...)) 不在本文件覆盖范围内 ——
// 测试库由 AutoMigrate 建表、不执行 migrations/ 下的 SQL。该约束由 migration-check 验证。
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

type contentFormatResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID            int64  `json:"id"`
		ContentFormat string `json:"content_format"`
	} `json:"data"`
}

type contentFormatDetailResp struct {
	Code int `json:"code"`
	Data struct {
		Topic struct {
			ID            int64  `json:"id"`
			ContentFormat string `json:"content_format"`
		} `json:"topic"`
		Replies []struct {
			ID            int64  `json:"id"`
			ContentFormat string `json:"content_format"`
		} `json:"replies"`
	} `json:"data"`
}

type contentFormatListResp struct {
	Code int `json:"code"`
	Data struct {
		Topics []struct {
			ID            int64  `json:"id"`
			ContentFormat string `json:"content_format"`
		} `json:"topics"`
	} `json:"data"`
}

func TestForumContentFormatContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(api, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

	now := testutil.Now()
	mkUser := func(account, phone, name string) model.HrwaiUser {
		u := model.HrwaiUser{Account: account, Phone: phone, Username: name, Status: 1, CreatedAt: now}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("建用户失败: %v", err)
		}
		return u
	}
	author := mkUser("cf_author", "13800000901", "楼主")
	replier := mkUser("cf_replier", "13800000902", "答主")

	tok := func(u model.HrwaiUser) string {
		s, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return s
	}
	authorTok, replierTok := tok(author), tok(replier)

	do := func(tk, method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+tk)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	createTopic := func(tk string, body map[string]any) *httptest.ResponseRecorder {
		return do(tk, http.MethodPost, "/api/forum/topics", body)
	}

	decodeTopic := func(w *httptest.ResponseRecorder) contentFormatResp {
		t.Helper()
		var got contentFormatResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v (%s)", err, w.Body.String())
		}
		return got
	}

	t.Run("发帖声明 markdown：创建响应回传 markdown", func(t *testing.T) {
		w := createTopic(authorTok, map[string]any{
			"category": "discussion", "title": "Markdown 帖", "content": "# 标题",
			"images": []string{}, "content_format": "markdown",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("期望 201, got %d %s", w.Code, w.Body.String())
		}
		if got := decodeTopic(w).Data.ContentFormat; got != "markdown" {
			t.Fatalf("content_format = %q, want markdown", got)
		}
	})

	t.Run("格式首尾空白按既有约定归一（与 normalizeForumCategory 同口径）", func(t *testing.T) {
		w := createTopic(authorTok, map[string]any{
			"category": "discussion", "title": "带空白", "content": "x",
			"images": []string{}, "content_format": "  markdown  ",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("期望 201, got %d %s", w.Code, w.Body.String())
		}
		if got := decodeTopic(w).Data.ContentFormat; got != "markdown" {
			t.Fatalf("content_format = %q, want markdown（应 TrimSpace）", got)
		}
	})

	t.Run("发帖不声明格式：缺省 text（而非空串）", func(t *testing.T) {
		w := createTopic(authorTok, map[string]any{
			"category": "discussion", "title": "纯文本帖", "content": "普通内容", "images": []string{},
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("期望 201, got %d %s", w.Code, w.Body.String())
		}
		if got := decodeTopic(w).Data.ContentFormat; got != "text" {
			t.Fatalf("缺省 content_format = %q, want text", got)
		}
	})

	t.Run("发帖非法格式：400 而非静默归一", func(t *testing.T) {
		// 大小写敏感与 normalizeForumCategory 同口径（写路径不 ToLower）；
		// 但首尾空白按既有约定 TrimSpace 后接受，见下一条用例。
		for _, bad := range []string{"html", "MD", "text;markdown", "markdownx"} {
			w := createTopic(authorTok, map[string]any{
				"category": "discussion", "title": "非法格式", "content": "x",
				"images": []string{}, "content_format": bad,
			})
			if w.Code != http.StatusBadRequest {
				t.Errorf("content_format=%q 期望 400, got %d %s", bad, w.Code, w.Body.String())
			}
		}
	})

	// 造一个 markdown 主题作为详情/列表/回复的载体。
	mkTopic := createTopic(authorTok, map[string]any{
		"category": "question", "title": "载体帖", "content": "## 步骤",
		"images": []string{}, "content_format": "markdown",
	})
	topicID := decodeTopic(mkTopic).Data.ID
	if topicID == 0 {
		t.Fatalf("建载体帖失败: %s", mkTopic.Body.String())
	}

	t.Run("回复声明 markdown：创建响应回传 markdown", func(t *testing.T) {
		w := do(replierTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID),
			map[string]any{"content": "```\nE01\n```", "images": []string{}, "content_format": "markdown"})
		if w.Code != http.StatusCreated {
			t.Fatalf("期望 201, got %d %s", w.Code, w.Body.String())
		}
		if got := decodeTopic(w).Data.ContentFormat; got != "markdown" {
			t.Fatalf("回复 content_format = %q, want markdown", got)
		}
	})

	t.Run("回复不声明格式：缺省 text", func(t *testing.T) {
		w := do(replierTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID),
			map[string]any{"content": "普通回复", "images": []string{}})
		if w.Code != http.StatusCreated {
			t.Fatalf("期望 201, got %d %s", w.Code, w.Body.String())
		}
		if got := decodeTopic(w).Data.ContentFormat; got != "text" {
			t.Fatalf("缺省回复 content_format = %q, want text", got)
		}
	})

	t.Run("回复非法格式：400", func(t *testing.T) {
		w := do(replierTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID),
			map[string]any{"content": "x", "images": []string{}, "content_format": "html"})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("详情回传主题与回复各自的格式", func(t *testing.T) {
		w := do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d?page=1&page_size=20", topicID), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("期望 200, got %d %s", w.Code, w.Body.String())
		}
		var got contentFormatDetailResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if got.Data.Topic.ContentFormat != "markdown" {
			t.Errorf("详情主题 content_format = %q, want markdown", got.Data.Topic.ContentFormat)
		}
		if len(got.Data.Replies) != 2 {
			t.Fatalf("应有 2 条回复, got %d", len(got.Data.Replies))
		}
		// 回复按时间正序：第一条 markdown、第二条 text —— 两条互不污染。
		if got.Data.Replies[0].ContentFormat != "markdown" {
			t.Errorf("第 1 条回复 = %q, want markdown", got.Data.Replies[0].ContentFormat)
		}
		if got.Data.Replies[1].ContentFormat != "text" {
			t.Errorf("第 2 条回复 = %q, want text", got.Data.Replies[1].ContentFormat)
		}
	})

	t.Run("列表回传格式", func(t *testing.T) {
		w := do(authorTok, http.MethodGet, "/api/forum/topics?scope=all&page=1&page_size=50", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("期望 200, got %d %s", w.Code, w.Body.String())
		}
		var got contentFormatListResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		seen := map[int64]string{}
		for _, tp := range got.Data.Topics {
			seen[tp.ID] = tp.ContentFormat
		}
		if seen[topicID] != "markdown" {
			t.Errorf("列表里载体帖 = %q, want markdown", seen[topicID])
		}
	})

	t.Run("编辑帖子不改变格式（移动端编辑不得静默重置）", func(t *testing.T) {
		w := do(authorTok, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
			"category": "question", "title": "载体帖（已编辑）", "content": "## 步骤v2", "images": []string{},
		})
		if w.Code != http.StatusOK {
			t.Fatalf("编辑期望 200, got %d %s", w.Code, w.Body.String())
		}
		var raw struct {
			Data struct {
				ContentFormat string `json:"content_format"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &raw)
		if raw.Data.ContentFormat != "markdown" {
			t.Errorf("编辑后响应 content_format = %q, want markdown（不得被重置）", raw.Data.ContentFormat)
		}
		// 以详情为准再确认一次（响应体形态不稳时以读取路径为事实源）。
		d := do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", topicID), nil)
		var detail contentFormatDetailResp
		_ = json.Unmarshal(d.Body.Bytes(), &detail)
		if detail.Data.Topic.ContentFormat != "markdown" {
			t.Errorf("编辑后详情 content_format = %q, want markdown", detail.Data.Topic.ContentFormat)
		}
	})
}
