package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestFeaturedContract_DocxUpload docx 导入的 HTTP 面（2026-11）：
// 上传端点回统一信封三键、文件落 featured/docx/、非法扩展名落 400。
//
// 与 internal/featured 的 docx_content_test.go 分工：那边锁「一次写请求之后库里是什么状态」，
// 这里锁**路由 + 信封 + 存储落点**——三条都只有跑真实路由才算验过。
func TestFeaturedContract_DocxUpload(t *testing.T) {
	t.Parallel()
	r, _, token := newAdminContractEnv(t)

	// 合法上传：.docx 字节（内容不解析，只验落点与信封形状）
	rec := doMultipart(t, r, token, "/api/admin/featured-content/upload-docx", "方案.docx",
		[]byte("PK\x03\x04 fake docx bytes"))
	got := adminObject(t, rec, http.StatusOK)
	assertKeysExact(t, got, "docx_url", "file_name", "file_size")

	var docxURL string
	if err := json.Unmarshal(got["docx_url"], &docxURL); err != nil {
		t.Fatalf("docx_url 不是字符串: %v", err)
	}
	if !bytes.Contains([]byte(docxURL), []byte("/featured/docx/")) {
		t.Errorf("docx_url = %q，期望落在 featured/docx/ 子目录", docxURL)
	}
	if !bytes.Contains([]byte(docxURL), []byte(".docx")) {
		t.Errorf("docx_url = %q，期望保留 .docx 扩展名", docxURL)
	}

	// 用它创建一篇 docx 文章：类型与文件两列都该落库，正文列清空
	rec = doWithToken(t, r, token, http.MethodPost, "/api/admin/featured-content",
		map[string]any{"title": "Word 文章", "category": "industry",
			"content_type": "docx", "docx_url": docxURL})
	created := adminObject(t, rec, http.StatusCreated)
	if got := string(created["content_type"]); got != `"docx"` {
		t.Errorf("content_type = %s，期望 \"docx\"", got)
	}
	if got := string(created["docx_url"]); got != fmt.Sprintf("%q", docxURL) {
		t.Errorf("docx_url = %s，期望 %q", got, docxURL)
	}
	if got := string(created["content"]); got != `""` {
		t.Errorf("content = %s，docx 文章的正文列应为空串", got)
	}

	// 公开详情也带这两列（门户与 Web 读者面靠它切换渲染）
	contentID := jsonInt(t, created["content_id"])
	rec = doWithToken(t, r, token, http.MethodGet, fmt.Sprintf("/api/featured-content/%d?no_view=1", contentID), nil)
	// 未发布时公开详情 404，先发布
	if rec.Code == http.StatusNotFound {
		rec = doWithToken(t, r, token, http.MethodPost,
			fmt.Sprintf("/api/admin/featured-content/%d/publish", contentID), nil)
		adminObject(t, rec, http.StatusOK)
		rec = doWithToken(t, r, token, http.MethodGet, fmt.Sprintf("/api/featured-content/%d?no_view=1", contentID), nil)
	}
	public := adminObject(t, rec, http.StatusOK)
	assertHasKeys(t, public, "content_type", "docx_url", "content")

	// 非 docx 扩展名 ⇒ 400
	rec = doMultipart(t, r, token, "/api/admin/featured-content/upload-docx", "a.pdf", []byte("%PDF-1.7"))
	adminData(t, rec, http.StatusBadRequest)
}

// doMultipart 发一个单文件 multipart 请求（带 admin token）。
func doMultipart(t *testing.T, r *gin.Engine, token, path, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("写入 multipart 内容失败: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
