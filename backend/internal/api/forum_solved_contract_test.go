// 契约测试 #367：问答 solved 筛选与详情采纳标记.
//
// 覆盖：solved=unsolved / solved / all 三档过滤正确；
// 已解决帖的详情中 accepted_reply_id 与某条回复 id 一致且该回复 is_accepted=true；
// 非法 solved 参数 400；
// 取消采纳后状态回退到求助。
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

func TestForumSolvedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "solved_author", Phone: "13800000201", Username: "楼主", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建楼主失败: %v", err)
	}
	answerer := model.HrwaiUser{Account: "solved_ans", Phone: "13800000202", Username: "答主", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&answerer).Error; err != nil {
		t.Fatalf("创建答主失败: %v", err)
	}
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

	issueToken := func(u model.HrwaiUser) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	authorTok := issueToken(author)
	ansTok := issueToken(answerer)

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

	createTopic := func(title string) int64 {
		rec := do(authorTok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "question", "title": title, "content": "求助内容"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("创建问答帖 %q 失败: %d %s", title, rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析创建帖失败: %v", err)
		}
		return got.Data.ID
	}
	q1 := createTopic("求助帖-未解决")
	q2 := createTopic("求助帖-待采纳")

	rec := do(ansTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", q2), map[string]any{"content": "答案"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建回复失败: %d %s", rec.Code, rec.Body.String())
	}
	var replyGot struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &replyGot); err != nil {
		t.Fatalf("解析回复失败: %v", err)
	}
	replyID := replyGot.Data.ID

	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", q2), map[string]any{"reply_id": replyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("采纳失败: %d %s", rec.Code, rec.Body.String())
	}

	listTitles := func(query string) map[string]bool {
		rec := do(authorTok, http.MethodGet, "/api/forum/topics"+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %q 失败: %d %s", query, rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				Topics []struct {
					ID    int64  `json:"id"`
					Title string `json:"title"`
				} `json:"topics"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		m := make(map[string]bool)
		for _, tp := range got.Data.Topics {
			m[tp.Title] = true
		}
		return m
	}

	if got := listTitles("?category=question&solved=unsolved"); got["求助帖-待采纳"] || !got["求助帖-未解决"] {
		t.Fatalf("solved=unsolved 过滤错误: %+v", got)
	}
	if got := listTitles("?category=question&solved=solved"); !got["求助帖-待采纳"] || got["求助帖-未解决"] {
		t.Fatalf("solved=solved 过滤错误: %+v", got)
	}
	if got := listTitles("?category=question&solved=all"); !got["求助帖-未解决"] || !got["求助帖-待采纳"] {
		t.Fatalf("solved=all 应含全部: %+v", got)
	}
	if got := listTitles("?category=question"); !got["求助帖-未解决"] || !got["求助帖-待采纳"] {
		t.Fatalf("不传 solved 应含全部: %+v", got)
	}
	if rec := do(authorTok, http.MethodGet, "/api/forum/topics?category=question&solved=bogus", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 solved 应 400, got %d", rec.Code)
	}

	rec = do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", q2), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("详情失败: %d %s", rec.Code, rec.Body.String())
	}
	var det struct {
		Code int `json:"code"`
		Data struct {
			Topic struct {
				ID              int64   `json:"id"`
				AcceptedReplyID *int64  `json:"accepted_reply_id"`
				SolvedAt        *string `json:"solved_at"`
			} `json:"topic"`
			Replies []struct {
				ID         int64 `json:"id"`
				IsAccepted bool  `json:"is_accepted"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &det); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	if det.Data.Topic.AcceptedReplyID == nil || *det.Data.Topic.AcceptedReplyID != replyID {
		t.Fatalf("详情 accepted_reply_id 应为 %d, got %v", replyID, det.Data.Topic.AcceptedReplyID)
	}
	if det.Data.Topic.SolvedAt == nil || *det.Data.Topic.SolvedAt == "" {
		t.Fatalf("已解决帖 solved_at 不能为空")
	}
	found := false
	for _, rp := range det.Data.Replies {
		if rp.ID == replyID && rp.IsAccepted {
			found = true
		}
		if rp.ID != replyID && rp.IsAccepted {
			t.Fatalf("非采纳回复 %d 不应标记 is_accepted", rp.ID)
		}
	}
	if !found {
		t.Fatalf("采纳回复 %d 未标记 is_accepted", replyID)
	}

	rec = do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", q1), nil)
	var det2 struct {
		Code int `json:"code"`
		Data struct {
			Topic struct {
				AcceptedReplyID *int64 `json:"accepted_reply_id"`
			} `json:"topic"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &det2); err != nil {
		t.Fatalf("解析求助详情失败: %v", err)
	}
	if det2.Data.Topic.AcceptedReplyID != nil {
		t.Fatalf("求助帖不应有 accepted_reply_id, got %v", *det2.Data.Topic.AcceptedReplyID)
	}

	rec = do(authorTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", q2), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消采纳失败: %d %s", rec.Code, rec.Body.String())
	}
	if got := listTitles("?category=question&solved=solved"); got["求助帖-待采纳"] {
		t.Fatalf("取消后 solved 集合不应再含该帖")
	}
	if got := listTitles("?category=question&solved=unsolved"); !got["求助帖-待采纳"] {
		t.Fatalf("取消后 unsolved 应含该帖")
	}

	fmt.Println("问答 solved 契约通过：筛选与详情采纳标记均已守住")
}
