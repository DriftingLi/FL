// Package handler 实现 HTTP 处理器
// 本文件：报告路径测试（#113 迁移 + 补电池覆盖）——评估与电池报告的
// 生成/再生成/缺文件再生成/并发单份生成，穿过与生产调用方相同的 seam。
package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/valuation/model"
)

// memStorage 计数存储 fake：统计 Save/Delete 次数（断言并发下载不重复上传、再生成清理旧 PDF）。
type memStorage struct {
	mu      sync.Mutex
	saves   int
	deletes int
	urls    map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{urls: map[string][]byte{}}
}

func (m *memStorage) Save(_ context.Context, key string, content []byte, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	m.urls[key] = content
	return "https://fake-cdn/" + key, nil
}

func (m *memStorage) Delete(_ context.Context, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes++
	delete(m.urls, strings.TrimPrefix(url, "https://fake-cdn/"))
	return nil
}

func (m *memStorage) Exists(_ context.Context, url string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.TrimPrefix(url, "https://fake-cdn/")
	_, ok := m.urls[key]
	return ok, nil
}

func (m *memStorage) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var urls []string
	for key := range m.urls {
		if strings.HasPrefix(key, prefix) {
			urls = append(urls, "https://fake-cdn/"+key)
		}
	}
	return urls, nil
}

