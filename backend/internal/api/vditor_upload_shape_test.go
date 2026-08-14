package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/service"
)

// legacyVditorErrorEnvelope 历史 vditorError/vditorFeatureError 的 map 实现（shape-lock 参照物）。
// 深化后统一为 vditorError 单点，其 JSON 输出必须与深化前逐字一致。
func legacyVditorErrorEnvelope(msg string, errFiles []string) map[string]any {
	return map[string]any{
		"msg":  msg,
		"code": 1,
		"data": map[string]any{"errFiles": errFiles, "succMap": map[string]string{}},
	}
}

// legacyVditorSuccessEnvelope 历史成功响应（tutor/featured 原文，shape-lock 参照物）。
func legacyVditorSuccessEnvelope(name, url string) map[string]any {
	return map[string]any{
		"msg":  "",
		"code": 0,
		"data": map[string]any{"errFiles": []string{}, "succMap": map[string]string{name: url}},
	}
}

// TestVditorErrorEnvelopeShapeLock 冻结 Vditor 错误信封：code=1、errFiles/succMap 结构逐字一致。
func TestVditorErrorEnvelopeShapeLock(t *testing.T) {
	got, _ := json.Marshal(vditorError("未找到上传文件", []string{}))
	want, _ := json.Marshal(legacyVditorErrorEnvelope("未找到上传文件", []string{}))
	if string(got) != string(want) {
		t.Errorf("错误信封漂移\n got: %s\nwant: %s", got, want)
	}

	gotFiles, _ := json.Marshal(vditorError("文件保存失败", []string{"a.png"}))
	wantFiles, _ := json.Marshal(legacyVditorErrorEnvelope("文件保存失败", []string{"a.png"}))
	if string(gotFiles) != string(wantFiles) {
		t.Errorf("错误信封(带 errFiles)漂移\n got: %s\nwant: %s", gotFiles, wantFiles)
	}
}

// TestVditorSuccessEnvelopeShapeLock 冻结 Vditor 成功信封：code=0、succMap 以文件名映射 URL。
func TestVditorSuccessEnvelopeShapeLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fs := service.NewFileService("", nil, zap.NewNop())

	r := gin.New()
	r.POST("/upload-image", func(c *gin.Context) {
		uploadVditorImage(c, fs, func(content []byte, filename string) (string, error) {
			return "https://example.com/" + filename, nil
		})
	})

	// 构造 multipart 请求携带 1 张合法图片
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "a.png")
	_, _ = fw.Write([]byte{0x89, 'P', 'N', 'G'})
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload-image", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	want := legacyVditorSuccessEnvelope("a.png", "https://example.com/a.png")
	wantJSON, _ := json.Marshal(want)
	if rec.Body.String() != string(wantJSON) {
		t.Errorf("成功信封漂移\n got: %s\nwant: %s", rec.Body.String(), wantJSON)
	}
}
