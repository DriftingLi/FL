// 契约测试 #811：作者本人编辑帖子 PUT /api/forum/topics/:id。
//
// 守五件事（票面验收 + 与发帖同构的不变量）：
//  1. 权限：仅作者本人 —— 非本人 403（ErrNotTopicOwner 哨兵，errors.Is 映射）；
//  2. 不存在 404（ErrTopicNotFound 哨兵）；非法类别 400（走 normalizeForumCategory，文案「帖子类别无效: X」）；
//  3. 更新生效回显：title/content/category 落库并随 DTO（详情响应形态）返回；
//  4. 与发帖同构的不变量：既有章节帖改为 question 被拒（编辑不迁移章节归属，按既有行判定）；
//  5. 向后兼容：category 传空串归一 discussion（移动端旧契约不传即此路径）。
//
// 注：category 值域与「question 不得挂章节」的 DB CHECK 只在迁移 000005、测试库
// AutoMigrate 不建，行为层必须由 service 校验守住——本文件即该行为层的契约。
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

// updateTopicResp 编辑响应（取本测试关心的字段）。
type updateTopicResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID        int64  `json:"id"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		ChapterID *int   `json:"chapter_id"`
	} `json:"data"`
}

func TestForumUpdateTopicContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "edit_author", Phone: "13800000106", Username: "作者", Status: 1, CreatedAt: testutil.Now()}
	other := model.HrwaiUser{Account: "edit_other", Phone: "13800000107", Username: "路人", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("创建路人失败: %v", err)
	}
	chapter := model.Chapter{ChapterID: 77, CourseID: 1, Title: "液压系统", CreatedAt: testutil.Now()}
	if err := db.Create(&chapter).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

	tokenOf := func(u model.HrwaiUser) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	authorToken := tokenOf(author)
	otherToken := tokenOf(other)

	do := func(tok, method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	create := func(body map[string]any) int64 {
		t.Helper()
		rec := do(authorToken, http.MethodPost, "/api/forum/topics", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("发帖应 201，实际 %d %s", rec.Code, rec.Body.String())
		}
		var got updateTopicResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v", err)
		}
		return got.Data.ID
	}

	// 素材：一篇普通讨论帖 + 一篇挂章节的讨论帖
	topicID := create(map[string]any{"title": "原标题", "content": "原内容"})
	chapterTopicID := create(map[string]any{"chapter_id": 77, "title": "章节帖", "content": "x"})

	// 1. 本人更新成功：字段落库 + DTO 回显
	rec := do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
		"title": "改后标题", "content": "改后内容", "category": "experience",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("本人编辑应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	var updated updateTopicResp
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("解析编辑响应失败: %v", err)
	}
	if updated.Data.Title != "改后标题" || updated.Data.Content != "改后内容" || updated.Data.Category != "experience" {
		t.Fatalf("编辑回显不符：title=%q content=%q category=%q", updated.Data.Title, updated.Data.Content, updated.Data.Category)
	}
	// 落库核对（不经响应自证）
	var row model.ForumTopic
	if err := db.First(&row, topicID).Error; err != nil {
		t.Fatalf("回读主题失败: %v", err)
	}
	if row.Title != "改后标题" || row.Category != "experience" {
		t.Fatalf("落库不符：title=%q category=%q", row.Title, row.Category)
	}

	// 2. 空类别归一 discussion（向后兼容：移动端旧契约不传 category）
	rec = do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
		"title": "回归讨论", "content": "内容",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("不传 category 应 200（归一 discussion），实际 %d %s", rec.Code, rec.Body.String())
	}
	var normResp updateTopicResp
	_ = json.Unmarshal(rec.Body.Bytes(), &normResp)
	if normResp.Data.Category != "discussion" {
		t.Fatalf("空类别应归一 discussion，实际 %q", normResp.Data.Category)
	}

	// 3. 非本人 → 403（owner 哨兵）
	if rec := do(otherToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
		"title": "篡改", "content": "x",
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("非本人编辑应 403，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 4. 非法类别 → 400（文案沿用固定口径）
	rec = do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
		"title": "x", "content": "y", "category": "bogus",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法类别应 400，实际 %d", rec.Code)
	}
	var invalidResp updateTopicResp
	_ = json.Unmarshal(rec.Body.Bytes(), &invalidResp)
	if invalidResp.Message != "帖子类别无效: bogus" {
		t.Fatalf("非法类别文案应为「帖子类别无效: bogus」，实际 %q", invalidResp.Message)
	}

	// 5. 主题不存在 → 404（not-found 哨兵）
	if rec := do(authorToken, http.MethodPut, "/api/forum/topics/999999", map[string]any{
		"title": "x", "content": "y",
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 6. 与发帖同构的不变量：既有章节帖改为 question 被拒（编辑不迁移章节归属）
	rec = do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", chapterTopicID), map[string]any{
		"title": "章节帖改问答", "content": "x", "category": "question",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("章节帖改 question 应 400，实际 %d %s", rec.Code, rec.Body.String())
	}
	// 反证：章节帖保持 discussion/experience 可正常更新
	if rec := do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", chapterTopicID), map[string]any{
		"title": "章节帖改经验", "content": "x", "category": "experience",
	}); rec.Code != http.StatusOK {
		t.Fatalf("章节帖改 experience 应 200，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 7. 长度越界 → 400（空标题）
	if rec := do(authorToken, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), map[string]any{
		"title": "   ", "content": "内容",
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("空标题应 400，实际 %d", rec.Code)
	}

	// 8. 未认证 → 401（路由挂在 JWTAuth 组内）
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), bytes.NewReader([]byte(`{"title":"x","content":"y"}`)))
	req.Header.Set("Content-Type", "application/json")
	plain := httptest.NewRecorder()
	r.ServeHTTP(plain, req)
	if plain.Code != http.StatusUnauthorized {
		t.Fatalf("未认证应 401，实际 %d", plain.Code)
	}

	fmt.Println("编辑帖子契约通过：本人/空类别归一/非本人403/非法类别400/不存在404/章节帖不变量/长度越界/未认证 401 均守住")
}
