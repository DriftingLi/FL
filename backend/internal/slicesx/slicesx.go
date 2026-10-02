// Package slicesx 切片小助手（#1445 P2 波 3b-1 的共享叶子）。
//
// 落点理由：Ints 原住 internal/service/training_catalog_service.go 的 dedupeInts，课程域
// （前置课程去重）与培训域（题目标签去重）都在用，而两域互为强环 ⇒ 进叶子包。
// 为什么不进 internal/coerce：那个包的自述是「宽松数值 / 指针转换」，切片去重不属于该词汇，
// 混进去会稀释它的命名族（<Type>Ptr / Parse* / To* / Clamp*）。
package slicesx

// Ints 去重并保持顺序。
func Ints(vals []int) []int {
	seen := make(map[int]struct{}, len(vals))
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
