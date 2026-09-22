// 契约测（ADR-0062 票12）：数组类契约出口归一——无图即 `"images": []`，不得出 `"images": null`。
// 生成的契约（frontend/src/api/generated/forum.ts 的 `images: string[]`）承诺数组，
// 旧形状靠各页面手写 `|| []` / `&&` 存活；新消费者写 `topic.images.length` 类型全绿、运行时 TypeError。
package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestForumImagesArrayNeverNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "images-wire-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	pwd, err := service.HashPassword("student123")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	student := testutil.SeedStudent(t, db, "images_wire_stu", pwd)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(student.ID, student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	topic := model.ForumTopic{Category: "discussion", UserID: student.ID,
		Title: "无图主题", Content: "正文", CreatedAt: testutil.Now()}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建主题失败: %v", err)
	}
	reply := model.ForumReply{TopicID: topic.ID, UserID: student.ID, Content: "无图回复", CreatedAt: testutil.Now()}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("建回复失败: %v", err)
	}

	cases := []string{
		"/api/forum/topics?page=1&page_size=20",       // 列表（DTO 出口）
		fmt.Sprintf("/api/forum/topics/%d", topic.ID), // 详情（主题 + 回复两处出口）
	}
	for _, url := range cases {
		rec := doWithToken(t, r, token, http.MethodGet, url, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s 应 200, got %d %s", url, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, `"images":null`) {
			t.Fatalf("%s 出 null 数组，与生成的契约 string[] 矛盾: %s", url, body)
		}
		if !strings.Contains(body, `"images":[]`) {
			t.Fatalf("%s 无图时应回空数组（出口归一，不是省略也不是 null）: %s", url, body)
		}
	}
}
