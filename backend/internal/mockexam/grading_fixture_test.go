// mockexam 包测试夹具（#1445 P2 波 4a）：boolPtr 一族的域内副本。
//
// 与留驻 internal/service/grading_fixture_test.go 的同名函数按接缝就地内联各一份
// （域包测试不得反向 import internal/service，同 3b-2 / 3c-2 先例）。
package mockexam

// boolPtrVal 把 *bool 归一成 any，供 DTO 字段断言（nil 与 false 是两件事）。
func boolPtrVal(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}
