package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// memForumStorage 论坛测试用内存存储：记录 Delete 调用与 List 返回。
type memForumStorage struct {
	deleted []string
	files   []string // List 返回的文件 URL
}

func (m *memForumStorage) Save(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "https://fake-cdn/x.webp", nil
}

func (m *memForumStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *memForumStorage) Exists(context.Context, string) (bool, error) { return true, nil }

func (m *memForumStorage) List(_ context.Context, _ string) ([]string, error) { return m.files, nil }

func (m *memForumStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte(url))), nil
}

// newForumTestSvc 构造论坛服务 + 内存存储（记录删除调用）。
func newForumTestSvc(t *testing.T) (*ForumService, *gorm.DB, *memForumStorage) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	st := &memForumStorage{}
	fileSvc := NewFileStore("", st, zap.NewNop())
	svc := NewForumService(db, fileSvc, NewNotificationService(db, zap.NewNop()), NewForumCounter(), zap.NewNop())
	return svc, db, st
}

func seedForumUser(t *testing.T, db *gorm.DB, name string) *model.HrwaiUser {
	t.Helper()
	u := &model.HrwaiUser{
		UID:       900000000000000000 + int64(len(name)),
		Account:   "forum_" + name,
		Username:  name,
		Password:  "x",
		Phone:     "f_" + name,
		Status:    1,
		CreatedAt: time.Now(),
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}
	return u
}

func forumURLs(n int) []string {
	urls := make([]string, 0, n)
	for i := 0; i < n; i++ {
		urls = append(urls, "/static/uploads/images/forum/img_1000"+string(rune('0'+i))+".webp")
	}
	return urls
}

// TestForum_CreateTopic_WithImages 发帖带图片：DTO 回显 images，DB 持久化 JSONB。
func TestForum_CreateTopic_WithImages(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "a")
	images := forumURLs(2)

	topic, err := svc.CreateTopic(user.ID, nil, "测试标题", "测试内容", images)
	if err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if len(topic.Images) != 2 {
		t.Fatalf("DTO 应回显 2 张图，得到 %d", len(topic.Images))
	}
	var stored model.ForumTopic
	if err := db.First(&stored, topic.ID).Error; err != nil {
		t.Fatal(err)
	}
	got := parseImageURLs(string(stored.Images))
	if len(got) != 2 || got[0] != images[0] {
		t.Fatalf("DB 持久化图片不符: %v", got)
	}
}

// TestForum_CreateTopic_ImageLimit 发帖图片超过 9 张应拒绝。
func TestForum_CreateTopic_ImageLimit(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "b")
	if _, err := svc.CreateTopic(user.ID, nil, "标题", "内容", forumURLs(10)); err == nil {
		t.Fatal("主题图片超过 9 张应报错")
	}
}

// TestForum_CreateTopic_RejectExternalImageURL 发帖图片为外部 URL 应拒绝。
func TestForum_CreateTopic_RejectExternalImageURL(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "c")
	if _, err := svc.CreateTopic(user.ID, nil, "标题", "内容", []string{"https://evil.com/x.png"}); err == nil {
		t.Fatal("外部图片 URL 应被拒绝")
	}
}

// TestForum_Reply_ImageLimit 回复图片超过 3 张应拒绝。
func TestForum_Reply_ImageLimit(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "d")
	topic, err := svc.CreateTopic(user.ID, nil, "标题", "内容", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReplyTopic(user.ID, topic.ID, "回复", nil, forumURLs(4)); err == nil {
		t.Fatal("回复图片超过 3 张应报错")
	}
}

// TestForum_DeleteTopic_CleansImages 删除主题时主题 + 回复（含子回复）图片全部清理。
func TestForum_DeleteTopic_CleansImages(t *testing.T) {
	svc, db, st := newForumTestSvc(t)
	user := seedForumUser(t, db, "e")
	topic, err := svc.CreateTopic(user.ID, nil, "标题", "内容", forumURLs(1))
	if err != nil {
		t.Fatal(err)
	}
	topicImages := forumURLs(1)
	reply, err := svc.ReplyTopic(user.ID, topic.ID, "回复", nil, forumURLs(1))
	if err != nil {
		t.Fatal(err)
	}
	replyImages := forumURLs(1)
	childImages := forumURLs(1)
	if _, err := svc.ReplyTopic(user.ID, topic.ID, "子回复", &reply.ID, childImages); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteTopic(user.ID, topic.ID); err != nil {
		t.Fatalf("删除主题失败: %v", err)
	}
	all := append(append(append([]string{}, topicImages...), replyImages...), childImages...)
	if len(st.deleted) != len(all) {
		t.Fatalf("应清理 %d 张图，实际清理 %d: %v", len(all), len(st.deleted), st.deleted)
	}
	for _, u := range all {
		if !containsForumURL(st.deleted, u) {
			t.Errorf("缺少清理 %q，已清理: %v", u, st.deleted)
		}
	}
}

// TestForum_DeleteReply_CleansSubReplyImages 删除回复时其下级回复图片一并清理。
func TestForum_DeleteReply_CleansSubReplyImages(t *testing.T) {
	svc, db, st := newForumTestSvc(t)
	user := seedForumUser(t, db, "f")
	topic, err := svc.CreateTopic(user.ID, nil, "标题", "内容", nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := svc.ReplyTopic(user.ID, topic.ID, "父回复", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	childImages := forumURLs(1)
	if _, err := svc.ReplyTopic(user.ID, topic.ID, "子回复", &parent.ID, childImages); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteReply(user.ID, parent.ID); err != nil {
		t.Fatalf("删除回复失败: %v", err)
	}
	if len(st.deleted) != len(childImages) {
		t.Fatalf("应清理子回复 %d 张图，实际清理 %d: %v", len(childImages), len(st.deleted), st.deleted)
	}
}

// containsStr 判断字符串切片是否包含目标。
func containsForumURL(list []string, target string) bool {
	for _, s := range list {
		if strings.TrimSpace(s) == target {
			return true
		}
	}
	return false
}
