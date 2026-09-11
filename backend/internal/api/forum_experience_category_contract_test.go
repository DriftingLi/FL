// 契约测试：备考经验类别的**遗留读侧语义**与意图写入收窄（ADR-0040）。
//
// ADR-0040 把「意图」与「认定」拆开：category 只表达作者自述的意图（discussion | question），
// 「备考经验」改为管理端认定（is_experience，后续批次落地）。本文件守四件事：
//  1. **学员不能自称经验**：发帖传 category=experience 一律 400（写入路径已收窄）。
//  2. **遗留行仍可读**：直接落库的 category='experience' 存量行，列表 category=experience 仍能取到——
//     降级迁移落地前它们还在库里，经验 Tab 不能瞬间瞎掉。挂章节的遗留行同样可读。
//  3. 叠加陷阱第二格：遗留经验行的 chapter_id 同样可为 NULL，
//     scope=general + category=discussion 与 category=question 两条 Tab 查询都不得混入。
//  4. sort=created（#722 排序档）按 created_at 排，id 兜底打破同刻并列。
//
// 与 forum_category_contract_test.go 同构：复用其响应类型与 deps 装配；
// CHECK 约束不在本文件覆盖范围（测试库 AutoMigrate 建表、无约束）。
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

func TestForumExperienceCategoryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "exp_author", Phone: "13800000102", Username: "经验作者", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	// 真实章节：让「遗留经验帖挂章节仍可读」这条测试为正确的理由通过，
	// 否则会因为「章节不存在」而 400，测的就不是类别规则了。
	chapter := model.Chapter{ChapterID: 78, CourseID: 1, Title: "电气系统", CreatedAt: testutil.Now()}
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

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
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

	createOK := func(body map[string]any) topicCategoryResp {
		t.Helper()
		rec := do(token, http.MethodPost, "/api/forum/topics", body)
		var got topicCategoryResp
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != http.StatusCreated {
			t.Fatalf("发帖 %v 状态码 = %d, 期望 201 (message=%s)", body, rec.Code, got.Message)
		}
		return got
	}

	list := func(query string) topicListResp {
		t.Helper()
		rec := do(token, http.MethodGet, "/api/forum/topics"+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %s 状态码 = %d, body=%s", query, rec.Code, rec.Body.String())
		}
		var got topicListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		return got
	}

	// 辅助：直接落库种一篇遗留经验帖（绕过已被收窄的发帖路径）。
	seedLegacyExperience := func(title string, chapterID *int, createdAt time.Time) int64 {
		t.Helper()
		tp := model.ForumTopic{
			ChapterID: chapterID, Category: "experience", UserID: author.ID,
			Title: title, Content: "存量内容", Images: model.JSONB("[]"),
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		if err := db.Create(&tp).Error; err != nil {
			t.Fatalf("种子遗留经验帖失败: %v", err)
		}
		return tp.ID
	}

	// 1. ★ ADR-0040：学员不能自称「备考经验」——发帖传 experience 一律 400。
	//    旧实现把 experience 当第三类别放行，本组即该收窄的守卫。
	rec := do(token, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "experience", "title": "叉车证备考心得", "content": "一次性通过"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("发帖传 experience 应 400（学员不能自称经验），实际 %d %s", rec.Code, rec.Body.String())
	}
	// 1b. 挂章节的 experience 同样被拒：拒绝发生在类别值域，与章节归属无关。
	rec = do(token, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "experience", "chapter_id": 78, "title": "章节内备考笔记", "content": "挂在章节下"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("挂章节的 experience 也应 400，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 2. 对照组：问答帖与无类别讨论帖经 API 正常创建。
	createOK(map[string]any{"category": "question", "title": "理论考试怎么预约", "content": "求解答"})
	createOK(map[string]any{"title": "普通讨论帖", "content": "无类别"})

	// 3. 遗留经验行（直接落库）：一篇不挂章节、一篇挂章节。
	base := testutil.Now()
	chapterID := 78
	expID1 := seedLegacyExperience("叉车证备考心得", nil, base.Add(-2*time.Hour))
	_ = seedLegacyExperience("章节内备考笔记", &chapterID, base.Add(-time.Hour))

	// 4. 列表 category=experience 只出遗留经验帖（含挂章节的那篇）。
	expTab := list("?category=experience")
	if expTab.Data.Total != 2 {
		t.Fatalf("经验列表应恰 2 条, 实际 total=%d", expTab.Data.Total)
	}
	for _, tp := range expTab.Data.Topics {
		if tp.Category != "experience" {
			t.Fatalf("经验列表返回了 category=%q 的帖子 %q", tp.Category, tp.Title)
		}
	}
	if got := expTab.titles(); !got["叉车证备考心得"] || !got["章节内备考笔记"] {
		t.Fatalf("经验列表应含两条遗留经验帖, 实际 %v", expTab.Data.Topics)
	}

	// 5. ★ 叠加陷阱第二格：讨论 Tab（scope=general + category=discussion）
	//    绝不能出现经验帖 —— 因为经验帖的 chapter_id 也可为 NULL。
	discussionTab := list("?scope=general&category=discussion")
	if got := discussionTab.titles(); got["叉车证备考心得"] || got["章节内备考笔记"] {
		t.Fatalf("叠加陷阱复现：经验帖灌进了讨论 Tab")
	} else if !got["普通讨论帖"] {
		t.Fatalf("讨论 Tab 应含综合区讨论帖, 实际 %v", discussionTab.Data.Topics)
	}

	// 6. 问答 Tab 同理：经验帖不得串入。
	qaTab := list("?category=question")
	if got := qaTab.titles(); got["叉车证备考心得"] || got["章节内备考笔记"] {
		t.Fatalf("经验帖灌进了问答 Tab")
	}
	for _, tp := range qaTab.Data.Topics {
		if tp.Category != "question" {
			t.Fatalf("问答 Tab 返回了 category=%q 的帖子", tp.Category)
		}
	}

	// 7. 不带 category：三类皆可见（2 遗留经验 + 1 问答 + 1 讨论）。
	all := list("")
	if all.Data.Total != 4 {
		t.Fatalf("不带 category 应看到全部 4 条（三类）, 实际 total=%d", all.Data.Total)
	}

	// 8. 采纳闸门不动：经验帖不可被采纳（category 校验先于 reply 存在性，reply_id 传 999 即可触发）。
	acc := do(token, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", expID1), map[string]any{"reply_id": 999})
	if acc.Code != http.StatusBadRequest {
		t.Fatalf("经验帖采纳应 400, 实际 %d %s", acc.Code, acc.Body.String())
	}

	// 9. sort=created（#722 排序档）：按发帖时间排，id 兜底打破同刻并列。
	_ = seedLegacyExperience("更晚发的经验帖", nil, base)
	byCreatedDesc := list("?category=experience&sort=created&order=desc")
	if len(byCreatedDesc.Data.Topics) < 2 || byCreatedDesc.Data.Topics[0].Title != "更晚发的经验帖" {
		t.Fatalf("sort=created desc 应以后发的经验帖置顶, 实际 %+v", byCreatedDesc.Data.Topics)
	}
	byCreatedAsc := list("?category=experience&sort=created&order=asc")
	if len(byCreatedAsc.Data.Topics) < 2 || byCreatedAsc.Data.Topics[0].Title != "叉车证备考心得" {
		t.Fatalf("sort=created asc 应以先发的经验帖置顶, 实际 %+v", byCreatedAsc.Data.Topics)
	}

	fmt.Println("备考经验遗留语义契约通过：意图收窄 400/遗留行可读/挂章节/采纳闸门/sort=created 均守住")
}
