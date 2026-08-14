package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// atoiDefault 字符串转 int，失败或为空时返回默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// queryIntPtr 解析可选整型查询参数（任意整数，含 0/负），非法或缺失时返回 nil。
// 用于非 ID 型参数（如 min_wrong_count）；ID 型参数改走 queryIDPtr（id>0 守卫）。
func queryIntPtr(c *gin.Context, key string) *int {
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

// queryIDPtr 解析可选 ID 型查询参数：非法、缺失或 <=0 均返回 nil。
// 这是 id>0 守卫的单点实现，替代各 handler 内联 strconv.Atoi + 手写 >0 判断。
func queryIDPtr(c *gin.Context, key string) *int {
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

// requiredPositiveID 解析必填的 ID 型查询参数字符串，非法或 <=0 返回 (0, false)。
// 调用方据此区分「缺失」与「非法」两种提示（如 practice-mode tag_id）。
func requiredPositiveID(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
