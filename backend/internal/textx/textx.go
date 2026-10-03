// Package textx 文本处理的叶子包（P2 波 4b，ADR-0070）：跨域共享的无状态纯函数。
// 收藏域与搜索域都要截断摘要，放任一域包都会让另一个域 import 兄弟域 ⇒ 按 ADR-0070 的第三种破法进叶子包。
package textx

import "unicode/utf8"

// Snippet 截取前 n 个 rune 作为摘要（超出加省略号）。
func Snippet(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}
