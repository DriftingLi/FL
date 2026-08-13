// Package report 测试：以 Spec 槽位（loader/render/writer/prepare + 存储替身）
// 直接构造协调器，验证生成/下载/再生成/并发去重语义（决策 D5）。
// 两个真实 producer（评估/电池）是 spec 槽位的 adapter；这里用假槽位做单元验证，
// HTTP 层翻译由 handler 包契约测试覆盖。
package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// memStorage 内存存储替身：满足 storage.Storage，key → 内容。
type memStorage struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMemStorage() *memStorage { return &memStorage{m: map[string][]byte{}} }

func (s *memStorage) Save(_ context.Context, key string, content []byte, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = content
	return "/static/uploads/" + key, nil
}

func (s *memStorage) Delete(_ context.Context, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.m {
		if "/static/uploads/"+k == url {
			delete(s.m, k)
		}
	}
	return nil
}

func (s *memStorage) Exists(_ context.Context, url string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.m {
		if "/static/uploads/"+k == url {
			return true, nil
		}
	}
	return false, nil
}

func (s *memStorage) List(context.Context, string) ([]string, error) { return nil, nil }

func (s *memStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.TrimPrefix(url, "/static/uploads/")
	content, ok := s.m[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *memStorage) hasKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[key]
	return ok
}

// evalRec 假记录：path 是报告 URL 字段，prep/render 记录槽位调用次数。
// 语义与生产一致：rec 是加载时的快照，Writer 写持久层、不改变内存 rec。
type evalRec struct {
	path   string
	prep   int
	render int
}

// newEvalSpec 组装评估型 Spec（两个 producer 形状之一），返回 spec 与记录。
// written 承载 Writer 的落库结果（模拟 DB 写），rec.path 保持加载时快照。
func newEvalSpec(st *memStorage, rec *evalRec, written *string) Spec[evalRec] {
	return Spec[evalRec]{
		Loader: func(_ context.Context, id int64) (*evalRec, error) {
			if rec == nil {
				return nil, errors.New("record not found")
			}
			return rec, nil
		},
		PathOf: func(r *evalRec) string { return r.path },
		Writer: func(_ context.Context, _ int64, url string) error {
			*written = url
			return nil
		},
		Prepare: func(_ context.Context, r *evalRec) { r.prep++ },
		Render: func(_ context.Context, r *evalRec) ([]byte, error) {
			r.render++
			return []byte("pdf-bytes"), nil
		},
		KeyPrefix: "reports/evaluation_report_",
		Logger:    zap.NewNop(),
		Storage:   st,
	}
}

func TestGenerate_WritesPDFAndPath(t *testing.T) {
	st := newMemStorage()
	rec := &evalRec{}
	var written string
	c := New(newEvalSpec(st, rec, &written))

	res, err := c.Generate(context.Background(), 1)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if res.ID != 1 || res.FileSize != len("pdf-bytes") || res.PDFURL == "" {
		t.Fatalf("Generate 结果异常: %+v", res)
	}
	if rec.prep != 1 || rec.render != 1 {
		t.Errorf("Prepare/Render 应各调用 1 次，got prep=%d render=%d", rec.prep, rec.render)
	}
	if written != res.PDFURL {
		t.Errorf("回写路径 = %q，期望 %q", written, res.PDFURL)
	}
}

func TestDownloadURL_UsesExistingWithoutRegenerate(t *testing.T) {
	st := newMemStorage()
	if _, err := st.Save(context.Background(), "reports/evaluation_report_1_old.pdf", []byte("old"), ""); err != nil {
		t.Fatalf("预置旧文件失败: %v", err)
	}
	rec := &evalRec{path: "/static/uploads/reports/evaluation_report_1_old.pdf"}
	var written string
	c := New(newEvalSpec(st, rec, &written))

	url, err := c.DownloadURL(context.Background(), 1)
	if err != nil {
		t.Fatalf("DownloadURL 失败: %v", err)
	}
	if url != rec.path {
		t.Errorf("应直接返回既有 URL，got %q want %q", url, rec.path)
	}
	if rec.render != 0 {
		t.Errorf("既有文件存在时不应再生成，render=%d", rec.render)
	}
}

