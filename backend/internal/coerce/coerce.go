// Package coerce 共享的宽松数值 / 指针转换（#1445 P2 波 0b 的共享叶子之一）。
//
// 落点理由：原住 internal/service/helpers.go。域包搬走后还要用它们（模拟考、课程、题库、
// 顺序练习都有老数据回填与查询参数兜底），留在 service 就是「域包 → service」的反向依赖；
// 它们是无状态纯函数 —— P2 五种破环手法的第三种（无状态纯函数进叶子包）。
//
// 波 3b-1 追加 `StrPtr` / `RoundFloat1` / `RoundFloat2`：课程域搬包时发现留驻 `internal/service` 仍在用
// 同名助手（`ptrStr` / `roundFloat1` / `roundFloat2`），与 `Ptr` 同理由收编 —— 本包自述仍是
// 「数值 / 指针转换」，取整属于数值族、不稀释命名族。
//
// 两族语义在名字上分开，别混：
//   - 宽容族（ToFloat）：解析失败**不报错**、回退零值，用于「老数据/外部字段，脏值不该让整条链路崩」；
//   - 严格族（ParseFloat / ParseInt）：原样交回 strconv 的错误，用于「调用方能据此回 400」的入口。
//
// 命名统一 <Type>Ptr（IntPtr / FloatPtr / StrPtr，另有泛型 Ptr[T]）：构造指针的小工具，指针形参在请求 DTO 与
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

// Ptr 构造任意类型 T 的指针（泛型版；IntPtr/FloatPtr 是它的具名特例，保留是因为
// 调用点「类型 + Ptr」更好认）。给「键缺失/存在两态」的 DTO 指针字段用。
func Ptr[T any](v T) *T { return &v }

// IntPtr 返回 int 指针。
func IntPtr(v int) *int { return &v }

// FloatPtr 从 float64 构造指针。
func FloatPtr(v float64) *float64 { return &v }

// StrPtr 返回 string 指针（P2 波 3b-1 从 internal/service/course_types.go 的 ptrStr 收编）。
func StrPtr(v string) *string { return &v }

// RoundFloat1 保留 1 位小数（P2 波 3b-1 从 internal/service/course_service.go 的 roundFloat1 收编：
// 课程域与留驻的阅卷 / 练习 / 管理三域共用，留课程域会让它们反向依赖课程包）。
func RoundFloat1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// RoundFloat2 保留 2 位小数（同上，原 internal/service/course_service.go 的 roundFloat2）。
func RoundFloat2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
