// Package service 实现业务服务层。
// 本文件：题库池 scope（ADR-0050 决策 1）——学员端题目可见性口径的唯一出处。
//
// 口径（CONTEXT.md「题库池」）：已发布（published）+ 排除来源标记标签题（is_source_tag，
// 如真题）+ 当前证件分区。它是学员端题目的可见性口径，覆盖每一条把题目暴露给学员的读路径——
// 作答抽题、搜索结果、按 id 取详情、标签计数一律组合本 module（ADR-0049 约束节）。
//
// interface 三形态同源（缺一不可）：
//   - gorm 链式形态 QuestionPoolScope：抽样、池计数、搜索分区共用（主形态）；
//   - SQL 片段常量 QuestionPoolPublishedSQL / QuestionPoolExcludeSourceTagsSQL：raw SQL 计数
//     直接引用（catalog 的 LEFT JOIN + FILTER 计数改写 gorm 链式反而要绕子查询，更复杂）；
//   - scope 值对象 QuestionReadScope / QuestionEditScope（ADR-0062 决策 4）：面向学员的题目
//     读/写路径的**必传参数**，把「过池」从 caller 的自觉升成类型约束（见文件下方说明）。
//
// 表别名固定为 question：与 gorm 形态的 Model(&model.Question{}) 同源，raw SQL 侧以
// `LEFT JOIN question AS question` 对齐别名即可逐字复用。
package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

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
	// 池的第三个元（证件分区）走归属分区具名谓词（ADR-0056 §2）：nil = 不分区、看全部。
	return EntityOwnedBy(q, QuestionPoolCredentialColumn, cred)
}

// ===== scope 值对象（ADR-0062 决策 4）=====
//
// 「学员读题目必须过池」原本是每条读路径 caller 的自觉（本文件上方那份单点谓词只被三条路径
// 引用，其余各抄一遍或不抄）。决策 4 把它升成**必传参数**：面向学员的题目读/写路径都收
// QuestionReadScope，编辑面（题库作者/审核者）收另一个具名形态 QuestionEditScope ——
// 新读面「忘过池」在类型上过不去（票1b「类型层就是那道锁」同形）。
//
// scope 的内容 = 可见性谓词 + 当前证件分区，两者都由入口（handler 经 CredentialScoped /
// 能力分流）装配，service 侧不再接受裸 *int 证件（ADR-0062 决策 4 的「入口装配」）。
// **权益不进 scope**（决策 3：它是第三个事实，只对付费内容存在，塞进来会让笔记/评论 caller
// 背上空槽；且其事实源住积分域）。
//
// 判据宿主仍是本文件：三种形态（gorm 链式 / SQL 片段 / by-id 判定）都委托上面那份单点谓词，
// 新增形态只是换个出口，不写第二份池。

// QuestionReadScope 学员侧题目读/写 scope：题库池可见性（published + 排源标记真题题）
// 叠加当前证件分区。凡把题目（含题干/选项/答案/解析，以及挂在题上的笔记/评论/收藏快照）
// 暴露给学员、或让学员往题上写东西的路径都必须收它。
type QuestionReadScope struct {
	cred *int
}

// NewQuestionReadScope 由入口装配：cred = CredentialScoped 解析出的当前证件
// （学员未选证件 → nil → 池的第三元不分区、看全部，ADR-0056 §2 归属分区语义）。
func NewQuestionReadScope(cred *int) QuestionReadScope { return QuestionReadScope{cred: cred} }

// Apply 把池口径叠到题目查询上（链式形态；q 的当前表/别名须为 question）。
// 调用方仍可在其上叠加题型/标签等读面差异。
func (s QuestionReadScope) Apply(q *gorm.DB) *gorm.DB { return QuestionPoolScope(q, s.cred) }

// WhereSQL 池口径的 SQL 片段形态（别名固定 question，与三个常量同源）：
// 供 raw 装配点复用——「我的笔记」列表的 `LEFT JOIN question ON ... AND <这里>` 是这类
// 位置的既有消费者，就地重写谓词正是本票要关闭的漂移窗口。
func (s QuestionReadScope) WhereSQL() (string, []any) {
	frag := QuestionPoolPublishedSQL + " AND " + QuestionPoolExcludeSourceTagsSQL
	clause, args := entityOwnedByClause(QuestionPoolCredentialColumn, s.cred)
	if clause == "" {
		return frag, nil
	}
	return frag + " AND " + clause, args
}

// VisibleByID 单题在本 scope 内是否对学员可见：by-id 读面与「往题上写」的挂载校验共用这一处
// （形态照 course_mount_scope.go 的 CourseVisibleByID / ADR-0058；池内三元一条都不少）。
// 查询失败仍按不可见处理（fail-closed 的保守方向不变），但**把 err 交出去**（ADR-0065 决策 7）：
// 「问不出可见性」不是「证明它不可见」，二者对外必须分得开（404 vs 500）。
func (s QuestionReadScope) VisibleByID(db *gorm.DB, questionID int) (bool, error) {
	var cnt int64
	if err := s.Apply(db.Model(&model.Question{})).Where("id = ?", questionID).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// QuestionEditScope 编辑面（题库作者 / 审核者）scope：与学员面的差别是**具名**的——
// 编辑面读 draft / pending、不排源标记真题题（审核队列正是这些东西），证件轴只是
// **列表筛选**（显式 ?credential_id=），不是可见性口径。
//
// 编辑面按 id 的读/写不在此再收 scope：那里的判据宿主是 handler 的能力分流（#981 起，
// `HasCapability(CapQuestionAuthor|CapQuestionReview)` 命中才走编辑支），本类型承载的是
// 列表侧那格筛选轴。
type QuestionEditScope struct {
	credFilter *int
}

// NewQuestionEditScope 由入口装配（编辑面请求的显式 credential_id 查询参数；nil = 不加这格筛选）。
func NewQuestionEditScope(credFilter *int) QuestionEditScope {
	return QuestionEditScope{credFilter: credFilter}
}

// ApplyListFilter 叠上证件筛选轴（归属分区谓词的编辑面用法：nil → 不筛，看全部）。
func (s QuestionEditScope) ApplyListFilter(q *gorm.DB) *gorm.DB {
	return EntityOwnedBy(q, "credential_id", s.credFilter)
}

// questionVisibleOrErr 把 scope 的 by-id 判定翻成错误：读不动 ⇒ 原样上抛（调用方渲染 500），
// 真不可见 ⇒ ErrQuestionNotFound（404）。三处笔记面共用这一格，不各抄一遍两分支。
// questionVisibleOrErr 把 scope 的 by-id 判定翻成错误：读不动 ⇒ 原样上抛（调用方渲染 500），
// 真不可见 ⇒ ErrQuestionNotFound（404）。宿主住在 question_pool_scope.go：它包装的就是本文件的
// VisibleByID，favorite / note / 评论三域共用这一格（favorite_service.go 自述「题目支的判据宿主
// 从此在 question_pool_scope.go」）。
func questionVisibleOrErr(scope QuestionReadScope, db *gorm.DB, questionID int) error {
	visible, err := scope.VisibleByID(db, questionID)
	if err != nil {
		return err
	}
	if !visible {
		return ErrQuestionNotFound
	}
	return nil
}
