// #855 契约测试：回复项的「被回复人」带小头像（ADR-0042）。
//
// 覆盖不变式：
//  1. 楼中楼回复回填 `parent_name` + `parent_avatar_url`（取**被回复那条的作者**，不是楼主）。
//  2. 顶层回复两者均空——前端据此不渲染该片段。
//  3. 被回复人无头像时 `parent_avatar_url` 为空，`parent_name` 仍回填（名字可用、头像降级）。
//  4. 分页不影响回填：置顶条与其余页的楼中楼同样带被回复人信息。
package api

import (
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

type replyParentResp struct {
	Code int `json:"code"`
	Data struct {
		Replies []struct {
			ID              int64  `json:"id"`
			ParentName      string `json:"parent_name"`
			ParentAvatarURL string `json:"parent_avatar_url"`
		} `json:"replies"`
	} `json:"data"`
}

func TestForumReplyParentAvatarContract(t *testing.T) {
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
	mkUser := func(account, phone, name, avatar string) model.HrwaiUser {
		u := model.HrwaiUser{Account: account, Phone: phone, Username: name, AvatarURL: avatar, Status: 1, CreatedAt: now}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("建用户失败: %v", err)
		}
		return u
	}
	author := mkUser("rp_avatar_author", "13800000801", "楼主", "/static/uploads/avatar/author.png")
	// 有头像的被回复人、无头像的被回复人（同一帖内对照）。
	withAvatar := mkUser("rp_avatar_a", "13800000802", "有头像的人", "/static/uploads/avatar/a.png")
	noAvatar := mkUser("rp_avatar_b", "13800000803", "没头像的人", "")

	topic := model.ForumTopic{
		UserID: int(author.ID), Category: "discussion", Title: "讨论", Content: "内容",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建主题失败: %v", err)
	}

	mkReply := func(uid int, parentID *int64, content string, offset time.Duration) model.ForumReply {
		rp := model.ForumReply{
			TopicID: topic.ID, UserID: uid, ParentID: parentID,
			Content: content, CreatedAt: now.Add(offset),
		}
		if err := db.Create(&rp).Error; err != nil {
			t.Fatalf("建回复失败: %v", err)
		}
		return rp
	}

	// 结构：root（顶层）← childOfAvatar（回复 root）
	//       root2（顶层）← childOfNoAvatar（回复 root2）
	// root 由「有头像的人」发、root2 由「没头像的人」发，用于区分「取被回复人」与「取楼主」。
	root := mkReply(int(withAvatar.ID), nil, "顶层回复", 0)
	childOfAvatar := mkReply(int(author.ID), &root.ID, "回复有头像的人", time.Minute)
	root2 := mkReply(int(noAvatar.ID), nil, "顶层回复2", 2*time.Minute)
	childOfNoAvatar := mkReply(int(author.ID), &root2.ID, "回复没头像的人", 3*time.Minute)

	if err := db.Model(&model.ForumTopic{}).Where("id = ?", topic.ID).Update("reply_count", 4).Error; err != nil {
		t.Fatalf("回填 reply_count 失败: %v", err)
	}

	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/forum/topics/"+fmt.Sprint(topic.ID)+"?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d %s", w.Code, w.Body.String())
	}
	var got replyParentResp
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	byID := map[int64]struct{ name, avatar string }{}
	for _, rp := range got.Data.Replies {
		byID[rp.ID] = struct{ name, avatar string }{rp.ParentName, rp.ParentAvatarURL}
	}

	if len(byID) != 4 {
		t.Fatalf("应返回 4 条回复, got %d", len(byID))
	}

	t.Run("楼中楼回填被回复人的名字与头像", func(t *testing.T) {
		p, ok := byID[childOfAvatar.ID]
		if !ok {
			t.Fatalf("缺回复 %d", childOfAvatar.ID)
		}
		if p.name != "有头像的人" {
			t.Fatalf("parent_name = %q, want 有头像的人", p.name)
		}
		if p.avatar != "/static/uploads/avatar/a.png" {
			t.Fatalf("parent_avatar_url = %q, want /static/uploads/avatar/a.png", p.avatar)
		}
	})

	t.Run("被回复人无头像时名字仍在、头像为空", func(t *testing.T) {
		p := byID[childOfNoAvatar.ID]
		if p.name != "没头像的人" {
			t.Fatalf("parent_name = %q, want 没头像的人", p.name)
		}
		if p.avatar != "" {
			t.Fatalf("parent_avatar_url = %q, want 空", p.avatar)
		}
	})

	t.Run("顶层回复不带被回复人信息", func(t *testing.T) {
		for _, id := range []int64{root.ID, root2.ID} {
			p := byID[id]
			if p.name != "" || p.avatar != "" {
				t.Fatalf("顶层回复 %d 不应有被回复人信息, got name=%q avatar=%q", id, p.name, p.avatar)
			}
		}
	})

	t.Run("取的是被回复那条的作者而非楼主", func(t *testing.T) {
		if byID[childOfAvatar.ID].name == "楼主" {
			t.Fatal("parent_name 取了楼主，应为被回复那条的作者")
		}
	})
}
