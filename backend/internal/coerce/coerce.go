// Package coerce 共享的宽松数值 / 指针转换（#1445 P2 波 0b 的共享叶子之一）。
//
// 落点理由：原住 internal/service/helpers.go。域包搬走后还要用它们（模拟考、课程、题库、
// 顺序练习都有老数据回填与查询参数兜底），留在 service 就是「域包 → service」的反向依赖；
// 它们是无状态纯函数 —— P2 五种破环手法的第三种（无状态纯函数进叶子包）。
//
// 两族语义在名字上分开，别混：
//   - 宽容族（ToFloat）：解析失败**不报错**、回退零值，用于「老数据/外部字段，脏值不该让整条链路崩」；
//   - 严格族（ParseFloat / ParseInt）：原样交回 strconv 的错误，用于「调用方能据此回 400」的入口。
//
// 命名统一 <Type>Ptr（IntPtr / FloatPtr）：构造指针的小工具，指针形参在请求 DTO 与
// 「未传即 nil」的更新语义里到处都是，名字一律「类型 + Ptr」便于在调用点一眼认出。
//
// 波 0b 同时删掉了两枚**只转发标准库**的助手（照波 0a 的先例，不留一层壳）：
// withTimeout → context.WithTimeout(context.Background(), d)、containsString → slices.Contains。
package coerce

import "strconv"

// ToFloat 将任意类型转为 float64；不支持的类型与解析失败的字符串一律回退 0（宽容族）。
func ToFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case string:
		// 宽容语义：字符串解析失败回退 0。
		f, _ := ParseFloat(n)
		return f
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return 0
}

// ClampFloat 将 v 限制在 [min, max] 区间。
func ClampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ParseFloat 解析字符串为 float64，失败返回 strconv 原生错误（严格族）。
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ParseInt 解析字符串为 int，失败返回 strconv 原生错误（严格族）。
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// IntPtr 返回 int 指针。
func IntPtr(v int) *int { return &v }

// FloatPtr 从 float64 构造指针。
func FloatPtr(v float64) *float64 { return &v }
