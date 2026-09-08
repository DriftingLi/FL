// 契约测试 #722：备考经验（experience）类别进场。
//
// 守四件事：
//  1. experience 是发帖与列表的合法类别（白名单放行、回显一致），且可挂章节——
//     与 question 的「禁章节」口径相反，防止有人顺手把收紧逻辑带过来。
//  2. 叠加陷阱第二格：经验帖的 chapter_id 同样可为 NULL，
//     scope=general + category=discussion 与 category=question 两条 Tab 查询都不得混入经验帖。
//  3. 采纳闸门不动：仍仅问答帖可被采纳（AcceptReply 的 category 校验）。
//  4. sort=created（#722 新排序档）按 created_at 排，id 兜底打破同刻并列。
//
// 与 forum_category_contract_test.go 同构：复用其响应类型与 deps 装配；
// CHECK 约束依旧不在本文件覆盖范围（测试库 AutoMigrate 建表、无约束），由 migration-check 验证。
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
	// 真实章节：让「经验帖挂章节 → 放行」这条测试为正确的理由通过，
	// 否则会因为「章节不存在」而 400，测的就不是 category 规则了。
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

	// 1. 不带章节的经验帖：201 且回显 experience。
	exp1 := create(map[string]any{"category": "experience", "title": "叉车证备考心得", "content": "一次性通过"}, http.StatusCreated)
	if exp1.Data.Category != "experience" {
		t.Fatalf("经验帖 category = %q, 期望 experience", exp1.Data.Category)
	}
	if exp1.Data.ChapterID != nil {
		t.Fatalf("不带章节的经验帖应在综合区, 实际 chapter_id=%d", *exp1.Data.ChapterID)
	}

	// 2. 经验帖挂章节 → 放行（与 question 口径相反，勿顺手收紧）。
	expCh := create(map[string]any{"category": "experience", "chapter_id": 78, "title": "章节内备考笔记", "content": "挂在章节下"}, http.StatusCreated)
	if expCh.Data.ChapterID == nil || *expCh.Data.ChapterID != 78 {
		t.Fatalf("经验帖挂章节应放行, 实际 chapter_id=%v", expCh.Data.ChapterID)
	}

	// 3. 对照组：问答帖与无类别讨论帖。
	create(map[string]any{"category": "question", "title": "理论考试怎么预约", "content": "求解答"}, http.StatusCreated)
	create(map[string]any{"title": "普通讨论帖", "content": "无类别"}, http.StatusCreated)

	// 4. 列表 category=experience 只出经验帖（含章节经验帖）。
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
		t.Fatalf("经验列表应含两条经验帖, 实际 %v", expTab.Data.Topics)
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

	// 7. 不带 category：三类皆可见。
	all := list("")
	if all.Data.Total != 4 {
		t.Fatalf("不带 category 应看到全部 4 条（三类）, 实际 total=%d", all.Data.Total)
	}

	// 8. 采纳闸门不动：经验帖不可被采纳（category 校验先于 reply 存在性，reply_id 传 999 即可触发）。
	acc := do(token, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", exp1.Data.ID), map[string]any{"reply_id": 999})
	if acc.Code != http.StatusBadRequest {
		t.Fatalf("经验帖采纳应 400, 实际 %d %s", acc.Code, acc.Body.String())
	}

	// 9. sort=created（#722 新排序档）：按发帖时间排，id 兜底打破同刻并列。
	create(map[string]any{"category": "experience", "title": "更晚发的经验帖", "content": "用于排序"}, http.StatusCreated)
	byCreatedDesc := list("?category=experience&sort=created&order=desc")
	if len(byCreatedDesc.Data.Topics) < 2 || byCreatedDesc.Data.Topics[0].Title != "更晚发的经验帖" {
		t.Fatalf("sort=created desc 应以后发的经验帖置顶, 实际 %+v", byCreatedDesc.Data.Topics)
	}
	byCreatedAsc := list("?category=experience&sort=created&order=asc")
	if len(byCreatedAsc.Data.Topics) < 2 || byCreatedAsc.Data.Topics[0].Title != "叉车证备考心得" {
		t.Fatalf("sort=created asc 应以先发的经验帖置顶, 实际 %+v", byCreatedAsc.Data.Topics)
	}

	fmt.Println("备考经验类别契约通过：experience 发帖/列表/挂章节/采纳闸门/sort=created 均守住")
}
