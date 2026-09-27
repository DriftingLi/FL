// 统计 / 学习进度 / 错题 / 练习 / 模考域的 nonnil 行为例（批①-A）。
//
// 为什么单独成文件：判据 5 的证据要跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
// 本域的出口形状特殊：stats_aggregate.go 一个聚合函数同时产出多格，故**一次调用举证多条键**
// （各占一键、各自 marshal，断言一条没少）。
//
// 留在 `nullable` 的那半怎么说：本文件的表只装**跑过且发出非 null** 的出口；没进表的名字
// 就是「没跑过」或「跑出来就是 null」，两者的清单由锁自己印，不在这里抄一份会烂掉的名单——
//
//	go test ./internal/apitypes/ -run TestResponseCollectionsMustDeclareNullability -v | grep 判据.4.待举证
//
// 这里只记**判据跑不出来、只能靠读代码定性**的那几类，因为它们是下一批的分工依据：
//   - 实测真发 null（批①-B 的补 x-nullable 射程）：MockExamSubmitDTO.details（同一条声明经
//     GetResult 出口在未交卷时发 null，而 GetResult 照样 200）、GenTaskStatus.results（pending
//     阶段 result 列还没写过）、ProgressResultDTO.answers_state、AIChatMessageDTO.images/.sources、
//     QuestionTagsResultDTO.tag_ids、DiagnosisFaultCodePage.items（上游 JSON 直 decode，无 make 兜底）。
//     ContactPlainDTO.* 与 JobCardDTO.* 那几格是 model.JSONArray，nil 直接 marshal 成 null。
//   - **非 null 来自列默认值而不是代码**：JSONArray 那几格若投影里只加了一道 `len(x)==0 → "[]"`，
//     列中存着 4 字节 JSON null 时照样透传出去。判据只跑一次成功出口，分不出这种脏列——所以
//     这类字段改判前必须先读投影（recruit_service.go 的脱敏卡三种形状各给了注释）。
//   - 结构性够不着举证：repository.* 那 7 格——判据 5 的证据源只扫 ../service 与 ../api，
//     而 DictionaryRepository 只握 *pgxpool.Pool，没有可脱离真库跑的出口。这不是没人去举证，
//     是举证装置照不到；下一波要么给这两个包加证据源，要么把它们从判据 4 的分母里显式移出。
//   - 键缺席而非 null：*[]T + omitempty 的那几格（CourseDTO.chapters / prerequisites /
//     prerequisite_course_ids、ContributionItemDTO.files）。诚实形状是 nonnil + 既有 x-optional，
//     marshalKey 在键缺席时判红，所以它们必须走一条**填得上**的出口才算举证。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nonnilOutletsStats = map[string]func(t *testing.T) any{
	"service.AdminStatisticsDTO.course_stats":         outletAdminStatisticsNoCourses,
	"service.WrongQuestionPageDTO.items":              outletWrongQuestionPageEmpty,
	"service.MockExamResumeDTO.questions":             outletMockExamResume,
	"service.GroupedCredentialsDTO.skill_level":       outletGroupedCredentialsNone,
	"service.GroupedCredentialsDTO.special_operation": outletGroupedCredentialsNone,
	"service.StudentProfileDTO.course_progress":       outletStudentProfileNoStudy,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsStats)
}

// outletAdminStatisticsNoCourses 管理端统计看板：无课程时 course_stats 由 make([]CourseStatDTO,0,n) 起手。
func outletAdminStatisticsNoCourses(t *testing.T) any {
	t.Helper()
	return NewAdminService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetStatistics()
}

// outletWrongQuestionPageEmpty 错题本分页：一行错题都没有时 items 仍是 make 出来的空集。
func outletWrongQuestionPageEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewWrongQuestionService(testutil.NewMemoryDB(t), nil, zap.NewNop()).
		GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("空错题本分页失败: %v", err)
	}
	return res
}

// outletMockExamResume 断点续考：卷面存的题号指向**已被删除的题目**时 loadOrderedQuestions
// 取不到任何一行，questions 走 make(0,0) 而不是 nil —— 这条正是「题被下架后学员还在考试中途」的线上形状。
func outletMockExamResume(t *testing.T) any {
	t.Helper()
	svc, db, mockExamID, studentID := seedMockInProgress(t)
	if err := db.Model(&model.MockExam{}).Where("id = ?", mockExamID).
		Update("question_ids", model.JSONB("[999999]")).Error; err != nil {
		t.Fatalf("改写考卷题号失败: %v", err)
	}
	res, err := svc.Resume(mockExamID, studentID)
	if err != nil {
		t.Fatalf("续考失败: %v", err)
	}
	return res
}

// outletGroupedCredentialsNone 证件分组：两组都以 []CredentialDict{} 起手，空集也发 `[]`。
func outletGroupedCredentialsNone(t *testing.T) any {
	t.Helper()
	return NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop()).ListGroupedCredentials()
}

// outletStudentProfileNoStudy 学员档案：有账号、零学习记录时 course_progress 是空集。
func outletStudentProfileNoStudy(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "档案学员", "x")
	res, err := NewStudentService(db, zap.NewNop()).GetProfile(student.ID)
	if err != nil {
		t.Fatalf("取档案失败: %v", err)
	}
	return res
}
