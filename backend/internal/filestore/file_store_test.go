package filestore

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/storage"
)

// fileStoreMemStorage 内存 storage adapter：记录 Save/Delete/List 调用。
type fileStoreMemStorage struct {
	savedKeys []string
	savedURLs []string
	deleted   []string
	files     []string
}

func (m *fileStoreMemStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	m.savedKeys = append(m.savedKeys, key)
	url := "/static/uploads/" + key
	m.savedURLs = append(m.savedURLs, url)
	return url, nil
}

func (m *fileStoreMemStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *fileStoreMemStorage) Exists(context.Context, string) (bool, error) { return true, nil }
func (m *fileStoreMemStorage) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *fileStoreMemStorage) List(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), m.files...), nil
}

// ListWithInfo 测试用：LastModified 由文件 URL 命名契约推导（见 fileStampToTime）。
func (m *fileStoreMemStorage) ListWithInfo(_ context.Context, _ string) ([]storage.FileInfo, error) {
	infos := make([]storage.FileInfo, 0, len(m.files))
	for _, u := range m.files {
		infos = append(infos, storage.FileInfo{URL: u, LastModified: fileStampToTime(u)})
	}
	return infos, nil
}

// fileStampToTime 测试 helper：从 URL 的命名契约（<name>_<ms>.<ext>）解析毫秒时间戳当 LastModified；
// 解析不出（无内嵌时间戳）返回零值。生产代码的时间戳来自 storage 层真实元数据，不反解析文件名
// （ADR-0027 C2）；本 helper 只服务测试内存适配器（0c 前住在 internal/service 的测试里）。
func fileStampToTime(url string) time.Time {
	base := url
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot < 0 {
		return time.Time{}
	}
	underscore := strings.LastIndex(base[:dot], "_")
	if underscore < 0 {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(base[underscore+1:dot], 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func TestFileStoreInterface(t *testing.T) {
	st := &fileStoreMemStorage{}
	store := NewFileStore("", st, zap.NewNop())

	url, err := store.Save([]byte("png"), "a.png", "images/questions")
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if !strings.Contains(url, "images/questions/") {
		t.Fatalf("URL 未包含 key: %q", url)
	}
	if len(st.savedKeys) != 1 {
		t.Fatalf("Save 调用次数 = %d", len(st.savedKeys))
	}

	if err := store.Delete(url); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	store.DeleteFiles([]string{url, ""})
	if len(st.deleted) != 2 {
		t.Fatalf("Delete 调用次数 = %d, 期望 2（DeleteFiles 忽略空 URL）", len(st.deleted))
	}

	st.files = []string{url}
	got := store.List("images/questions")
	if len(got) != 1 || got[0] != url {
		t.Fatalf("List 结果 = %v, 期望 [%s]", got, url)
	}
}

func TestFileStoreValidateImage(t *testing.T) {
	store := NewFileStore("", &fileStoreMemStorage{}, zap.NewNop())
	if ok, _ := store.ValidateImage("a.png", 1024); !ok {
		t.Error("png 应通过校验")
	}
	if ok, _ := store.ValidateImage("a.exe", 1024); ok {
		t.Error("exe 不应通过图片校验")
	}
	if ok, _ := store.ValidateImage("a.png", 21*1024*1024); ok {
		t.Error("超过图片大小上限应拒绝")
	}
}

// TestFileExtension 扩展名解析口径表（无扩展名 / 多点 / 大写 / 空串）。
// 本用例原先住在 internal/service，跟随 fileExtension 一起搬进来（#1445 波 0c）。
func TestFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"test.pdf", "pdf"},
		{"archive.tar.gz", "gz"},
		{"noext", ""},
		{".gitignore", "gitignore"},
		{"path/to/file.PPTX", "pptx"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := FileExtension(tt.filename); got != tt.want {
			t.Errorf("FileExtension(%q) = %q，期望 %q", tt.filename, got, tt.want)
		}
	}
}

// TestUploadGateHelpers 上传闸门的三个导出谓词与分档上限（导师章节文件、招聘卡附件共用）。
func TestUploadGateHelpers(t *testing.T) {
	if !AllowedFile("a.pdf") {
		t.Error("pdf 应在白名单内")
	}
	if AllowedFile("a.exe") {
		t.Error("exe 不在类型表内，不应放行")
	}
	if AllowedFile("a.svg") {
		t.Error("svg 登记为 unsafe 且 upload 列为空，不应放行")
	}
	if got := FileContentType("a.pptx"); got != "ppt" {
		t.Errorf("FileContentType(pptx) = %q，期望 ppt", got)
	}
	if MaxFileSize("a.mp4") != 200*1024*1024 || MaxFileSize("a.png") != 20*1024*1024 || MaxFileSize("a.pdf") != 50*1024*1024 {
		t.Errorf("分档上限错：mp4=%d png=%d pdf=%d", MaxFileSize("a.mp4"), MaxFileSize("a.png"), MaxFileSize("a.pdf"))
	}
	if MaxFileSize("a.unknownext") != 50*1024*1024 {
		t.Errorf("未登记类别应回落 default 50MB，实得 %d", MaxFileSize("a.unknownext"))
	}
	if ValidateFileSize(21*1024*1024, "a.png") {
		t.Error("png 21MB 应超上限（image 档 20MB）")
	}
	if !ValidateFileSize(21*1024*1024, "a.pdf") {
		t.Error("pdf 21MB 应在 default 档内")
	}
}

// TestFileStoreExistsUnconfigured 适配器没装时 Exists 报错而不是 (false, nil)：
// 「没配存储」与「文件不存在」必须可分辨，前者在上层落 500（投稿暂存第四校验）。
func TestFileStoreExistsUnconfigured(t *testing.T) {
	bare := NewFileStore("", nil, zap.NewNop())
	if _, err := bare.Exists(context.Background(), "/static/uploads/x.pdf"); !errors.Is(err, ErrStorageUnconfigured) {
		t.Fatalf("未装适配器应回 ErrStorageUnconfigured，实得 %v", err)
	}
	ok, err := NewFileStore("", &fileStoreMemStorage{}, zap.NewNop()).Exists(context.Background(), "/static/uploads/x.pdf")
	if err != nil || !ok {
		t.Fatalf("装了适配器应回 (true, nil)，实得 (%v, %v)", ok, err)
	}
}
