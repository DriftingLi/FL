// ADR-0018 契约测试：论坛互动——点赞 / 举报 / 我的帖子 / 我的回复。
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

func TestForumInteractionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "acct_a", Phone: "13800000001", Username: "作者甲", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	replier := model.HrwaiUser{Account: "acct_b", Phone: "13800000002", Username: "回帖乙", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&replier).Error; err != nil {
		t.Fatalf("创建回帖人失败: %v", err)
	}
	now := testutil.Now()
	topic := model.ForumTopic{UserID: int(author.ID), Title: "液压系统求助", Content: "叉车液压油温偏高怎么办", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	reply := model.ForumReply{TopicID: topic.ID, UserID: int(replier.ID), Content: "检查散热器", CreatedAt: now}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("创建回复失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(api, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

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

	do := func(token, method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. 点赞（幂等：重复点赞计数不重复）。
	rec := do(token, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/like", topic.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("点赞期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var like struct {
		Data struct {
			LikesCount int64 `json:"likes_count"`
			Liked      bool  `json:"liked"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &like); err != nil {
		t.Fatalf("解析点赞响应失败: %v", err)
	}
	if like.Data.LikesCount != 1 || !like.Data.Liked {
		t.Fatalf("点赞响应错误: %+v", like.Data)
	}
	rec = do(token, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/like", topic.ID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &like); err != nil {
		t.Fatalf("解析重复点赞响应失败: %v", err)
	}
	if like.Data.LikesCount != 1 {
		t.Fatalf("重复点赞应幂等计数 1, got %d", like.Data.LikesCount)
	}

	// 2. 详情带点赞状态。
	rec = do(token, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", topic.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("详情期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Data struct {
			Topic struct {
				LikesCount int64 `json:"likes_count"`
				LikedByMe  bool  `json:"liked_by_me"`
			} `json:"topic"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	if detail.Data.Topic.LikesCount != 1 || !detail.Data.Topic.LikedByMe {
		t.Fatalf("详情点赞状态错误: %+v", detail.Data.Topic)
	}

	// 3. 取消点赞。
	rec = do(token, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/like", topic.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消点赞期望 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. 举报主题 / 回复；重复无效目标 400。
	rec = do(token, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/report", topic.ID), map[string]string{"reason": "垃圾广告"})
	if rec.Code != http.StatusOK {
		t.Fatalf("举报主题期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(token, http.MethodPost, "/api/forum/topics/99999/report", map[string]string{"reason": "不存在"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("举报不存在主题期望 400, got %d", rec.Code)
	}
	rec = do(token, http.MethodPost, fmt.Sprintf("/api/forum/replies/%d/report", reply.ID), map[string]string{"reason": "人身攻击"})
	if rec.Code != http.StatusOK {
		t.Fatalf("举报回复期望 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. 我的帖子（作者视角）。
	rec = do(token, http.MethodGet, "/api/forum/my-topics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("我的帖子期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var mine struct {
		Data struct {
			Total  int64 `json:"total"`
			Topics []struct {
				Title string `json:"title"`
			} `json:"topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mine); err != nil {
		t.Fatalf("解析我的帖子失败: %v", err)
	}
	if mine.Data.Total != 1 || len(mine.Data.Topics) != 1 || mine.Data.Topics[0].Title != "液压系统求助" {
		t.Fatalf("我的帖子错误: %+v", mine.Data)
	}

	// 6. 我的回复（回帖人视角，带主题标题回填）。
	replyToken, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(replier.ID), replier.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发回帖人 token 失败: %v", err)
	}
	rec = do(replyToken, http.MethodGet, "/api/forum/my-replies", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("我的回复期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var myReplies struct {
		Data struct {
			Total   int64 `json:"total"`
			Replies []struct {
				TopicID    int64  `json:"topic_id"`
				TopicTitle string `json:"topic_title"`
				Content    string `json:"content"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &myReplies); err != nil {
		t.Fatalf("解析我的回复失败: %v", err)
	}
	if myReplies.Data.Total != 1 || len(myReplies.Data.Replies) != 1 {
		t.Fatalf("我的回复数量错误: %+v", myReplies.Data)
	}
	if myReplies.Data.Replies[0].TopicID != topic.ID || myReplies.Data.Replies[0].TopicTitle != "液压系统求助" {
		t.Fatalf("我的回复回填错误: %+v", myReplies.Data.Replies[0])
	}

	// 7. 管理端：举报列表（待处理 2 条）→ 处理一条 → 待处理 1 条。
	rec = do(adminToken, http.MethodGet, "/api/admin/forum/reports?status=0", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("举报列表期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var reports struct {
		Data struct {
			Total   int64 `json:"total"`
			Reports []struct {
				ID         int64  `json:"id"`
				Reason     string `json:"reason"`
				Reporter   string `json:"reporter"`
				TopicTitle string `json:"topic_title"`
				Status     int16  `json:"status"`
			} `json:"reports"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reports); err != nil {
		t.Fatalf("解析举报列表失败: %v", err)
	}
	if reports.Data.Total != 2 {
		t.Fatalf("待处理举报应为 2, got %d", reports.Data.Total)
	}
	var topicReportID int64
	for _, rp := range reports.Data.Reports {
		if rp.TopicTitle == "液压系统求助" && rp.Reporter == "作者甲" {
			topicReportID = rp.ID
		}
	}
	if topicReportID == 0 {
		t.Fatalf("未找到主题举报（回填错误）: %+v", reports.Data.Reports)
	}
	rec = do(adminToken, http.MethodPut, fmt.Sprintf("/api/admin/forum/reports/%d", topicReportID), map[string]int16{"status": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("处理举报期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(adminToken, http.MethodGet, "/api/admin/forum/reports?status=0", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &reports); err != nil {
		t.Fatalf("解析举报列表失败: %v", err)
	}
	if reports.Data.Total != 1 {
		t.Fatalf("处理后待处理应为 1, got %d", reports.Data.Total)
	}
}
