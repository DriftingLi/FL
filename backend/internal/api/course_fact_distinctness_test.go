// ADR-0065 决策 3·9 的锁：课程两条写面点名的这些事实必须**两两不可替换**，且同文案只允许
// 出现「已登记的那一对」。
//
// 为什么单独一把：第③批的教训是把哨兵拆成 `A = B` 别名时，编译通过、404 映射测试全绿，
// 而 errors.Is 把几态认成同一件 ⇒ 验「对外码一致」的测试对「内部拆分」类改动是无效验证。
// 本批一次新增 12 枚哨兵，其中「所属证件不存在」与题库域逐字同文案而**有意不合并**
// （合并会连带动那条端点的默认码）⇒ 需要一把锁同时钉住「不许别名回去」与「这对是同文案但不同事实」。
package api

import (
	"fmt"
	"testing"

	"forklift-training/internal/service"
)

// courseWaveFacts 本批在课程写面上点名的全部事实。
var courseWaveFacts = []error{
	service.ErrCourseNameRequired, service.ErrSpecialtyRequired, service.ErrCourseLevelRequired,
	service.ErrCourseCredentialIDInvalid, service.ErrCourseSpecialtyIDInvalid,
	service.ErrCourseLevelIDInvalid, service.ErrCertificateTemplateIDInvalid,
	service.ErrCourseCredentialRefNotFound, service.ErrSpecialtyNotFound,
	service.ErrCourseLevelNotFound, service.ErrCertificateTemplateNotFound,
	service.ErrCourseTheoryHoursNegative, service.ErrCoursePracticeHoursNegative,
	service.ErrCourseSortOrderNegative,
	service.ErrCoursePrerequisiteSelf, service.ErrCoursePrerequisiteNotFound,
	service.ErrCoursePrerequisiteCycle, service.ErrCourseNotFound,
	service.ErrEntityNotSortable, service.ErrSwapItemNotFound,
	service.ErrCourseNotMountedForSort, service.ErrCourseSortGroupMismatch,
	service.ErrCourseSwapTargetNotFound,
}

// allowedSameText 是**登记过**的同文案不同事实对（ADR-0065 决策 9）。除此之外两两不得同句。
var allowedSameText = [][2]string{
	{"所属证件不存在", "所属证件不存在"}, // 课程写面 ↔ 题库域，见下条断言的理由
}

func TestCourseWriteFactsArePairwiseDistinct(t *testing.T) {
	seen := map[string]error{}
	for i, a := range courseWaveFacts {
		if prev, dup := seen[a.Error()]; dup {
			t.Fatalf("%q 出现了两次（%v 与 %v）⇒ 同一句被两个载体说，或有人把两件事压成了一件；"+
				"如属有意分裂，请登记进 allowedSameText 并写明理由", a.Error(), prev, a)
		}
		seen[a.Error()] = a
		for j, b := range courseWaveFacts {
			if j <= i {
				continue
			}
			if a == b {
				t.Fatalf("%q 与 %q 是同一个 error 值（别名/复用）⇒ errors.Is 会把两件事实认成一件，"+
					"档位分档在运行期不会发生", a, b)
			}
		}
	}
	// 有意分裂的那对必须仍然同句且不同值：只钉「不许合并」而不钉「句子别乱改」，
	// 会让下一个人以为改文案是免费的（文案一致是决策 5 的「同措辞」半边）。
	q := fmt.Sprint(service.ErrQuestionCredentialNotFound)
	c := fmt.Sprint(service.ErrCourseCredentialRefNotFound)
	if q != c || service.ErrQuestionCredentialNotFound == service.ErrCourseCredentialRefNotFound {
		t.Fatalf("证件那对同文案双载体现状变了（question=%q course=%q）⇒ 要么被合并了（决策 9 否掉），"+
			"要么句子分家了（决策 5 的「同措辞」半边）", q, c)
	}
	if len(allowedSameText) != 1 {
		t.Fatalf("同文案豁免表被改宽（%d 条）", len(allowedSameText))
	}
}
