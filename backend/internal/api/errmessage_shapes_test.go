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

	// 无条件条目**不再**吃解析错误（ADR-0062 票8：*ParseError 恒优先）——它回自己的状态码与自己的文案，
	// 三个无条件条目构造器（WithSuccess 用的就是 errStatusAll）一条规则全盖住：
	// 固定文案槽与人读前缀都不参与解析错误的渲染。
	pe := badRequest("参数错了")
	for name, tbl := range map[string]*errStatusTable{
		"errStatusAll":       errStatusAll(http.StatusInternalServerError),
		"errStatusAllMsg":    errStatusAllMsg(http.StatusNotFound, "会话不存在"),
		"errStatusAllPrefix": errStatusAllPrefix(http.StatusInternalServerError, "查询失败: "),
	} {
		if c, m := renderWithTable(t, tbl, pe); c != http.StatusBadRequest || m != "参数错了" {
			t.Errorf("%s 不得吞掉解析错误（应回 400 +「参数错了」），实际 %d %q", name, c, m)
		}
	}
	// {fallback} 表一直是这个行为；两者现在只在「非解析错误」上分岔：无条件条目兜一切、fallback 只兜未命中。
	if c, _ := renderWithTable(t, &errStatusTable{fallback: http.StatusInternalServerError}, pe); c != http.StatusBadRequest {
		t.Errorf("fallback 表应让解析错误回自己的 400，实际 %d", c)
	}
	// 非解析错误：两种表各自的原形状（无条件条目 = 该条的状态码 + 该条的文案槽；fallback = 兜底码）
	if c, _ := renderWithTable(t, errStatusAll(http.StatusInternalServerError), errors.New("db down")); c != http.StatusInternalServerError {
		t.Errorf("无条件条目仍兜住业务/DB 错误，实际 %d", c)
	}
	if c, _ := renderWithTable(t, &errStatusTable{fallback: http.StatusInternalServerError}, errors.New("db down")); c != http.StatusInternalServerError {
		t.Errorf("fallback 表仍兜住未命中错误，实际 %d", c)
	}
}
