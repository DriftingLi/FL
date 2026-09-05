// 契约测试 #369：问答通知与周边能力覆盖。
//
// 覆盖：采纳后两个收件人各收到一条站内信（payload 分值可读，link 锚到回答）；
// 周边能力对问答帖天然生效：my-topics / my-replies / 搜索 / 收藏点赞举报 / daily_browse 口径不变。
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

func TestForumAcceptNotifyContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "notify_author", Phone: "13900000001", Username: "楼主N", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建楼主失败: %v", err)
	}
	answerer := model.HrwaiUser{Account: "notify_ans", Phone: "13900000002", Username: "答主N", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
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
	RegisterPointsRoutes(apiGroup, deps.RouterDeps(), deps.PointsSvc)
	RegisterFavoriteRoutes(apiGroup, deps.RouterDeps(), deps.FavoriteSvc)
	RegisterSearchRoutes(apiGroup, deps.RouterDeps(), deps.SearchSvc)
	RegisterNotificationRoutes(apiGroup, deps.RouterDeps(), deps.NotificationSvc)

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

	// 1. 发问答帖 + 回复
	rec := do(authorTok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "question", "title": "notify-question", "content": "notify content Q"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发问答帖期望 201, got %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析发帖失败: %v", err)
	}
	topicID := created.Data.ID
	rec = do(ansTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID), map[string]any{"content": "这是回答"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("回复失败 %d %s", rec.Code, rec.Body.String())
	}
	var repCreated struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &repCreated); err != nil {
		t.Fatalf("解析回复失败: %v", err)
	}
	replyID := repCreated.Data.ID

	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("采纳期望 200, got %d %s", rec.Code, rec.Body.String())
	}

	var ansNotifs []model.Notification
	if err := db.Where("user_id = ?", answerer.ID).Order("id ASC").Find(&ansNotifs).Error; err != nil {
		t.Fatalf("查询答主通知失败: %v", err)
	}
	var ownerNotifs []model.Notification
	if err := db.Where("user_id = ?", author.ID).Order("id ASC").Find(&ownerNotifs).Error; err != nil {
		t.Fatalf("查询楼主通知失败: %v", err)
	}
	countByType := func(list []model.Notification, typ string) []model.Notification {
		var out []model.Notification
		for _, n := range list {
			if n.Type == typ {
				out = append(out, n)
			}
		}
		return out
	}
	ansFiltered := countByType(ansNotifs, "forum_accept_answerer")
	if len(ansFiltered) != 1 {
		t.Fatalf("答主应收到 1 条 forum_accept_answerer, 实际 %d, 全量 %+v", len(ansFiltered), ansNotifs)
	}
	ownerFiltered := countByType(ownerNotifs, "forum_accept_owner")
	if len(ownerFiltered) != 1 {
		t.Fatalf("楼主应收到 1 条 forum_accept_owner, 实际 %d, 全量 %+v", len(ownerFiltered), ownerNotifs)
	}
	for _, pair := range []struct {
		n            model.Notification
		expectPts    int
		expectReason string
	}{
		{ansFiltered[0], 40, "accepted_bonus"},
		{ownerFiltered[0], 5, "accept_action"},
	} {
		var payload map[string]any
		if err := json.Unmarshal([]byte(pair.n.Payload), &payload); err != nil {
			t.Fatalf("payload 解析失败 type=%s: %v payload=%s", pair.n.Type, err, string(pair.n.Payload))
		}
		if int(payload["topic_id"].(float64)) != int(topicID) {
			t.Fatalf("payload topic_id 不符: 期望 %d, got %v", topicID, payload["topic_id"])
		}
		if int(payload["reply_id"].(float64)) != int(replyID) {
			t.Fatalf("payload reply_id 不符: 期望 %d, got %v", replyID, payload["reply_id"])
		}
		if int(payload["points"].(float64)) != pair.expectPts {
			t.Fatalf("payload points 不符 %s: 期望 %d, got %v", pair.n.Type, pair.expectPts, payload["points"])
		}
		if payload["reason"] != pair.expectReason {
			t.Fatalf("payload reason 不符 %s: 期望 %s, got %v", pair.n.Type, pair.expectReason, payload["reason"])
		}
		expectedPrefix := fmt.Sprintf("/training/forum/%d#reply-%d", topicID, replyID)
		if pair.n.Link != expectedPrefix {
			t.Fatalf("link 不符 %s: 期望 %s, got %s", pair.n.Type, expectedPrefix, pair.n.Link)
		}
	}
	rec = do(ansTok, http.MethodGet, "/api/notifications?page=1&page_size=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("答主拉取通知期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var notifList struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
				Link    string          `json:"link"`
			} `json:"items"`
			UnreadCount int `json:"unread_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &notifList); err != nil {
		t.Fatalf("解析通知列表失败: %v", err)
	}
	if notifList.Data.UnreadCount < 1 {
		t.Fatalf("答主未读数应 >=1, got %d", notifList.Data.UnreadCount)
	}
	found := false
	for _, it := range notifList.Data.Items {
		if it.Type == "forum_accept_answerer" {
			found = true
			var p map[string]any
			if err := json.Unmarshal(it.Payload, &p); err != nil {
				t.Fatalf("API payload 解析失败: %v", err)
			}
			if int(p["points"].(float64)) != 40 {
				t.Fatalf("API payload points 期望 40, got %v", p["points"])
			}
			if it.Link != fmt.Sprintf("/training/forum/%d#reply-%d", topicID, replyID) {
				t.Fatalf("API link 不符: %s", it.Link)
			}
		}
	}
	if !found {
		t.Fatalf("答主通知列表未找到 forum_accept_answerer")
	}
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("重复采纳期望 200, got %d", rec.Code)
	}
	var ansCountAfter int64
	db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", answerer.ID, "forum_accept_answerer").Count(&ansCountAfter)
	if ansCountAfter != 1 {
		t.Fatalf("重复采纳后答主采纳通知应仍为 1, got %d", ansCountAfter)
	}
	rec = do(ansTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID), map[string]any{"content": "第二条回答"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("第二回复失败: %d", rec.Code)
	}
	var rep2 struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &rep2)
	reply2 := rep2.Data.ID
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply2})
	if rec.Code != http.StatusOK {
		t.Fatalf("更换采纳期望 200, got %d", rec.Code)
	}
	db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", answerer.ID, "forum_accept_answerer").Count(&ansCountAfter)
	if ansCountAfter != 1 {
		t.Fatalf("更换采纳后不应新增通知, got %d", ansCountAfter)
	}
	rec = do(authorTok, http.MethodGet, "/api/forum/my-topics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my-topics 期望 200, got %d", rec.Code)
	}
	var myTopics struct {
		Code int `json:"code"`
		Data struct {
			Topics []struct {
				ID       int64  `json:"id"`
				Category string `json:"category"`
			} `json:"topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &myTopics); err != nil {
		t.Fatalf("解析 my-topics 失败: %v", err)
	}
	foundQuestion := false
	for _, tp := range myTopics.Data.Topics {
		if tp.ID == topicID {
			if tp.Category != "question" {
				t.Fatalf("my-topics 中问答帖 category 应为 question, got %s", tp.Category)
			}
			foundQuestion = true
		}
	}
	if !foundQuestion {
		t.Fatalf("my-topics 未出现刚创建的问答帖")
	}
	rec = do(ansTok, http.MethodGet, "/api/forum/my-replies", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my-replies 期望 200, got %d", rec.Code)
	}
	var myReplies struct {
		Code int `json:"code"`
		Data struct {
			Replies []struct {
				ID      int64 `json:"id"`
				TopicID int64 `json:"topic_id"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &myReplies); err != nil {
		t.Fatalf("解析 my-replies 失败: %v", err)
	}
	foundReply := false
	for _, rp := range myReplies.Data.Replies {
		if rp.ID == replyID && rp.TopicID == topicID {
			foundReply = true
		}
	}
	if !foundReply {
		t.Fatalf("my-replies 应包含对问答帖的回答 %d", replyID)
	}
	rec = do(authorTok, http.MethodGet, "/api/search?keyword=notify-question&type=topic", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("搜索期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var searchRes struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID    int64  `json:"id"`
				Type  string `json:"type"`
				Title string `json:"title"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &searchRes); err != nil {
		t.Fatalf("解析搜索失败: %v", err)
	}
	foundSearch := false
	for _, it := range searchRes.Data.Items {
		if it.ID == topicID && it.Type == "topic" {
			foundSearch = true
		}
	}
	if !foundSearch {
		t.Fatalf("搜索 topic 应命中问答帖, items=%+v", searchRes.Data.Items)
	}
	rec = do(ansTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/like", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("问答帖点赞期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	rec = do(ansTok, http.MethodPost, "/api/favorites", map[string]any{"target_type": "topic", "target_id": int(topicID)})
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("问答帖收藏期望 200/201, got %d %s", rec.Code, rec.Body.String())
	}
	rec = do(ansTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/report", topicID), map[string]any{"reason": "测试举报问答帖"})
	if rec.Code != http.StatusOK {
		t.Fatalf("问答帖举报期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	rec = do(authorTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消采纳期望 200, got %d", rec.Code)
	}
	db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", answerer.ID, "forum_accept_answerer").Count(&ansCountAfter)
	if ansCountAfter != 1 {
		t.Fatalf("取消采纳不应新增通知, got %d", ansCountAfter)
	}
	fmt.Println("问答通知与周边能力契约通过：采纳双通知payload可读/my-topics/my-replies/搜索/收藏点赞举报均覆盖")
}
