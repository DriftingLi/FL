// spec #297 契约测试：论坛点赞计数在账号注销后同事务回扣——
// POST /api/forum/topics/:id/like（+ replies/:id/like）→ DELETE /api/auth/account → 计数归位。
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestForumLikeRefundOnAccountDeletionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "acct_ref_a", Phone: "13800000011", Username: "楼主", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	liker := model.HrwaiUser{Account: "acct_ref_b", Phone: "13800000012", Username: "点赞人", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&liker).Error; err != nil {
		t.Fatalf("创建点赞人失败: %v", err)
	}

	now := time.Now()
	topic := model.ForumTopic{UserID: int(author.ID), Title: "注销回扣主题", Content: "内容", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	reply := model.ForumReply{TopicID: topic.ID, UserID: int(author.ID), Content: "被赞回复", CreatedAt: now}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("创建回复失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(liker.ID), liker.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	do := func(method, path string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 点赞主题与回复（计数经 forumCounter 同步）。
	if rec := do(http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/like", topic.ID)); rec.Code != http.StatusOK {
		t.Fatalf("点赞主题期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/api/forum/replies/%d/like", reply.ID)); rec.Code != http.StatusOK {
		t.Fatalf("点赞回复期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertColumnInt64(t, db, "forum_topics", topic.ID, "likes_count", 1)
	assertColumnInt64(t, db, "forum_replies", reply.ID, "likes_count", 1)

	// 注销：DELETE /api/auth/account 后点赞行删除、计数回扣。
	if rec := do(http.MethodDelete, "/api/auth/account"); rec.Code != http.StatusOK {
		t.Fatalf("注销期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertColumnInt64(t, db, "forum_topics", topic.ID, "likes_count", 0)
	assertColumnInt64(t, db, "forum_replies", reply.ID, "likes_count", 0)

	var likeRows int64
	db.Table("forum_topic_like").Where("user_id = ?", liker.ID).Count(&likeRows)
	db.Table("forum_reply_like").Where("user_id = ?", liker.ID).Count(&likeRows)
	if likeRows != 0 {
		t.Fatalf("注销后点赞行应清零, got %d", likeRows)
	}
	var users int64
	db.Table("hrwai_users").Where("id = ?", liker.ID).Count(&users)
	if users != 0 {
		t.Fatal("注销后账号应已硬删除")
	}
}

// assertColumnInt64 断言表内某行某整型列的值。
func assertColumnInt64(t *testing.T, db *gorm.DB, table string, id int64, col string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Select(col).Where("id = ?", id).Scan(&got).Error; err != nil {
		t.Fatalf("读取 %s.%s 失败: %v", table, col, err)
	}
	if got != want {
		t.Fatalf("%s.%s = %d, want %d", table, col, got, want)
	}
}
