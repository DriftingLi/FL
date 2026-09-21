// 错误面文案的三种形态必须各自可表达且互不吞掉（ADR-0060 票1b 实施回记：第五种形态）。
// seam：(*errStatusTable).renderError 输出的信封字节。
// 病根：票1b 首轮收编把 `response.ServerError(c, "查询失败: "+err.Error())` 收成 errStatusAll(500)，
// 前缀被静默丢掉，而没有任何一条测试断言过前缀 —— 全绿是假绿。本文件就是那条断言。
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func renderWithTable(t *testing.T, tbl *errStatusTable, err error) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tbl.renderError(c, err)
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatalf("解析信封失败: %v body=%s", e, w.Body.String())
	}
	return body.Code, body.Message
}

func TestErrTableMessageShapes(t *testing.T) {
	boom := errors.New("db down")

	code, msg := renderWithTable(t, errStatusAll(http.StatusInternalServerError), boom)
	if code != 500 || msg != "db down" {
		t.Errorf("裸形态应回 500 + 错误原文，实际 %d %q", code, msg)
	}

	code, msg = renderWithTable(t, errStatusAllMsg(http.StatusNotFound, "会话不存在"), boom)
	if code != 404 || msg != "会话不存在" {
		t.Errorf("固定文案形态应回 404 + 该文案，实际 %d %q", code, msg)
	}

	// 第五种：前缀 + 错误原文（旧闭包 `response.ServerError(c, "查询失败: "+err.Error())` 的等价收编）
	code, msg = renderWithTable(t, errStatusAllPrefix(http.StatusInternalServerError, "查询失败: "), boom)
	if code != 500 || msg != "查询失败: db down" {
		t.Errorf("前缀形态应回 500 + 「查询失败: db down」，实际 %d %q", code, msg)
	}

	// 无条件条目必须抢在 *ParseError 规则之前（旧闭包对解析错误也回同一个码），
	// 而 {fallback} 表不是无条件：解析错误回自己的状态码。两者不可互换。
	pe := badRequest("参数错了")
	if c, _ := renderWithTable(t, errStatusAll(http.StatusInternalServerError), pe); c != 500 {
		t.Errorf("无条件条目应把解析错误也渲染成 500（与旧闭包逐字等价），实际 %d", c)
	}
	if c, _ := renderWithTable(t, &errStatusTable{fallback: http.StatusInternalServerError}, pe); c != http.StatusBadRequest {
		t.Errorf("fallback 表应让解析错误回自己的 400，实际 %d", c)
	}
}
