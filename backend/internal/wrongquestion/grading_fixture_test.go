// 本包判分夹具（ADR-0070 波 4c：错题域测试从 internal/service 搬来，仍消费的助手就地在域包内联，
// 别去 import 留驻包的 _test.go —— 测试文件不跨包）。
package wrongquestion

// boolPtrVal 打印 *bool 时把 nil 显示成 nil 而不是地址（留驻侧同名函数的副本见
// internal/service/grading_fixture_test.go；原件是 internal/practicemode/grading_test.go 的同名函数）。
func boolPtrVal(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}
