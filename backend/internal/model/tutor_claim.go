package model

import "time"

// 课程认领（#2596 / 规格 B1）：讲师与课程的多对多关系——一门课可由多位讲师共认分工，
// 一位讲师也可认领多门课。**不是归属**：没有「课程的主人」这回事，只有「谁在编这门课」。
//
// 与 chapter.tutor_id 的分工（ADR-0095 决策二）：那格记的是「章节作者」，只在首次写入时
// 落定、认领退出不回溯；本表记的是「当前编辑权」，退出即刻生效。两件事分开存，故退出认领
// 之后历史作者归属照旧。
//
// 编辑权由域谓词 RequireChapterClaim 单点表达（规格 B2）：能进讲师面 ≠ 能改这门课的章节。

// 认领来源（Source 列取值域）。
const (
	// ClaimSourceSelf 讲师自助认领。
	ClaimSourceSelf = "self"
	// ClaimSourceAdmin 管理员代分配（供审计区分「自己要干的活」与「被派下来的活」）。
	ClaimSourceAdmin = "admin"
)

// TutorCourseClaim 讲师对一门课程的认领行。(TutorID, CourseID) 唯一。
type TutorCourseClaim struct {
	ClaimID   int       `gorm:"column:claim_id;primaryKey" json:"claim_id"`
	TutorID   int       `gorm:"column:tutor_id" json:"tutor_id"`
	CourseID  int       `gorm:"column:course_id" json:"course_id"`
	Source    string    `gorm:"column:source;default:self" json:"source"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (TutorCourseClaim) TableName() string { return "tutor_course_claim" }
