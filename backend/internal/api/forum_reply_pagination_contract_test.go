// #854 契约测试：详情接口回复分页 + 被采纳回复恒占首页第一条（ADR-0042）。
//
// 覆盖不变式：
//  1. `page`/`page_size` 生效，响应 `{ topic, replies, page, pages, total }`（topic 与 replies 平级）。
//  2. **置顶优先于排序**：被采纳回复占首页第一条，且**不重复出现**在其余页。
//  3. 首页容量 = page_size − 1（有采纳回复时）——等价于「[置顶] + [其余按序] 拼成一条流后按 page_size 切块」。
//  4. `total` = 回复总数（含置顶条），与 `topic.reply_count` 一致。
//  5. `page` 越界返回**空数组**而非 nil/报错（前端据此走空态）。
//  6. 旧的全量路径已退役：不传 page_size 时默认 20，而非「返回全部」。
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

type replyPageResp struct {
	Code int `json:"code"`
	Data struct {
		Page  int   `json:"page"`
		Pages int   `json:"pages"`
		Total int64 `json:"total"`
		Topic struct {
			ID         int64 `json:"id"`
			ReplyCount int   `json:"reply_count"`
		} `json:"topic"`
		Replies []struct {
			ID         int64 `json:"id"`
			IsAccepted bool  `json:"is_accepted"`
		} `json:"replies"`
	} `json:"data"`
}

