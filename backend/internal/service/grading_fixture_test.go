// 留驻 service 侧判分夹具（ADR-0070 波 3c-2：判分内核搬进 internal/practicemode 之后，
// 仍在 service 测试里被消费的那几件按接缝就地内联，别去 import 域包的 _test.go —— 测试文件不跨包）。
package service

// boolPtrVal 打印 *bool 时把 nil 显示成 nil 而不是地址（原件见
// internal/practicemode/grading_test.go 的同名函数；消费者是 mock_exam_total_score_test.go
// 与 wrong_question_service_test.go）。
func boolPtrVal(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}
