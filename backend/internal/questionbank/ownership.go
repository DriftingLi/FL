// 本文件：题库写隔离的域谓词（#2598 / 规格块 C）——「只能动自己创建的题」的唯一判据出处。
//
// 为什么它是域谓词而不是能力键：谁能写题（`question.author`）回答的是**资格**，
// 「这道题是不是你的」回答的是**这一行的归属**。后者随数据变化，能力表按角色静态回答不了
// 它（ADR-0047 §1 已把数据级不变式划在能力表之外）。
//
// **与课程认领谓词互不引用**（规格硬约束 3）：题目归属只看 question.created_by，
// 不看任何认领行；course.RequireCourseClaim 也不看题目归属。两套判据服务于两件事——
// 讲师认领一门课是为了写它的章节，不代表他能改别人写的题；反之亦然。
// 把它们「顺手统一」成一句「有没有认领」会让两个领域的授权事实互相污染。
package questionbank

import (
	"errors"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// ErrQuestionNotOwned 题目不属于当前操作者（映射 403）。
//
// 不用 404 伪装（规格 C3）：题库编辑面本来就全量可读（C4），题目的存在性不是秘密，
// 404 在这里没有信息保护收益，只会让前端分不清「题没了」与「题不是你的」。
var ErrQuestionNotOwned = errors.New("只能编辑自己创建的题目")

// RequireQuestionOwnership 域谓词：要求当前操作者拥有这道题（规格 C1）。
//
// 判据三条：
//   - 管理员直通（管理员即审核者，且管理端要能捡起任人的题）；
//   - 否则 created_by 必须等于操作者 id；
//   - created_by 为 NULL 的存量题**只有管理员能写**（规格 C2）——空值不是「人人有份」，
//     而是「作者未知」，把它判给任何一位讲师都等于凭空授权。
//
// actorID <= 0 一律拒（fail closed）：`middleware.CurrentUserID` 在 gin ctx 里取不到 uid 时
// 返回 0，而 `created_by = 0` 在理论上是可写进库的值（建题时 uid 取失败会传 &0）。
// 「取不到身份」绝不能退化成「匹配上了某个创建者」——那会把一次鉴权故障变成一次越权。
//
// 入参是已载入的题行：三条写路径都要先读行（判断状态、算内容差异），谓词不重复查库。
func RequireQuestionOwnership(q *model.Question, actorID int, actorType string) error {
	if actorType == string(authz.RoleAdmin) {
		return nil
	}
	if actorID <= 0 {
		return ErrQuestionNotOwned
	}
	if q.CreatedBy == nil || *q.CreatedBy != actorID {
		return ErrQuestionNotOwned
	}
	return nil
}
