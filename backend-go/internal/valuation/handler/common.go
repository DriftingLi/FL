// Package handler 实现 HTTP 处理器
// 本文件：统一响应包装，所有接口返回 {code, message, data} 格式
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 业务状态码定义
// 0 表示成功，其他值表示业务失败（非 HTTP 状态码）
const (
	CodeOK                 = 0     // 成功
	CodeBadRequest         = 40000 // 请求参数错误
	CodeInvalidParam       = 40001 // 业务参数校验失败
	CodeNotFound           = 40400 // 资源未找到
	CodeMethodNotAllowed   = 40500 // 方法不允许
	CodeInternalError      = 50000 // 服务器内部错误
	CodeDatabaseError      = 50001 // 数据库错误
	CodeUpstreamError      = 50002 // 上游服务错误
)

// Response 统一响应结构
// 与 API 设计文档保持一致：{code, message, data}
type Response struct {
	Code    int         `json:"code"`              // 业务状态码
	Message string      `json:"message"`           // 提示信息
	Data    interface{} `json:"data,omitempty"`    // 业务数据（成功时返回）
}

// OK 返回成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeOK,
		Message: "success",
		Data:    data,
	})
}

// Fail 返回业务失败响应（HTTP 200 + 业务非 0 状态码）
// 这种设计便于前端在单一 catch 中处理所有业务错误
func Fail(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// FailWithData 返回业务失败响应并附带数据
func FailWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Error 返回 HTTP 错误（如 4xx/5xx 状态码 + 业务响应体）
// 用于客户端明显错误（参数缺失、404 等）
func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
	c.Abort()
}