func (m *memStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.TrimPrefix(url, "https://fake-cdn/")
	content, ok := m.urls[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func createEvalForReport(t *testing.T, r *gin.Engine) float64 {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("创建评估失败: %d\n%s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	return data["id"].(float64)
}

// TestReportGenerate_WritesPath 评估报告：上传 + 回写路径（既有行为保留，B3 迁移）。
func TestReportGenerate_WritesPath(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	id := createEvalForReport(t, r)

	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations/1/report", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("生成报告状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if data["pdf_url"] == nil || data["pdf_url"].(string) == "" {
		t.Error("生成报告缺少 pdf_url")
	}
	if int(data["file_size"].(float64)) != len("fake-pdf") {
		t.Errorf("file_size 错误: %v", data["file_size"])
	}
	if st.saves != 1 {
		t.Errorf("应上传 1 次, got %d", st.saves)
	}
	if id != 1 {
		t.Fatalf("预期 id=1, got %v", id)
	}
}

// TestReportDownload_RegeneratesOnMissing 下载时路径缺失 → 再生成 → 流式返回 PDF（B3 迁移 + 代理下载改造）。
func TestReportDownload_RegeneratesOnMissing(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	createEvalForReport(t, r)

	w := performRequest(r, http.MethodGet, "/api/valuation/evaluations/1/report", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("下载状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, 期望 application/pdf", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd == "" || !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition 应含 attachment, got %q", cd)
	}
	if st.saves != 1 {
		t.Errorf("缺失路径应再生成上传 1 次, got %d", st.saves)
	}
}

// TestReportDownload_ConcurrentSingleGeneration 并发下载同 ID 只产生一份 PDF（singleflight 去重，B3 迁移）。
func TestReportDownload_ConcurrentSingleGeneration(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	createEvalForReport(t, r)

	const n = 8
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/valuation/evaluations/1/report", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			codes[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("并发请求 %d 状态码 = %d, 期望 200", i, c)
		}
	}
	if st.saves != 1 {
		t.Errorf("并发下载应只上传 1 份 PDF（无孤儿文件）, got %d", st.saves)
	}
}

// batteryCreateRequest 电池评估创建请求（10 个循环，满足业务校验）。
func batteryCreateRequest() model.CreateBatteryRequest {
	cycles := make([]model.CycleData, 0, 10)
	for i := 1; i <= 10; i++ {
		cycles = append(cycles, model.CycleData{
			CycleIndex:    i,
			VoltageSeries: []float64{3.2, 3.3, 3.4, 3.5, 3.6},
			CurrentSeries: []float64{1.0, 1.0, 1.0, 1.0, 1.0},
			Capacity:      100 - float64(i),
		})
	}
	return model.CreateBatteryRequest{BatteryType: model.BatteryTypeLFP, BatteryModel: "LFP-100A", Cycles: cycles}
}

// createBatteryForReport 创建电池评估并断言成功，返回记录 ID。
func createBatteryForReport(t *testing.T, r *gin.Engine, auth string) int64 {
	t.Helper()
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/battery/evaluations", batteryCreateRequest(), auth)
	if w.Code != http.StatusOK {
		t.Fatalf("创建电池评估失败: %d\n%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("创建电池评估业务码 = %d\nbody=%s", code, w.Body.String())
	}
	id := int64(data["evaluation_id"].(float64))
	if id <= 0 {
		t.Fatalf("电池评估缺少 id: %v", data["evaluation_id"])
	}
	return id
}

// TestBatteryReport_GenerateWritesPath 电池报告：创建 → 列表 → 生成（上传 + 回写路径，B3 补测试）。
func TestBatteryReport_GenerateWritesPath(t *testing.T) {
	st := newMemStorage()
	r, _, _, batteryStore := newTestValuationEngineWithStorage(t, st)
	auth := authHeader(t, 1)
	id := createBatteryForReport(t, r, auth)

	// 列表：记录应出现在登录用户视角
	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/battery/evaluations", nil, auth)
	if w.Code != http.StatusOK {
		t.Fatalf("列表状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("列表业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if int(data["total"].(float64)) != 1 {
		t.Errorf("列表 total 应为 1, got %v", data["total"])
	}

	// 生成报告
	w = performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("生成电池报告状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data = decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("生成电池报告业务码 = %d\nbody=%s", code, w.Body.String())
	}
	pdfURL, _ := data["pdf_url"].(string)
	if pdfURL == "" {
		t.Error("生成电池报告缺少 pdf_url")
	}
	if int(data["file_size"].(float64)) == 0 {
		t.Error("生成电池报告 file_size 应为正数")
	}
	if st.saves != 1 {
		t.Errorf("应上传 1 次, got %d", st.saves)
	}

	// 路径已回写（store 侧可见）
	if batteryStore.records[id].ReportPdfPath == "" {
		t.Error("报告路径未回写")
	}
}

// TestBatteryReport_RegenerateDeletesOld 电池报告再生成：上传新 PDF + 清理旧 PDF（B3 补测试）。
// 上传 key 含秒级时间戳：间隔 1.1s 再生成保证新旧 key 不同，旧文件进入清理路径。
func TestBatteryReport_RegenerateDeletesOld(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	id := createBatteryForReport(t, r, authHeader(t, 1))

	w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("第 1 次生成电池报告状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	time.Sleep(1100 * time.Millisecond)
	w = performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("第 2 次生成电池报告状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if st.saves != 2 {
		t.Errorf("再生成应上传 2 次, got %d", st.saves)
	}
	// 每次再生成清理一份旧 PDF（第二次生成清理第一份）
	if st.deletes != 1 {
		t.Errorf("再生成应删除 1 份旧 PDF, got %d", st.deletes)
	}
}

// TestBatteryReport_DownloadRegeneratesOnMissing 电池报告下载时路径缺失 → 再生成 → 流式返回 PDF（B3 补测试 + 代理下载改造）。
func TestBatteryReport_DownloadRegeneratesOnMissing(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	id := createBatteryForReport(t, r, authHeader(t, 1))

	w := performRequest(r, http.MethodGet, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("电池报告下载状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	if st.saves != 1 {
		t.Errorf("缺失路径应再生成上传 1 次, got %d", st.saves)
	}
}

// TestBatteryReport_ConcurrentSingleGeneration 电池报告并发下载同 ID 只产生一份 PDF（singleflight 去重，B3 补测试）。
func TestBatteryReport_ConcurrentSingleGeneration(t *testing.T) {
	st := newMemStorage()
	r, _, _, _ := newTestValuationEngineWithStorage(t, st)
	id := createBatteryForReport(t, r, authHeader(t, 1))

	const n = 8
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			codes[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("并发请求 %d 状态码 = %d, 期望 200", i, c)
		}
	}
	if st.saves != 1 {
		t.Errorf("并发下载应只上传 1 份 PDF（无孤儿文件）, got %d", st.saves)
	}
}
