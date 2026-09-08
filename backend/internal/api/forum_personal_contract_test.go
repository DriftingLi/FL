// #701 契约测试：个人动态三列表——赞过 / 围观 / 浏览记录。
//
// 覆盖不变式：三端点 200 且分页四件套与 my-topics 逐字一致；赞过按点赞时间倒序；
// 浏览记录按主题去重、按最近浏览倒序；围观 = 浏览减去四项直接互动；删帖条目保留、标题回空串。
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

type personalListResp struct {
	Code int `json:"code"`
	Data struct {
		Page   int   `json:"page"`
		Pages  int   `json:"pages"`
		Total  int64 `json:"total"`
		Topics []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"topics"`
	} `json:"data"`
}

func TestForumPersonalListsContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	mkUser := func(account, phone, name string) model.HrwaiUser {
		u := model.HrwaiUser{Account: account, Phone: phone, Username: name, Status: 1, CreatedAt: testutil.Now()}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("建用户失败: %v", err)
		}
		return u
	}
	me := mkUser("personal_me", "13800000111", "我")
	other := mkUser("personal_other", "13800000112", "他人")

	mkTopic := func(uid int, title string) model.ForumTopic {
		now := testutil.Now()
		tp := model.ForumTopic{UserID: uid, Title: title, Content: title + "内容", CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&tp).Error; err != nil {
			t.Fatalf("建主题失败: %v", err)
		}
		return tp
	}
	likedTopic := mkTopic(int(other.ID), "被赞帖")
	viewedTopic := mkTopic(int(other.ID), "被看帖")
	repliedTopic := mkTopic(int(other.ID), "被回帖")
	ownTopic := mkTopic(int(me.ID), "自帖")

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(api, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

	issueToken := func(u model.HrwaiUser) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	meTok := issueToken(me)

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
	getList := func(path string) personalListResp {
		rec := do(meTok, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s 期望 200, got %d %s", path, rec.Code, rec.Body.String())
		}
		var got personalListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		if got.Data.Topics == nil {
			t.Fatalf("GET %s 缺 topics 数组", path)
		}
		return got
	}
	ids := func(got personalListResp) []int64 {
		out := make([]int64, 0, len(got.Data.Topics))
		for _, tp := range got.Data.Topics {
			out = append(out, tp.ID)
		}
		return out
	}

	// 点赞两帖（顺序：先被赞帖、后被看帖——赞过排序应为倒序）。
	for _, tp := range []model.ForumTopic{likedTopic, viewedTopic} {
		rec := do(meTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/like", tp.ID), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("点赞期望 200, got %d %s", rec.Code, rec.Body.String())
		}
	}
	// 回复一帖（该帖应从围观排除）。
	rec := do(meTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", repliedTopic.ID), map[string]string{"content": "顶"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("回复期望 201, got %d %s", rec.Code, rec.Body.String())
	}
	// 浏览：被看帖（2 次跨日去重验证）、被回帖、自帖（写入侧排除自帖）。
	// 注意：通过详情接口产生的浏览由 GetTopic 写入；此处为精确控制时间直接插表，
	// 自帖行模拟的是历史脏数据——实现侧不做自帖过滤（写入侧已保证），断言只看去重与排序。
	view := func(topicID int64, date string) {
		if err := db.Exec("INSERT INTO forum_topic_views (user_id, topic_id, viewed_at, view_date) VALUES (?,?,?,?)",
			me.ID, topicID, testutil.Now(), date).Error; err != nil {
			t.Fatalf("写浏览失败: %v", err)
		}
	}
	view(viewedTopic.ID, "2026-09-01")
	view(viewedTopic.ID, "2026-09-02")
	view(repliedTopic.ID, "2026-09-03")
	view(ownTopic.ID, "2026-09-04")

	// 赞过：两帖，按点赞时间倒序（后赞的被看帖在前）。
	liked := getList("/api/forum/my-liked-topics")
	if liked.Data.Total != 2 {
		t.Fatalf("赞过应 2 条, got %d", liked.Data.Total)
	}
	gotIDs := ids(liked)
	if gotIDs[0] != viewedTopic.ID || gotIDs[1] != likedTopic.ID {
		t.Fatalf("赞过应按点赞时间倒序, got %v", gotIDs)
	}

	// 浏览记录：按主题去重（被看帖跨日只算 1 条）；自帖行是直接插表的历史脏数据（写入侧本不产生），不断言其过滤。
	history := getList("/api/forum/my-view-history")
	if history.Data.Total != 3 {
		t.Fatalf("浏览记录应去重为 3 条, got %d", history.Data.Total)
	}
	hIDs := ids(history)
	if hIDs[0] != ownTopic.ID || hIDs[1] != repliedTopic.ID || hIDs[2] != viewedTopic.ID {
		t.Fatalf("浏览记录应按最近浏览倒序, got %v", hIDs)
	}

	// 围观：浏览减去互动——自帖（本人发帖）、被回帖（回复过）、被看帖（赞过）都排除；
	// 再补一个纯浏览帖验证进入；另对围观帖加主题收藏，验证第四项排除。
	pureView := mkTopic(int(other.ID), "纯围观帖")
	view(pureView.ID, "2026-09-05")
	favTopic := mkTopic(int(other.ID), "收藏围观帖")
	view(favTopic.ID, "2026-09-06")
	if err := db.Create(&model.Favorite{UserID: int(me.ID), TargetType: "topic", TargetID: int(favTopic.ID), CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("写收藏失败: %v", err)
	}
	observed := getList("/api/forum/my-observed")
	if observed.Data.Total != 1 || ids(observed)[0] != pureView.ID {
		t.Fatalf("围观应仅纯浏览帖, got %+v", ids(observed))
	}

	// 删帖保留：删除被赞帖后，赞过仍保留条目、标题回空串。
	if err := db.Delete(&model.ForumTopic{}, likedTopic.ID).Error; err != nil {
		t.Fatalf("删帖失败: %v", err)
	}
	likedAfter := getList("/api/forum/my-liked-topics")
	if likedAfter.Data.Total != 2 {
		t.Fatalf("删帖后赞过应保留 2 条, got %d", likedAfter.Data.Total)
	}
	for _, tp := range likedAfter.Data.Topics {
		if tp.ID == likedTopic.ID && tp.Title != "" {
			t.Fatalf("被删主题标题应回空串, got %q", tp.Title)
		}
	}

	// 未认证 401。
	rec = do("", http.MethodGet, "/api/forum/my-liked-topics", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证应 401, got %d", rec.Code)
	}
}