func TestDownloadURL_RegeneratesWhenMissing(t *testing.T) {
	st := newMemStorage()
	// path 指向存储中不存在的文件（旧记录 URL 失效）
	oldPath := "/static/uploads/reports/evaluation_report_1_gone.pdf"
	rec := &evalRec{path: oldPath}
	var written string
	c := New(newEvalSpec(st, rec, &written))

	url, err := c.DownloadURL(context.Background(), 1)
	if err != nil {
		t.Fatalf("DownloadURL 失败: %v", err)
	}
	if url == oldPath {
		t.Error("失效 URL 应触发再生成并返回新 URL")
	}
	if rec.render != 1 {
		t.Errorf("应再生成 1 次，render=%d", rec.render)
	}
	if written == "" || written == oldPath {
		t.Errorf("新路径应回写，written=%q", written)
	}
}

func TestGenerate_DeletesOldPDFAfterWriteback(t *testing.T) {
	st := newMemStorage()
	if _, err := st.Save(context.Background(), "reports/evaluation_report_1_old.pdf", []byte("old"), ""); err != nil {
		t.Fatalf("预置旧文件失败: %v", err)
	}
	rec := &evalRec{path: "/static/uploads/reports/evaluation_report_1_old.pdf"}
	var written string
	c := New(newEvalSpec(st, rec, &written))

	if _, err := c.Generate(context.Background(), 1); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if st.hasKey("reports/evaluation_report_1_old.pdf") {
		t.Error("新 PDF 回写成功后旧 PDF 应被清理")
	}
}

func TestDownloadURL_ConcurrentSameID_SingleGeneration(t *testing.T) {
	st := newMemStorage()
	rec := &evalRec{}
	var written string
	spec := newEvalSpec(st, rec, &written)

	// Render 阻塞在 release 屏障上；全部 goroutine 抵达后留出调度窗口，
	// 确保最后抵达者的 Do 调用落在在途 fn 内（fast-fake 场景 goroutine 启动 ~µs，
	// 50ms 窗口足够覆盖 8 个 goroutine 的 Do 全部发出）。
	release := make(chan struct{})
	origRender := spec.Render
	spec.Render = func(ctx context.Context, r *evalRec) ([]byte, error) {
		<-release
		return origRender(ctx, r)
	}
	c := New(spec)

	const n = 8
	var arrived int32
	arrivedCh := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if atomic.AddInt32(&arrived, 1) == n {
				close(arrivedCh)
			}
			_, errs[i] = c.DownloadURL(context.Background(), 1)
		}(i)
	}
	<-arrivedCh
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d DownloadURL 失败: %v", i, err)
		}
	}
	if rec.render != 1 {
		t.Errorf("同 ID 并发下载只应生成 1 份 PDF，render=%d", rec.render)
	}
}

func TestGenerate_ConcurrentSameID_SingleGeneration(t *testing.T) {
	st := newMemStorage()
	rec := &evalRec{}
	var written string
	spec := newEvalSpec(st, rec, &written)

	release := make(chan struct{})
	origRender := spec.Render
	spec.Render = func(ctx context.Context, r *evalRec) ([]byte, error) {
		<-release
		return origRender(ctx, r)
	}
	c := New(spec)

	const n = 8
	var arrived int32
	arrivedCh := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]GenerateResult, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if atomic.AddInt32(&arrived, 1) == n {
				close(arrivedCh)
			}
			results[i], errs[i] = c.Generate(context.Background(), 1)
		}(i)
	}
	<-arrivedCh
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d Generate 失败: %v", i, err)
		}
	}
	if rec.render != 1 {
		t.Errorf("同 ID 并发 POST 只应生成 1 份 PDF，render=%d", rec.render)
	}
	for i := 1; i < n; i++ {
		if results[i].PDFURL != results[0].PDFURL {
			t.Errorf("并发 POST 应拿到同一份 PDF，goroutine %d URL 不一致", i)
		}
	}
}
