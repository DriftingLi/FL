// Package response 提供统一响应结构 {code, message, data}。
package response

import "github.com/gin-gonic/gin"

// R 统一响应体。
type R struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    any `json:"data"`
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
func ServerError(c *gin.Context, msg string) {
	c.JSON(500, R{Code: 500, Message: msg, Data: nil})
}