func TestForumReplyPaginationContract(t *testing.T) {
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
	author := mkUser("rp_author", "13800000701", "楼主")
	answerer := mkUser("rp_answerer", "13800000702", "答主")

	topic := model.ForumTopic{
		UserID: int(author.ID), Category: "question", Title: "求助", Content: "如题",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建主题失败: %v", err)
	}

	// 25 条回复，按 id 递增即时间递增（排序取 created_at ASC, id ASC）。
	const replyTotal = 25
	replyIDs := make([]int64, 0, replyTotal)
	for i := 0; i < replyTotal; i++ {
		rp := model.ForumReply{
			TopicID: topic.ID, UserID: int(answerer.ID),
			Content:   fmt.Sprintf("回复 %d", i+1),
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := db.Create(&rp).Error; err != nil {
			t.Fatalf("建回复失败: %v", err)
		}
		replyIDs = append(replyIDs, rp.ID)
	}
	if err := db.Model(&model.ForumTopic{}).Where("id = ?", topic.ID).
		Update("reply_count", replyTotal).Error; err != nil {
		t.Fatalf("回填 reply_count 失败: %v", err)
	}

	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(answerer.ID), answerer.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	getPage := func(query string) replyPageResp {
		req, _ := http.NewRequest(http.MethodGet, "/api/forum/topics/"+fmt.Sprint(topic.ID)+query, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s 期望 200, got %d %s", query, w.Code, w.Body.String())
		}
		var got replyPageResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析 %s 失败: %v", query, err)
		}
		if got.Data.Replies == nil {
			t.Fatalf("GET %s 的 replies 为 null，应为数组", query)
		}
		return got
	}
	idsOf := func(p replyPageResp) []int64 {
		out := make([]int64, 0, len(p.Data.Replies))
		for _, rp := range p.Data.Replies {
			out = append(out, rp.ID)
		}
		return out
	}

	t.Run("无采纳回复时首页容量等于 page_size", func(t *testing.T) {
		got := getPage("?page=1&page_size=10")
		if len(got.Data.Replies) != 10 {
			t.Fatalf("首页应 10 条, got %d", len(got.Data.Replies))
		}
		if got.Data.Total != replyTotal {
			t.Fatalf("total = %d, want %d", got.Data.Total, replyTotal)
		}
		if got.Data.Pages != 3 {
			t.Fatalf("pages = %d, want 3", got.Data.Pages)
		}
		if got.Data.Topic.ReplyCount != replyTotal {
			t.Fatalf("topic.reply_count = %d, want %d", got.Data.Topic.ReplyCount, replyTotal)
		}
		if got.Data.Topic.ID != topic.ID {
			t.Fatalf("topic.id = %d, want %d", got.Data.Topic.ID, topic.ID)
		}
		// 分页要有真实切片：首页应是前 10 条而非全量。
		if idsOf(got)[0] != replyIDs[0] {
			t.Fatalf("首页首条 = %d, want %d", idsOf(got)[0], replyIDs[0])
		}
	})

	t.Run("默认 page_size 为 20 而非全量", func(t *testing.T) {
		got := getPage("")
		if len(got.Data.Replies) != 20 {
			t.Fatalf("默认每页应 20 条（旧全量路径已退役）, got %d", len(got.Data.Replies))
		}
	})

	// 采纳第 13 条（既不在首位也不在末位，能区分「置顶」与「恰好排在前」）。
	acceptedIdx := 12
	acceptedID := replyIDs[acceptedIdx]
	if err := db.Model(&model.ForumTopic{}).Where("id = ?", topic.ID).
		Updates(map[string]any{"accepted_reply_id": acceptedID, "solved_at": now}).Error; err != nil {
		t.Fatalf("采纳失败: %v", err)
	}

	t.Run("被采纳回复占首页第一条", func(t *testing.T) {
		got := getPage("?page=1&page_size=10&sort=latest&order=asc")
		if len(got.Data.Replies) == 0 {
			t.Fatal("首页不应为空")
		}
		first := got.Data.Replies[0]
		if first.ID != acceptedID {
			t.Fatalf("首页第一条 = %d, want 被采纳的 %d", first.ID, acceptedID)
		}
		if !first.IsAccepted {
			t.Fatal("首页第一条应带 is_accepted")
		}
	})

	t.Run("置顶占一格：首页总条数仍为 page_size", func(t *testing.T) {
		got := getPage("?page=1&page_size=10&sort=latest&order=asc")
		if len(got.Data.Replies) != 10 {
			t.Fatalf("首页应 10 条（1 置顶 + 9 其余）, got %d", len(got.Data.Replies))
		}
	})

	t.Run("置顶不重复出现且翻页无重叠无遗漏", func(t *testing.T) {
		seen := map[int64]int{}
		for page := 1; page <= 3; page++ {
			got := getPage(fmt.Sprintf("?page=%d&page_size=10&sort=latest&order=asc", page))
			for _, id := range idsOf(got) {
				seen[id]++
			}
		}
		if len(seen) != replyTotal {
			t.Fatalf("三页合计覆盖 %d 条, want %d（有重叠或遗漏）", len(seen), replyTotal)
		}
		for id, n := range seen {
			if n != 1 {
				t.Fatalf("回复 %d 出现了 %d 次, want 1", id, n)
			}
		}
	})

	t.Run("total 与 topic.reply_count 同源", func(t *testing.T) {
		got := getPage("?page=2&page_size=10&sort=latest&order=asc")
		if got.Data.Total != replyTotal {
			t.Fatalf("total = %d, want %d", got.Data.Total, replyTotal)
		}
		if got.Data.Total != int64(got.Data.Topic.ReplyCount) {
			t.Fatalf("total=%d 与 reply_count=%d 不一致", got.Data.Total, got.Data.Topic.ReplyCount)
		}
	})

	t.Run("越界页返回空数组而非报错", func(t *testing.T) {
		got := getPage("?page=99&page_size=10")
		if len(got.Data.Replies) != 0 {
			t.Fatalf("越界页应空数组, got %d 条", len(got.Data.Replies))
		}
		if got.Data.Total != replyTotal {
			t.Fatalf("越界页 total 仍应为 %d, got %d", replyTotal, got.Data.Total)
		}
	})

	t.Run("置顶与排序正交：倒序时仍占首页第一条", func(t *testing.T) {
		got := getPage("?page=1&page_size=10&sort=latest&order=desc")
		if len(got.Data.Replies) == 0 || got.Data.Replies[0].ID != acceptedID {
			t.Fatalf("倒序首页第一条应为被采纳的 %d, got %v", acceptedID, idsOf(got))
		}
		// 倒序下第二条应是最后一条回复（排序轴生效）
		if len(got.Data.Replies) > 1 && got.Data.Replies[1].ID != replyIDs[replyTotal-1] {
			t.Fatalf("倒序第二条 = %d, want %d", got.Data.Replies[1].ID, replyIDs[replyTotal-1])
		}
	})

	// 越界页的 replies 必须是空数组而非 nil——前端据此走空态而非崩溃。
	t.Run("越界页 replies 序列化为 []", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/forum/topics/"+fmt.Sprint(topic.ID)+"?page=99&page_size=10", bytes.NewReader(nil))
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var raw struct {
			Data struct {
				Replies json.RawMessage `json:"replies"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if string(raw.Data.Replies) != "[]" {
			t.Fatalf("越界页 replies 应序列化为 [], got %s", raw.Data.Replies)
		}
	})
}
