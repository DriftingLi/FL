package api

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// performRequest 向测试路由器发起 HTTP 请求并返回响应记录器。
func performRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}
