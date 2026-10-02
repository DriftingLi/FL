// 目录树 / 字典列表 / 章节文件 / 精选 / 模考 / 学员侧读面的 nonnil 行为例
// （批①-A 第④段「wave 16」）。
//
// 为什么单独成文件：判据 5 的证据跟着出口走，一个域一张表（汇总表机制见 nonnil_declaration_test.go）。
// 本段跨的域比 stats/course 两段更杂，共同点只有一件：这些字段全部由 `make([]T, 0, n)` 或
// `make(map, n)` 起手、只 append，从来没有一条成功出口发得出 `null`。
//
// 下面是**逐字段跑过真实出口后留在 `nullable` 的那半**（清单是按域扫出来的，判据是跑出来的）：
//   - AIChatMessageDTO.images / .sources ⇒ GetSessionMessages 里 `var imgs []string` 只在
//     列非空串时才 Unmarshal，「没图 / 没来源」就是 nil ⇒ `null`。那句 `nullable` 是真的
//     （已带 x-nullable，属批①-B 的消费端加宽，不是改判对象）。
//   - DiagnosisFaultCodePage.items ⇒ ListFaultCodes 把上游助手的 JSON 直接 Decode 进结构体，
//     没有任何 `make` 兜底 ⇒ 上游发 `{"items":null}` 或省略该键时这里就是 `null`。
//     出口形状由对端进程决定，本仓改判不了它。
//   - GenTaskStatus.results ⇒ pending 阶段 async_task.result 列还没写过，GetTaskStatus 只在
//     `len(task.Result) > 0` 时才赋值 ⇒ 实测发 `null`（已带 x-nullable）。
//   - MockExamSubmitDTO.details ⇒ Submit 那一侧是 `make(0,n)` 恒非 null，但同一条声明经
//     GetResult（未交卷记录 result 列为空）发 `null`，且 MockExamResultDTO 内嵌本 DTO 共用它。
//   - ProgressResultDTO.answers_state ⇒ 该键的证据已随域包搬去
//     internal/practicemode/nullable_outlets_test.go（ADR-0070 波 3c-2），判词原文随之搬走。
//   - QuestionTagsResultDTO.tag_ids ⇒ 该键的证据已随域包搬去
//     internal/training/nullable_outlets_test.go（ADR-0070 波 3b-2），判词原文随之搬走，不在这里复述。
//
// 本段**没跑**因而也没改判的三组，原因各不相同（都不是「嫌麻烦」）：
//   - ContactPlainDTO.photos / .resume_certifications、QuestionCommentPageResult.items（键前缀随 4c 改 questioninteraction.）、
//     api.AuditLogPageResult.items ⇒ 宿主文件由另一条在飞的分支持有（contact_service.go、
//     internal/questioninteraction/service.go、internal/api/），本段不改。前两格另有独立理由：
//     service.JSONArray.MarshalJSON 在 `j == nil` 时**字面发出 `null`**，本就恒可空。
//   - repository.AlgorithmParameters 的 4 格与 repository.SeriesConfigOptions 的 3 格 ⇒ 两道
//     硬阻塞，任一都足以让它留在原地：
//     (1) 判据 5 的证据源只有 `../service` 与 `../api` 两个目录（见 nullability_lock_test.go
//     的 nonNilEvidenceSources），repository 包**不在扫描面内** ⇒ 在
//     internal/valuation/repository/ 下建一张 nonnilOutlets* 表，锁一条也读不到，
//     改判后判据 5 直接判红；
//     (2) 这些出口要真 Postgres：DictionaryRepository 只持 `*pgxpool.Pool`（无 gorm、无
//     sqlite 构造口），listCached 走 pgx.Rows ⇒ testutil.NewMemoryDB 那套内存库进不去，
//     行为例就成了「只在配了 DATABASE_URL 的机器上才跑」——判据 5 要的是一次真跑过。
//     ⇒ 正解是给 nonNilEvidenceSources 加一个 repository 来源 + 给该包一条可跑 PG 的 seam，
//     那是独立一件工具改动，不塞进本段。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/questionbank"
	"forklift-training/internal/testutil"
)

var nonnilOutletsCatalog = map[string]func(t *testing.T) any{
	// 培训域的 7 格（目录树两层 + 五个字典列表信封）已随域包搬去
	// internal/training/nonnil_outlets_test.go（ADR-0070 波 3b-2）；岗位字典与证件分组两处同批搬走。

	// 课程域的课程 DTO 字段已随域包搬去 internal/course/nonnil_outlets_test.go；
	// 本包仍留 course.ChapterDTO.files 与 course.CourseDTO.chapters 两键 —— 见 nonnil_outlets_course_test.go。

	// 精选域的 items / related 举证已随域包搬去 internal/featured/nonnil_outlets_test.go（ADR-0070）。

	"service.BatchDeleteFilesResult.failed_ids": outletBatchDeleteFilesEmpty,

	// 练习域的三格（HistoryResultDTO.records / PracticeStartResultDTO.questions / PracticeStatsDTO.by_type）
	// 已随域包搬去 internal/practicemode/nonnil_outlets_test.go（ADR-0070 波 3c-2）。

	// 模考域三格（MockExamHistoryDTO.exams / MockExamStartDTO.questions / MockExamResumeDTO.questions）
	// 已随域包搬去 internal/mockexam/nonnil_outlets_test.go（ADR-0070 波 4a）。
	"questionbank.QuestionBankStatsDTO.by_type":   outletQuestionBankStatsEmpty,
	"questionbank.QuestionBankStatsDTO.by_status": outletQuestionBankStatsEmpty,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsCatalog)
}

// ===== 批量删文件 =====

func outletBatchDeleteFilesEmpty(t *testing.T) any {
	t.Helper()
	svc := newTutorServiceForTest(t, testutil.NewMemoryDB(t))
	return svc.BatchDeleteChapterFiles(nil)
}

// ===== 题目域统计 =====
// （错题本那格已随域包搬去 internal/wrongquestion/nonnil_outlets_test.go，波 4c）

// outletQuestionBankStatsEmpty 一次调用举证 by_type 与 by_status 两格：两个 map 都取自
// questionbank.GroupByCount 的 make(map,...) 再按合法维度零填充 ⇒ 恒 `{}` 起、只会更满。
func outletQuestionBankStatsEmpty(t *testing.T) any {
	t.Helper()
	return questionbank.NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetStats(nil)
}

// 学员侧四格（StudentCoursesDTO.courses / StudentCourseDetailDTO.chapters / StudyRecordPageResult.records /
// StudyDailyStatsDTO.labels / StudyDailyStatsDTO.data / StudentProfileDTO.course_progress）
// 已随域包搬去 internal/student/nonnil_outlets_test.go（ADR-0070 波 4b）。
