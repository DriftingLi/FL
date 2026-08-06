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

// queryIntPtr 解析整型查询参数，非法或缺失时返回 nil。
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
