// 本文件：导师文件库 HTTP 契约（#2494）——路由装配、鉴权、哨兵错误→状态码映射与端到端上传面。
package tutorfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/admincap"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// newTutorFileRouter 装配文件库蓝图测试路由器（与装配根 NewRouter 的讲师端注册同义）。
func newTutorFileRouter(t *testing.T) (*gin.Engine, *Service, *config.Config) {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "tutorfile-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	st := storage.NewLocalStorage(t.TempDir())
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	svc := NewService(db, st)

	r := gin.New()
	api := r.Group("/api")
	api.Use(middleware.AdminCapabilityResolver(admincap.New(db, zap.NewNop())))
	RegisterRoutes(api, sess, svc)
	return r, svc, cfg
}

// issueTutorToken 签发讲师 token（与 course_domain_face_ledger_test 的讲师面同型）。
func issueTutorToken(t *testing.T, cfg *config.Config, tutorID int, account string) string {
	t.Helper()
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	tok, err := sess.Issue(tutorID, account, core.TutorRole)
	if err != nil {
		t.Fatalf("签发讲师 token 失败: %v", err)
	}
	return tok
}

func doJSON(t *testing.T, r *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("编码请求体失败: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doRaw(t *testing.T, r *gin.Engine, token, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHTTPUploadFlowEndToEnd(t *testing.T) {
	r, _, cfg := newTutorFileRouter(t)
	tok := issueTutorToken(t, cfg, 1, "tutor_a")

	// init
	w := doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload",
		InitUploadRequest{FileName: "录课.mp4", FileSize: 2*testChunkSz + 4, ChunkSize: testChunkSz})
	if w.Code != http.StatusOK {
		t.Fatalf("init 状态 = %d, body = %s", w.Code, w.Body.String())
	}
	var initResp struct {
		Data InitUploadResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("解析 init 响应失败: %v", err)
	}
	if initResp.Data.TotalChunks != 3 {
		t.Fatalf("total_chunks = %d, want 3", initResp.Data.TotalChunks)
	}
	sid := initResp.Data.SessionID

	// 乱序三片
	for _, c := range []struct {
		n int
		b []byte
	}{{3, []byte("DDDD")}, {1, bytesOf('A', int(testChunkSz))}, {2, bytesOf('B', int(testChunkSz))}} {
		w = doRaw(t, r, tok, http.MethodPut,
			fmt.Sprintf("/api/tutor/files/upload/%s/chunk/%d", sid, c.n), c.b)
		if w.Code != http.StatusOK {
			t.Fatalf("chunk %d 状态 = %d, body = %s", c.n, w.Code, w.Body.String())
		}
	}

	// complete
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload/"+sid+"/complete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("complete 状态 = %d, body = %s", w.Code, w.Body.String())
	}

	// 列表可见
	w = doJSON(t, r, tok, http.MethodGet, "/api/tutor/files", nil)
	var listResp struct {
		Data FileListDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if len(listResp.Data.Files) != 1 {
		t.Fatalf("列表应有 1 行: %+v", listResp.Data.Files)
	}
	fileID := listResp.Data.Files[0].FileID

	// 删除
	w = doJSON(t, r, tok, http.MethodDelete, fmt.Sprintf("/api/tutor/files/%d", fileID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("删除状态 = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestHTTPAuthRequired(t *testing.T) {
	r, _, _ := newTutorFileRouter(t)
	for _, c := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/tutor/files/upload"},
		{http.MethodGet, "/api/tutor/files"},
	} {
		w := doJSON(t, r, "", c.method, c.path, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s 未认证应 401, got %d", c.method, c.path, w.Code)
		}
	}
}

func TestHTTPErrorStatusMapping(t *testing.T) {
	r, _, cfg := newTutorFileRouter(t)
	tok := issueTutorToken(t, cfg, 1, "tutor_a")

	// 404：不存在的会话
	w := doRaw(t, r, tok, http.MethodPut, "/api/tutor/files/upload/no-such/chunk/1", []byte("AAAAAAAA"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在会话应 404, got %d: %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload/no-such/complete", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在会话 complete 应 404, got %d", w.Code)
	}

	// 400：init 参数越界
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload",
		InitUploadRequest{FileName: "evil.exe", FileSize: testChunkSz, ChunkSize: testChunkSz})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("类型越界 init 应 400, got %d: %s", w.Code, w.Body.String())
	}

	// 409：abort 后 complete
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload",
		InitUploadRequest{FileName: "a.mp4", FileSize: 8, ChunkSize: testChunkSz})
	var initResp struct {
		Data InitUploadResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &initResp)
	sid := initResp.Data.SessionID
	_ = doRaw(t, r, tok, http.MethodPut, "/api/tutor/files/upload/"+sid+"/chunk/1", []byte("AAAAAAAA"))
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload/"+sid+"/abort", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("abort 状态 = %d", w.Code)
	}
	w = doJSON(t, r, tok, http.MethodPost, "/api/tutor/files/upload/"+sid+"/complete", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("abort 后 complete 应 409, got %d: %s", w.Code, w.Body.String())
	}

	// 404：删除不存在的文件
	w = doJSON(t, r, tok, http.MethodDelete, "/api/tutor/files/999999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("删除不存在文件应 404, got %d", w.Code)
	}
}
