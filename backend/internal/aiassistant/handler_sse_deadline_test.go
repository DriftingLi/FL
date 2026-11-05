package aiassistant

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestWriteSSEHeadersExtendsWriteDeadline 钉住「SSE 写超时必须拉到 aiStreamTimeout」这条纪律。
//
// 背景：#2240 切流后带图提问在 60~90 秒之间必然失败，浏览器报 network error，而后端日志里
// 那条 access 记录仍是 200（状态码在超时前就写出去了），排查一度以为是对端问题。真因是
// http.Server 的 60 秒全局 WriteTimeout 比诊断链路的 aiStreamTimeout（300 秒）短得多。
// 这个用例在 httptest 里没有真实的 http.Server 超时，所以断言的是「SetWriteDeadline 被调到
// 了 aiStreamTimeout 量级」而不是连接真的活过 60 秒 —— 让后来人改短它时至少红一次。
func TestWriteSSEHeadersExtendsWriteDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	before := time.Now()
	writeSSEHeaders(c)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q，期望 text/event-stream", got)
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q，期望 no", got)
	}
	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", got)
	}

	// httptest.ResponseRecorder 不实现 SetWriteDeadline，ResponseController 会返回
	// ErrNotSupported —— 这正是「降级」分支，标记头必须出现，且不能把请求打挂。
	if rec.Header().Get("X-SSE-Deadline") != "unset" {
		t.Fatal("ResponseRecorder 不支持 SetWriteDeadline 时应打 X-SSE-Deadline: unset 降级标记")
	}
	if elapsed := time.Since(before); elapsed > time.Second {
		t.Fatalf("writeSSEHeaders 不应阻塞，实际耗时 %v", elapsed)
	}

	// 纪律本体：SSE 上限必须远大于 http.Server 的 60 秒写超时（main.go），
	// 否则长答案会在写回浏览器时被自己的服务器掐断。
	if aiStreamTimeout <= 60*time.Second {
		t.Fatalf("aiStreamTimeout = %v，必须大于 http.Server 的 60 秒 WriteTimeout", aiStreamTimeout)
	}
}

// TestWriteSSEHeadersSetsDeadlineOnRealServer 在真实 http.Server 上确认能摸到底层连接。
func TestWriteSSEHeadersSetsDeadlineOnRealServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var reached bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		writeSSEHeaders(c)
		reached = w.Header().Get("X-SSE-Deadline") == ""
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	if !reached {
		t.Fatal("真实 http.Server 上 ResponseController 应能拿到连接，不该走降级分支")
	}
}
