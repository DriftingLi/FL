// Package response 提供统一响应结构 {code, message, data}。
package response

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// R 统一响应体。
type R struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Success 输出 200 成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(200, R{Code: 200, Message: "success", Data: data})
}

// SuccessWithMsg 输出 200 成功响应，自定义 message。
func SuccessWithMsg(c *gin.Context, msg string, data any) {
	c.JSON(200, R{Code: 200, Message: msg, Data: data})
}

// Created 输出 201 创建成功响应。
func Created(c *gin.Context, msg string, data any) {
	c.JSON(201, R{Code: 201, Message: msg, Data: data})
}

// BadRequest 输出 400 错误响应。
func BadRequest(c *gin.Context, msg string) {
	c.JSON(400, R{Code: 400, Message: msg, Data: nil})
}

// Unauthorized 输出 401 未认证响应。
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(401, R{Code: 401, Message: msg, Data: nil})
}

// Forbidden 输出 403 无权限响应。
func Forbidden(c *gin.Context, msg string) {
	c.JSON(403, R{Code: 403, Message: msg, Data: nil})
}

// NotFound 输出 404 未找到响应。
func NotFound(c *gin.Context, msg string) {
	c.JSON(404, R{Code: 404, Message: msg, Data: nil})
}

// ServerError 输出 500 服务器错误响应。
// ⚠️ msg 必须是**固定文案**：5xx 不得外发驱动/ORM 原文（见 ClientErrorText 与 ServerErrorCause）。
func ServerError(c *gin.Context, msg string) {
	c.JSON(500, R{Code: 500, Message: msg, Data: nil})
}

// ClientErrorText 是「4xx 回错误原文 / 5xx 不外发驱动原文」这条规则的**唯一实现**
// （ADR-0064 决策 9）。原先住在 internal/api 的端点渲染单点，P1 提到本包：不经端点表的
// 裸 handler（文件流 / SSE / 裸字节 / valuation 两个 handler 包）也要用同一条判据，
// 不能再各抄一遍「保留前缀、丢掉尾巴」。
//
//	4xx —— 「前缀 + 错误自身文本」。4xx 文案本来就是给调用方看的领域说明
//	       （「证件不存在」「专业方向编码已存在」），收掉会直接伤可用性。
//	5xx —— 一律不回 err.Error()：驱动/ORM 原文（record not found、no such table、
//	       SQL logic error、pq: …、dial tcp …）进 message 等于把实现细节交给外部。
//	       有前缀时保留前缀本身（「导出失败: 」→「导出失败」），丢掉的是尾巴；
//	       没有前缀时回「服务器内部错误」。真实错误由调用方经 c.Error 记账（日志面不丢）。
func ClientErrorText(status int, err error, prefix string) string {
	if status < http.StatusInternalServerError {
		if err == nil {
			return prefix
		}
		return prefix + err.Error()
	}
	if trimmed := strings.TrimRight(prefix, " :："); trimmed != "" {
		return trimmed
	}
	return "服务器内部错误"
}

// ServerErrorCause 5xx 的一次做齐形态：**固定文案（保留前缀、丢尾巴）+ c.Error 记账**。
// 不经端点表的裸 handler 用它，替代 `ServerError(c, "导出失败: "+err.Error())` 那一族手抄
// （ADR-0064 决策 9 的执行面；判据仍是 ClientErrorText 一处）。
// prefix 为空表示「无前缀」，响应回「服务器内部错误」。
func ServerErrorCause(c *gin.Context, prefix string, err error) {
	if err != nil {
		_ = c.Error(err) //nolint:errcheck // gin 的 Error 只记账，返回值是链式用的
	}
	ServerError(c, ClientErrorText(http.StatusInternalServerError, err, prefix))
}

// PageCount 计算分页页数：ceil(total/pageSize) 的唯一实现。
// pageSize<=0 按 1 处理（调用方均已先归一化，防御除零）；total=0 时为 0 页。
func PageCount(total int64, pageSize int) int {
	if pageSize <= 0 {
		pageSize = 1
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
