// Package httpx 是 HTTP 面的**跨包共享出口**。当前承载请求解析单点与解析错误值
// （ParseError / BadRequest / PathInt / PathInt64）；端点骨架（Endpoint 与 ErrStatus 域表）
// 仍在 internal/api，等它的消费面（域包自带 handler）落定后再一并迁入。
//
// 为什么先有这一步：路径整型 id 的解析此前只有 internal/api 一处的出口（pathInt/pathInt64），
// 而 internal/valuation/handler 另有 6 处裸 `strconv.ParseInt(c.Param("id"), …)` 收不进来 ——
// 缺的正是「一枚跨包共享的解析出口」（ADR-0065 决策 1 末段登记的债务）。
// 本包就是那枚出口：HTTP 面上解析请求一律从这里取，不再各写一份。
package httpx

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseError 请求解析失败的哨兵错误：携带 HTTP 状态码与用户可见文案。
type ParseError struct {
	Status  int
	Message string
}

func (e *ParseError) Error() string { return e.Message }

// BadRequest 构造 400 解析错误值（参数错误统一出口）。
//
// 与响应层的 `response.BadRequest` 是两件事：那个**写**一个错误信封，这个只**构造**一个
// 交给端点骨架渲染的解析错误值（骨架按 Status/Message 落码与文案）。
func BadRequest(msg string) *ParseError {
	return &ParseError{Status: http.StatusBadRequest, Message: msg}
}

// PathInt 解析路径参数为正整数 id；非数字、0 与负数一律 400（带调用方给的那句文案）。
//
// 「路径上的整数 id 不是正整数」是一件**解析层**事实，与「这个资源不存在」无关：改之前这里只看
// `strconv.Atoi` 的 err ⇒ 0 与负数被放行到 service，于是 `GET /course/0` 对外答 404「课程不存在」
// （拿一个不存在的 id 冒充一个不存在的资源），而用户/讲师面因 service 有 `id <= 0` guard 答 400，
// 且用的是另一句文案（「用户 ID 非法」）——同一件输入错误在三个地方说出三种话（ADR-0065 决策 1）。
// `PathInt64` 一直是这里的形状，本函数向它对齐。
func PathInt(c *gin.Context, key, failMsg string) (int, error) {
	v, err := strconv.Atoi(c.Param(key))
	if err != nil || v <= 0 {
		return 0, BadRequest(failMsg)
	}
	return v, nil
}

// PathInt64 解析路径参数为正整数 id（int64 版），判定与 PathInt 逐字相同。
// 两枚 helper 是**仅有的**两处路径整数解析点，由 pkg/httpx 独占；
// path_parse_point_drift_lock_test.go 钉住「不许再出现第三处」。
func PathInt64(c *gin.Context, key, failMsg string) (int64, error) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || v <= 0 {
		return 0, BadRequest(failMsg)
	}
	return v, nil
}
