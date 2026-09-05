// 契约测试 #368：问答积分防刷乙档（自答零分、日封顶、配对衰减）
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestForumAcceptCapsContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	doReq := func(r *gin.Engine, tok, method, path string, body any) *httptest.ResponseRecorder {
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
	issueTok := func(cfg *config.Config, u model.HrwaiUser) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return tok
	}
	getBal := func(r *gin.Engine, tok string) int {
		rec := doReq(r, tok, http.MethodGet, "/api/points/balance", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("balance %d %s", rec.Code, rec.Body.String())
		}
		var got balanceResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal balance: %v", err)
		}
		return got.Data.Balance
	}
	createTopic := func(r *gin.Engine, tok string, title string) int64 {
		rec := doReq(r, tok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "question", "title": title, "content": "求解答"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create topic %s: %d %s", title, rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal topic: %v", err)
		}
		return got.Data.ID
	}
	createReply := func(r *gin.Engine, tok string, topicID int64, content string) int64 {
		rec := doReq(r, tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID), map[string]any{"content": content})
		if rec.Code != http.StatusCreated {
			t.Fatalf("reply %s: %d %s", content, rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal reply: %v", err)
		}
		return got.Data.ID
	}
	t.Run("self_rejected", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		cfg := &config.Config{JWTSecretKey: "caps-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
		r := gin.New()
		deps := newContractDeps(t, db, cfg)
		RegisterForumRoutes(r.Group("/api"), deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
		RegisterPointsRoutes(r.Group("/api"), deps.RouterDeps(), deps.PointsSvc)
		author := model.HrwaiUser{Account: "caps_self", Phone: "13800001001", Username: "自答楼主", Status: 1, CreatedAt: testutil.Now()}
		if err := db.Create(&author).Error; err != nil {
			t.Fatal(err)
		}
		tok := issueTok(cfg, author)
		topicID := createTopic(r, tok, "自答帖")
		replyID := createReply(r, tok, topicID, "我自己回答")
		before := getBal(r, tok)
		rec := doReq(r, tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
		// ADR-0028：自采纳直接拒绝（400），不再「静默零分发」
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("self accept should be 400, got %d %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "不能采纳自己的回答") {
			t.Fatalf("self accept message mismatch: %s", rec.Body.String())
		}
		if after := getBal(r, tok); after != before {
			t.Fatalf("self accept should not change balance, before %d after %d", before, after)
		}
		var cnt int64
		db.Model(&model.PointsLedger{}).Where("ref_type = ? AND ref_id = ?", "forum_topic", fmt.Sprintf("%d", topicID)).Count(&cnt)
		if cnt != 0 {
			t.Fatalf("self accept should create 0 ledger, got %d", cnt)
		}
		// 状态未被采纳
		var topic model.ForumTopic
		if err := db.First(&topic, topicID).Error; err != nil {
			t.Fatal(err)
		}
		if topic.AcceptedReplyID != nil {
			t.Fatalf("rejected self accept should not mark solved")
		}
	})
	t.Run("answerer_daily_3", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		cfg := &config.Config{JWTSecretKey: "caps-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
		r := gin.New()
		deps := newContractDeps(t, db, cfg)
		RegisterForumRoutes(r.Group("/api"), deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
		RegisterPointsRoutes(r.Group("/api"), deps.RouterDeps(), deps.PointsSvc)
		ans := model.HrwaiUser{Account: "caps_ans_daily", Phone: "13800001002", Username: "答主日封", Status: 1, CreatedAt: testutil.Now()}
		db.Create(&ans)
		ansTok := issueTok(cfg, ans)
		askers := make([]model.HrwaiUser, 4)
		askerToks := make([]string, 4)
		for i := 0; i < 4; i++ {
			u := model.HrwaiUser{Account: fmt.Sprintf("caps_asker_%d_%d", i, time.Now().UnixNano()), Phone: fmt.Sprintf("1380000200%d", i), Username: fmt.Sprintf("asker%d", i), Status: 1, CreatedAt: testutil.Now()}
			db.Create(&u)
			askers[i] = u
			askerToks[i] = issueTok(cfg, u)
		}
		for i := 0; i < 4; i++ {
			topicID := createTopic(r, askerToks[i], fmt.Sprintf("答主日封帖%d", i))
			replyID := createReply(r, ansTok, topicID, fmt.Sprintf("回答%d", i))
			rec := doReq(r, askerToks[i], http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
			if rec.Code != http.StatusOK {
				t.Fatalf("accept %d %d %s", i, rec.Code, rec.Body.String())
			}
			var got acceptResp
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Data.AcceptedReplyID == nil || *got.Data.AcceptedReplyID != replyID {
				t.Fatalf("topic %d should be accepted", topicID)
			}
		}
		bal := getBal(r, ansTok)
		if bal != 120 {
			t.Fatalf("answerer daily 3: expected 120, got %d", bal)
		}
	})
	t.Run("asker_daily_5", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		cfg := &config.Config{JWTSecretKey: "caps-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
		r := gin.New()
		deps := newContractDeps(t, db, cfg)
		RegisterForumRoutes(r.Group("/api"), deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
		RegisterPointsRoutes(r.Group("/api"), deps.RouterDeps(), deps.PointsSvc)
		asker := model.HrwaiUser{Account: "caps_asker_daily", Phone: "13800001003", Username: "楼主日封", Status: 1, CreatedAt: testutil.Now()}
		db.Create(&asker)
		askerTok := issueTok(cfg, asker)
		for i := 0; i < 6; i++ {
			ans := model.HrwaiUser{Account: fmt.Sprintf("caps_ans_asker_%d_%d", i, time.Now().UnixNano()), Phone: fmt.Sprintf("1380000300%d", i), Username: fmt.Sprintf("ans%d", i), Status: 1, CreatedAt: testutil.Now()}
			db.Create(&ans)
			ansTok := issueTok(cfg, ans)
			topicID := createTopic(r, askerTok, fmt.Sprintf("楼主日封帖%d", i))
			replyID := createReply(r, ansTok, topicID, fmt.Sprintf("回答%d", i))
			rec := doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
			if rec.Code != http.StatusOK {
				t.Fatalf("accept %d %d %s", i, rec.Code, rec.Body.String())
			}
		}
		bal := getBal(r, askerTok)
		if bal != 25 {
			t.Fatalf("asker daily 5: expected 25, got %d", bal)
		}
	})
	t.Run("pair_decay_6", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		cfg := &config.Config{JWTSecretKey: "caps-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
		r := gin.New()
		deps := newContractDeps(t, db, cfg)
		RegisterForumRoutes(r.Group("/api"), deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
		RegisterPointsRoutes(r.Group("/api"), deps.RouterDeps(), deps.PointsSvc)
		asker := model.HrwaiUser{Account: "caps_pair_asker", Phone: "13800001004", Username: "配对楼主", Status: 1, CreatedAt: testutil.Now()}
		db.Create(&asker)
		ans := model.HrwaiUser{Account: "caps_pair_ans", Phone: "13800001005", Username: "配对答主", Status: 1, CreatedAt: testutil.Now()}
		db.Create(&ans)
		askerTok := issueTok(cfg, asker)
		ansTok := issueTok(cfg, ans)
		for i := 0; i < 3; i++ {
			topicID := createTopic(r, askerTok, fmt.Sprintf("配对帖%d", i))
			replyID := createReply(r, ansTok, topicID, fmt.Sprintf("配对回答%d", i))
			rec := doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
			if rec.Code != http.StatusOK {
				t.Fatalf("pair accept %d %d %s", i, rec.Code, rec.Body.String())
			}
		}
		if bal := getBal(r, ansTok); bal != 120 {
			t.Fatalf("pair after 3 expected 120, got %d", bal)
		}
		yesterday := time.Now().Add(-48 * time.Hour)
		db.Model(&model.PointsLedger{}).Where("user_id = ?", ans.ID).Update("created_at", yesterday)
		db.Model(&model.PointsLedger{}).Where("user_id = ?", asker.ID).Update("created_at", yesterday)
		topic4 := createTopic(r, askerTok, "配对帖3")
		reply4 := createReply(r, ansTok, topic4, "配对回答3")
		before4 := getBal(r, ansTok)
		rec := doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topic4), map[string]any{"reply_id": reply4})
		if rec.Code != http.StatusOK {
			t.Fatalf("pair 4th %d %s", rec.Code, rec.Body.String())
		}
		after4 := getBal(r, ansTok)
		if after4-before4 != 20 {
			t.Fatalf("pair 4th expected +20, got %d (before %d after %d)", after4-before4, before4, after4)
		}
		topic5 := createTopic(r, askerTok, "配对帖4")
		reply5 := createReply(r, ansTok, topic5, "配对回答4")
		before5 := getBal(r, ansTok)
		rec = doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topic5), map[string]any{"reply_id": reply5})
		if rec.Code != http.StatusOK {
			t.Fatalf("pair 5th %d %s", rec.Code, rec.Body.String())
		}
		after5 := getBal(r, ansTok)
		if after5-before5 != 20 {
			t.Fatalf("pair 5th expected +20, got %d", after5-before5)
		}
		topic6 := createTopic(r, askerTok, "配对帖5")
		reply6 := createReply(r, ansTok, topic6, "配对回答5")
		before6 := getBal(r, ansTok)
		rec = doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topic6), map[string]any{"reply_id": reply6})
		if rec.Code != http.StatusOK {
			t.Fatalf("pair 6th %d %s", rec.Code, rec.Body.String())
		}
		after6 := getBal(r, ansTok)
		if after6-before6 != 0 {
			t.Fatalf("pair 6th expected +0, got %d", after6-before6)
		}
		var got acceptResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.AcceptedReplyID == nil || *got.Data.AcceptedReplyID != reply6 {
			t.Fatalf("pair 6th status should be accepted")
		}
		finalBal := getBal(r, ansTok)
		if finalBal != 160 {
			t.Fatalf("pair final expected 160, got %d", finalBal)
		}
		newAns := model.HrwaiUser{Account: "caps_pair_newans", Phone: "13800001006", Username: "新答主", Status: 1, CreatedAt: testutil.Now()}
		db.Create(&newAns)
		newAnsTok := issueTok(cfg, newAns)
		delTopic2 := createTopic(r, askerTok, "待删新帖")
		delReply2 := createReply(r, newAnsTok, delTopic2, "新答")
		rec = doReq(r, askerTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", delTopic2), map[string]any{"reply_id": delReply2})
		if rec.Code != http.StatusOK {
			t.Fatalf("del accept %d %s", rec.Code, rec.Body.String())
		}
		balBeforeDel := getBal(r, newAnsTok)
		rec = doReq(r, askerTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d", delTopic2), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete topic %d %s", rec.Code, rec.Body.String())
		}
		balAfterDel := getBal(r, newAnsTok)
		if balAfterDel != balBeforeDel {
			t.Fatalf("delete solved post should not rollback, before %d after %d", balBeforeDel, balAfterDel)
		}
	})
}
