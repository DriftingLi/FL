// Package service 实现业务服务层。
// 本文件：题库池 scope（ADR-0050 决策 1）——学员端题目可见性口径的唯一出处。
//
// 口径（CONTEXT.md「题库池」）：已发布（published）+ 排除来源标记标签题（is_source_tag，
// 如真题）+ 当前证件分区。它是学员端题目的可见性口径，覆盖每一条把题目暴露给学员的读路径——
// 作答抽题、搜索结果、按 id 取详情、标签计数一律组合本 module（ADR-0049 约束节）。
//
// interface 双形态同源（缺一不可）：
//   - gorm 链式形态 QuestionPoolScope：抽样、池计数、搜索分区共用（主形态）；
//   - SQL 片段常量 QuestionPoolPublishedSQL / QuestionPoolExcludeSourceTagsSQL：raw SQL 计数
//     直接引用（catalog 的 LEFT JOIN + FILTER 计数改写 gorm 链式反而要绕子查询，更复杂）。
//
// 表别名固定为 question：与 gorm 形态的 Model(&model.Question{}) 同源，raw SQL 侧以
// `LEFT JOIN question AS question` 对齐别名即可逐字复用。
package service

import "gorm.io/gorm"

// 题库池谓词的两个 SQL 片段（表别名固定 question）。raw SQL 计数必须引用它们，
// 不得就地重写——「raw 重写与常量脱钩」正是本 module 要关闭的漂移窗口。
const (
	// QuestionPoolPublishedSQL 题库池的「已发布」谓词。
	QuestionPoolPublishedSQL = "question.status = 'published'"
	// QuestionPoolExcludeSourceTagsSQL 排除来源标记标签（is_source_tag，如真题）题目的公共过滤片段：
	// 这类题目只能经真题卷作答，不进顺序/随机/专项练习与模拟考抽题池（ADR-0022）。
	QuestionPoolExcludeSourceTagsSQL = "NOT EXISTS (SELECT 1 FROM question_tag_relation qtr JOIN question_tag qt ON qt.id = qtr.tag_id WHERE qtr.question_id = question.id AND qt.is_source_tag)"
	// QuestionPoolCredentialColumn 题库池的证件分区列：raw 计数按方言追加占位符时引用同一列名，
	// 池的第三个元（当前证件分区）也就只有这一处出处。
	QuestionPoolCredentialColumn = "question.credential_id"
)

// QuestionPoolScope 题库池 scope（gorm 链式形态）：已发布 + 排除来源标记标签题 + 证件分区。
// cred 为 nil 时不作证件分区（全局池）；是否传证件由读面语义决定（学员读面传当前证件）。
// 调用方可继续叠加题型/标签等读面差异。
func QuestionPoolScope(q *gorm.DB, cred *int) *gorm.DB {
	q = q.Where(QuestionPoolPublishedSQL).Where(QuestionPoolExcludeSourceTagsSQL)
	if cred != nil {
		q = q.Where(QuestionPoolCredentialColumn+" = ?", *cred)
	}
	return q
}
