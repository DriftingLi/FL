// Package questionpool 题库池可见性口径的 SQL 片段（ADR-0050 决策 1）。
//
// 这三枚片段是题库池「已发布 + 排除来源标记标签题（is_source_tag）+ 当前证件分区」在
// **raw SQL** 侧的唯一定义处：raw SQL 计数必须引用它们，不得就地重写——「raw 重写与常量
// 脱钩」正是 ADR-0050 决策 1 要关闭的漂移窗口。表别名固定为 question，与 gorm 形态的
// Model(&model.Question{}) 同源，raw SQL 侧以 `LEFT JOIN question AS question` 对齐别名
// 即可逐字复用。
//
// **为什么是叶子包（ADR-0070 第三种破法：无状态纯片段进叶子包）**：两个域用同一份判据——
// 留驻 internal/service 的题库池 scope（internal/service/question_pool_scope.go：gorm 链式
// 形态 QuestionPoolScope 与 QuestionReadScope / QuestionEditScope 值对象）与
// internal/training 的标签题目计数（GET /api/tags 的 published 计数，见
// internal/training/catalog_service.go）。载体留任一域包都会让另一域反向依赖它，而域包不得
// import internal/service（3b-2 起 internal/service 反向 import internal/training）。
//
// 池的另外两形态（gorm 链式 / scope 值对象）暂留 internal/service/question_pool_scope.go，
// 待题目域收口时同迁。
package questionpool

const (
	// PublishedSQL 题库池的「已发布」谓词。
	PublishedSQL = "question.status = 'published'"
	// ExcludeSourceTagsSQL 排除来源标记标签（is_source_tag，如真题）题目的公共过滤片段：
	// 这类题目只能经真题卷作答，不进顺序/随机/专项练习与模拟考抽题池（ADR-0022）。
	ExcludeSourceTagsSQL = "NOT EXISTS (SELECT 1 FROM question_tag_relation qtr JOIN question_tag qt ON qt.id = qtr.tag_id WHERE qtr.question_id = question.id AND qt.is_source_tag)"
	// CredentialColumn 题库池的证件分区列：raw 计数按方言追加占位符时引用同一列名，
	// 池的第三个元（当前证件分区）也就只有这一处出处。
	CredentialColumn = "question.credential_id"
)
