package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// forumImgStorage 论坛图片测试用内存存储：记录 Save/Delete 调用，List 返回预设文件。
type forumImgStorage struct {
	savedKeys    []string
	savedContent [][]byte
	deleted      []string
	files        []string // List 返回的文件 URL
	saveErr      error
}

func (m *forumImgStorage) Save(_ context.Context, key string, content []byte, _ string) (string, error) {
	if m.saveErr != nil {
		return "", m.saveErr
	}
	m.savedKeys = append(m.savedKeys, key)
	m.savedContent = append(m.savedContent, content)
	return "/static/uploads/" + key, nil
}

func (m *forumImgStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *forumImgStorage) Exists(context.Context, string) (bool, error) { return true, nil }

func (m *forumImgStorage) List(_ context.Context, _ string) ([]string, error) { return m.files, nil }

// newForumImageTestSvc 构造论坛图片服务 + 内存存储。
func newForumImageTestSvc(t *testing.T) (*ForumImageService, *gorm.DB, *forumImgStorage) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	st := &forumImgStorage{}
	fileSvc := NewFileService("", st, zap.NewNop())
	svc := NewForumImageService(db, fileSvc, zap.NewNop())
	return svc, db, st
}

// multipartFileHeader 构造一个名为 filename、内容为 content 的 multipart 文件头。
func multipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, header, err := req.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	return header
}

// TestForumImage_Upload_Success 上传成功：保存到 images/forum/ 前缀，
// 文件名内嵌毫秒时间戳、保留扩展名，返回完整可访问 URL，内容原样写入。
func TestForumImage_Upload_Success(t *testing.T) {
	svc, _, st := newForumImageTestSvc(t)
	header := multipartFileHeader(t, "photo.png", []byte("png-bytes"))

	url, err := svc.Upload(context.Background(), header)
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if len(st.savedKeys) != 1 {
		t.Fatalf("应调用一次 Save，实际 %d", len(st.savedKeys))
	}
	key := st.savedKeys[0]
	if !strings.HasPrefix(key, ForumImageDirPrefix+"/") {
		t.Fatalf("应保存到 %s/ 前缀，得到 %q", ForumImageDirPrefix, key)
	}
	name := strings.TrimPrefix(key, ForumImageDirPrefix+"/")
	if ms, ok := svc.fileSvc.ExtractTimestamp(name); !ok || ms <= 0 {
		t.Fatalf("文件名应内嵌毫秒时间戳，得到 %q (ok=%v, ms=%d)", name, ok, ms)
	}
	if !strings.HasSuffix(name, ".png") {
		t.Fatalf("应保留原扩展名 .png，得到 %q", name)
	}
	if want := "/static/uploads/" + key; url != want {
		t.Fatalf("URL 应为 %q，得到 %q", want, url)
	}
	if string(st.savedContent[0]) != "png-bytes" {
		t.Fatalf("上传内容不符: %q", st.savedContent[0])
	}
}

// TestForumImage_Upload_EmptyFilename 文件名为空：返回 400 未选择文件，不触发 Save。
func TestForumImage_Upload_EmptyFilename(t *testing.T) {
	svc, _, st := newForumImageTestSvc(t)
	_, err := svc.Upload(context.Background(), &multipart.FileHeader{Filename: ""})
	var fe *ForumImageError
	if !errors.As(err, &fe) || fe.Status != http.StatusBadRequest || fe.Message != "未选择文件" {
		t.Fatalf("应返回 400 未选择文件，得到 %v", err)
	}
	if len(st.savedKeys) != 0 {
		t.Fatalf("不应触发 Save，实际 %d 次", len(st.savedKeys))
	}
}

