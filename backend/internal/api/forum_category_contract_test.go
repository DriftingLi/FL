// 契约测试 #364：论坛类别分流（discussion / question）。
//
// 本文件守的核心陷阱：现有 scope=general 的定义就是 chapter_id IS NULL，
// 而问答帖按设计 chapter_id 恒为 NULL —— 问答帖天然满足综合讨论区的筛选条件。
// 列表查询若漏掉 category（或只在应用层事后过滤），问答帖会整片灌进讨论 Tab。
// 因此 TestForumCategorySplitContract 里"讨论 Tab 结果集不含已落库问答帖"这条断言
// 是本票存在的意义，不要删。
//
// 注意：迁移 SQL 给主题表加的 CHECK (category <> 'question' OR chapter_id IS NULL)
// 不在本文件覆盖范围内 —— 测试库由 AutoMigrate 建表、不执行 migrations/ 下的 SQL，
// 在此写"违反约束被拒"的测试只会因为约束不存在而假绿。该约束由 migration-check 验证。
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

// topicCategoryResp 发帖响应（只取本测试关心的字段）。
type topicCategoryResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID        int64  `json:"id"`
		Category  string `json:"category"`
		ChapterID *int   `json:"chapter_id"`
	} `json:"data"`
}

// topicListResp 列表响应（只取本测试关心的字段）。
type topicListResp struct {
	Code int `json:"code"`
	Data struct {
		Total  int64 `json:"total"`
		Topics []struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			Category string `json:"category"`
		} `json:"topics"`
	} `json:"data"`
}

// titles 返回列表里的标题集合，便于做包含/排除断言。
func (r topicListResp) titles() map[string]bool {
	out := make(map[string]bool, len(r.Data.Topics))
	for _, tp := range r.Data.Topics {
		out[tp.Title] = true
	}
	return out
}

func TestForumCategorySplitContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "cat_author", Phone: "13800000101", Username: "分类作者", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	// 真实章节：让"问答帖挂章节 → 400"这条测试为正确的理由通过，
	// 否则会因为"章节不存在"而 400，测的就不是 category 规则了。
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
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.CheckInSvc, deps.ForumImageSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	adminToken, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(1, "admin1", "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}

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

	// create 发帖并断言状态码；body 直接透传，nil 表示不带 category。
	create := func(body map[string]any, wantStatus int) topicCategoryResp {
		t.Helper()
		rec := do(token, http.MethodPost, "/api/forum/topics", body)
		var got topicCategoryResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v (body=%s)", err, rec.Body.String())
		}
		if rec.Code != wantStatus {
			t.Fatalf("发帖 %v 状态码 = %d, 期望 %d (message=%s)", body, rec.Code, wantStatus, got.Message)
		}
		return got
	}

	listOn := func(tok, path, query string) topicListResp {
		t.Helper()
		rec := do(tok, http.MethodGet, path+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %s%s 状态码 = %d, body=%s", path, query, rec.Code, rec.Body.String())
		}
		var got topicListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		return got
	}

	list := func(tok, query string) topicListResp {
		t.Helper()
		return listOn(tok, "/api/forum/topics", query)
	}

	// 1. 不传 category 向后兼容：归为 discussion，且不破坏移动端既有语义。
	legacy := create(map[string]any{"title": "无类别旧帖", "content": "移动端这样发"}, http.StatusCreated)
	if legacy.Data.Category != "discussion" {
		t.Fatalf("不传 category 应归一为 discussion, 实际 %q", legacy.Data.Category)
	}
	if legacy.Data.ChapterID != nil {
		t.Fatalf("不传 chapter_id 应在综合讨论区, 实际 %v", *legacy.Data.ChapterID)
	}

	// 2. 显式发问答帖：不带章节。
	q := create(map[string]any{"category": "question", "title": "液压油温偏高怎么办", "content": "求解答"}, http.StatusCreated)
	if q.Data.Category != "question" {
		t.Fatalf("问答帖 category = %q, 期望 question", q.Data.Category)
	}
	if q.Data.ChapterID != nil {
		t.Fatalf("问答帖不应挂章节, 实际 chapter_id=%d", *q.Data.ChapterID)
	}

	// 3. 讨论帖挂章节（现状行为一行都不改）。
	ch := create(map[string]any{"category": "discussion", "chapter_id": 77, "title": "章节讨论帖", "content": "章节内讨论"}, http.StatusCreated)
	if ch.Data.ChapterID == nil || *ch.Data.ChapterID != 77 {
		t.Fatalf("章节讨论帖 chapter_id = %v, 期望 77", ch.Data.ChapterID)
	}

	// 4. 既有归一语义不得收紧：chapter_id 传 0 仍视为综合讨论区（移动端按 API.md 传 0）。
	if z := create(map[string]any{"title": "零章节旧帖", "content": "传 0", "chapter_id": 0}, http.StatusCreated); z.Data.ChapterID != nil {
		t.Fatalf("chapter_id=0 应归一为综合区, 实际 %v", *z.Data.ChapterID)
	}

	// 5. 非法 category 值 → 400，不落库。
	bad := create(map[string]any{"category": "job_wanted", "title": "非法类别", "content": "x"}, http.StatusBadRequest)
	if bad.Data.ID != 0 {
		t.Fatalf("非法 category 不应落库")
	}

	// 6. 非法组合：category=question 且 chapter_id>0 → 400（章节真实存在，故 400 只可能来自 category 规则）。
	illegal := create(map[string]any{"category": "question", "chapter_id": 77, "title": "非法组合", "content": "x"}, http.StatusBadRequest)
	if illegal.Data.ID != 0 {
		t.Fatalf("非法组合不应落库")
	}
	if illegal.Message == "" {
		t.Fatalf("非法组合应返回可读错误信息")
	}

	// 7. ★ 本票核心回归护栏：讨论 Tab（scope=general + category=discussion）
	//    绝不能出现已落库的问答帖 —— 因为问答帖的 chapter_id 也是 NULL。
	discussionTab := list(token, "?scope=general&category=discussion")
	if got := discussionTab.titles(); got["液压油温偏高怎么办"] {
		t.Fatalf("叠加陷阱复现：问答帖灌进了讨论 Tab（scope=general + category=discussion 命中了 chapter_id IS NULL 的问答帖）")
	} else if !got["无类别旧帖"] {
		t.Fatalf("讨论 Tab 应含综合区讨论帖, 实际 %v", discussionTab.Data.Topics)
	}
	if got := discussionTab.titles(); got["章节讨论帖"] {
		t.Fatalf("讨论 Tab 的 scope=general 语义变了：不应含章节讨论帖（现状只综合区）")
	}

	// 8. 问答 Tab：只按 category=question 分流，且讨论帖不出现。
	qaTab := list(token, "?category=question")
	if got := qaTab.titles(); !got["液压油温偏高怎么办"] {
		t.Fatalf("问答 Tab 应含问答帖, 实际 %v", qaTab.Data.Topics)
	} else if got["无类别旧帖"] || got["章节讨论帖"] {
		t.Fatalf("讨论帖串进了问答 Tab")
	}
	for _, tp := range qaTab.Data.Topics {
		if tp.Category != "question" {
			t.Fatalf("问答 Tab 返回了 category=%q 的帖子", tp.Category)
		}
	}

	// 9. 不传 category 时行为与现状一致：两类都看得到（向后兼容）。
	all := list(token, "")
	if all.Data.Total != 4 {
		t.Fatalf("不传 category 应看到全部 4 条（含章节讨论帖）, 实际 total=%d", all.Data.Total)
	}

	// 10. 列表里每条都应带 category，前端分段渲染靠它。
	for _, tp := range all.Data.Topics {
		if tp.Category != "discussion" && tp.Category != "question" {
			t.Fatalf("列表项 %q 缺少合法 category, 实际 %q", tp.Title, tp.Category)
		}
	}

	// 11. 管理端列表（独立路径 /api/admin/forum/topics，复用同一 handler）：
	//     能单独审问答区，综合讨论区子 tab 不再混入问答帖。
	adminQA := listOn(adminToken, "/api/admin/forum/topics", "?category=question")
	if got := adminQA.titles(); !got["液压油温偏高怎么办"] || len(adminQA.Data.Topics) != 1 {
		t.Fatalf("管理端按 category=question 过滤未生效: %+v", adminQA.Data.Topics)
	}
	adminGeneral := listOn(adminToken, "/api/admin/forum/topics", "?scope=general&category=discussion")
	if got := adminGeneral.titles(); got["液压油温偏高怎么办"] {
		t.Fatalf("管理端综合讨论区混入了问答帖")
	}
	// 管理端不带 category 时仍看全部（现状行为不改）。
	if all := listOn(adminToken, "/api/admin/forum/topics", ""); all.Data.Total != 4 {
		t.Fatalf("管理端不带 category 应看全部 4 条, 实际 %d", all.Data.Total)
	}

	// 12. 非法 category 作为查询参数同样拒绝，不静默当成"不过滤"。
	if rec := do(token, http.MethodGet, "/api/forum/topics?category=bogus", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("列表传非法 category 应 400, 实际 %d", rec.Code)
	}

	fmt.Println("论坛类别分流契约通过：讨论/问答各自列表，叠加陷阱与管理端过滤均已守住")
}
