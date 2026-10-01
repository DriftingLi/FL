// Package httpx 是 HTTP 面的**跨包共享出口**：请求解析单点（ParseError / BadRequest /
// PathInt / PathInt64 / QueryIntPtr / QueryIDPtr / QueryIntDefault / PositiveID）与端点骨架（Endpoint 与 ErrStatus 域表，endpoint.go）
// 都住在这里；域包（internal/<域>/）自带的 handler 一律从这里取解析出口，不再各写一份。
//
// 为什么要有这个包：路径整型 id 的解析此前只有 internal/api 一处的出口（pathInt/pathInt64），
// 而 internal/valuation/handler 另有 6 处裸 `strconv.ParseInt(c.Param("id"), …)` 收不进来 ——
// 缺的正是「一枚跨包共享的解析出口」（ADR-0065 决策 1 末段登记的债务）。
// 域包自带 handler 后这条要求更硬：解析出口若留在 internal/api，域包就只能反过来依赖装配根。
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

// QueryIntPtr 解析可选整型查询参数（任意整数，含 0/负），非法或缺失时返回 nil。
// 用于非 ID 型参数（如 min_wrong_count）；ID 型参数另走带 `id > 0` 守卫的那枚（QueryIDPtr）。
// 与 PathInt 一样，这里是**请求解析**：只看输入合法性，
// 不判断资源是否存在（nil 表示「不筛这一维」，不是「参数错了」）。
//
// 为什么从 internal/api 搬进来：域包自带 handler 后，解析出口若留在装配根，域包就要
// 反向依赖 internal/api（域包迁移手册：解析类助手一律升级到本包，见 ADR-0070）。
func QueryIntPtr(c *gin.Context, key string) *int {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

// QueryIntDefault 解析带默认值的整型查询参数（分页/条数这类）：缺失或非数字时返回 def，
// 0 与负数**原样透传**（钳制是服务层的事，解析层不替它决定）。
//
// 与 QueryIntPtr 的分工：那枚表达「没给这一维筛选项」（nil），这枚表达「这一维有默认值」。
// 为什么升级到这里：internal/api 的 atoiDefault 是包私有且吃字符串，域包自带 handler 后拿不到它，
// 各域再抄一份就是同一件解析事实的第二处实现（域包迁移手册：解析类助手一律升级到本包，ADR-0070）。
// 本包是它的唯一宿主，path_parse_point_drift_lock_test.go 的 queryParseHelperNames 钉住这件事。
func QueryIntDefault(c *gin.Context, key string, def int) int {
	s := c.Query(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// QueryIDPtr 解析可选 **ID 型**查询参数：非法、缺失或 <=0 均返回 nil。
// 这是 id>0 守卫的单点实现，替代各 handler 内联 strconv.Atoi + 手写 >0 判断。
//
// 与 QueryIntPtr 的分工：那枚是「任意整数」（含 0/负，用于 min_wrong_count 这类非 ID 维度），
// 这枚是 ID 型、带 >0 守卫；与 PositiveID 的分工：那枚只吃字符串、把「缺失 vs 非法」的分流留给调用方。
// 为什么从 internal/api 搬进来：它原是包私有（queryIDPtr），域包自带 handler 后拿不到它
// （域包迁移手册：解析类助手一律升级到本包，ADR-0070）。
// 本包是查询侧解析出口的唯一宿主，path_parse_point_drift_lock_test.go 的 queryParseHelperNames 钉住这件事。
func QueryIDPtr(c *gin.Context, key string) *int {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

// PositiveID 解析必填的 ID 型**参数字符串**，非法或 <=0 返回 (0, false)。
// 调用方已从查询串或请求体取到原文、并要自己分流「缺失」（空串）与「非法」（非正整数）两种提示时用它
// （如 practice-mode 的 tag_id 答「请指定题库标签」/「题库标签ID无效」两句不同的话）。
//
// 与 PathInt 的关系：那枚直接吃 *gin.Context 与路径键、把 400 也一并构造好；这枚是纯字符串解析、
// 不带 HTTP 语义。为什么从 internal/api 搬进来：它原是包私有（requiredPositiveID），
// 域包自带 handler 后拿不到它（域包迁移手册：解析类助手一律升级到本包，ADR-0070）。
func PositiveID(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