// TestForumImage_Upload_InvalidType 不支持的格式：返回 400 与允许格式提示。
func TestForumImage_Upload_InvalidType(t *testing.T) {
	svc, _, _ := newForumImageTestSvc(t)
	header := multipartFileHeader(t, "doc.txt", []byte("x"))

	_, err := svc.Upload(context.Background(), header)
	var fe *ForumImageError
	if !errors.As(err, &fe) || fe.Status != http.StatusBadRequest {
		t.Fatalf("应返回 400 校验错误，得到 %v", err)
	}
	if !strings.Contains(fe.Message, "不支持的图片格式") {
		t.Fatalf("消息应提示格式不支持，得到 %q", fe.Message)
	}
}

// TestForumImage_Upload_Oversize 超过 20MB 大小限制：返回 400。
func TestForumImage_Upload_Oversize(t *testing.T) {
	svc, _, _ := newForumImageTestSvc(t)
	header := multipartFileHeader(t, "big.png", []byte("x"))
	header.Size = 20*1024*1024 + 1

	_, err := svc.Upload(context.Background(), header)
	var fe *ForumImageError
	if !errors.As(err, &fe) || fe.Status != http.StatusBadRequest {
		t.Fatalf("应返回 400 大小错误，得到 %v", err)
	}
	if !strings.Contains(fe.Message, "大小超出限制") {
		t.Fatalf("消息应提示大小超限，得到 %q", fe.Message)
	}
}

// TestForumImage_Upload_SaveError 保存失败：返回 500 且消息携带底层错误。
func TestForumImage_Upload_SaveError(t *testing.T) {
	svc, _, st := newForumImageTestSvc(t)
	st.saveErr = errors.New("磁盘满")
	header := multipartFileHeader(t, "photo.png", []byte("png-bytes"))

	_, err := svc.Upload(context.Background(), header)
	var fe *ForumImageError
	if !errors.As(err, &fe) || fe.Status != http.StatusInternalServerError {
		t.Fatalf("应返回 500 保存错误，得到 %v", err)
	}
	if !strings.Contains(fe.Message, "图片上传失败: 磁盘满") {
		t.Fatalf("消息应携带底层错误，得到 %q", fe.Message)
	}
}

// TestForumImage_CleanupOrphanImages 悬空图片清理：被引用/未超时保留，未引用且超时删除。
func TestForumImage_CleanupOrphanImages(t *testing.T) {
	svc, db, st := newForumImageTestSvc(t)
	user := seedForumUser(t, db, "g")

	// 被主题引用的图片（即使超时也应保留）
	referenced := "/static/uploads/images/forum/keep_1000000000000.webp"
	topic := &model.ForumTopic{
		UserID:    user.ID,
		Title:     "标题",
		Content:   "内容",
		Images:    marshalImageURLs([]string{referenced}),
		CreatedAt: testutil.Now(),
	}
	if err := db.Create(topic).Error; err != nil {
		t.Fatalf("插入引用图片的主题失败: %v", err)
	}
	// 悬空但未超时（时间戳 < 24h）
	fresh := "/static/uploads/images/forum/fresh_2000000000000.webp"
	// 悬空且超时
	stale := "/static/uploads/images/forum/stale_1000000000000.webp"
	st.files = []string{referenced, fresh, stale}

	cleaned := svc.CleanupOrphans(context.Background())
	if cleaned != 1 {
		t.Fatalf("应清理 1 张悬空超时图片，实际 %d", cleaned)
	}
	if len(st.deleted) != 1 || st.deleted[0] != stale {
		t.Fatalf("应删除 %q，实际删除: %v", stale, st.deleted)
	}
}

// TestForumImage_CleanupOrphanImages_Empty 存储为空列表：直接返回 0，不触发删除。
func TestForumImage_CleanupOrphanImages_Empty(t *testing.T) {
	svc, _, st := newForumImageTestSvc(t)
	st.files = nil

	cleaned := svc.CleanupOrphans(context.Background())
	if cleaned != 0 {
		t.Fatalf("空列表应清理 0 张，实际 %d", cleaned)
	}
	if len(st.deleted) != 0 {
		t.Fatalf("空列表不应删除任何文件，实际: %v", st.deleted)
	}
}
