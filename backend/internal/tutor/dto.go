// 讲师域的响应形（P2 波 4d 从 internal/service/residual_types.go 随域包搬来）：
// 它们原本定义在课程域文件里，但归属域是讲师域 —— domains.go 把这两枚登记为 tutor 域 Roots，
// 消费方是本包的 service.go 与 handler.go。
package tutor

// DeleteFileResult 删除章节文件结果（原 map{"file_id","deleted"} 的 typed 形态）。
// 归属讲师域：消费方是 tutor/service.go 与 tutor/handler.go，domains.go 把本枚与下一枚登记为 tutor 域 Roots。
type DeleteFileResult struct {
	FileID  int  `json:"file_id"`
	Deleted bool `json:"deleted"`
}

// BatchDeleteFilesResult 批量删除章节文件结果（同上，tutor 域 Roots）。
type BatchDeleteFilesResult struct {
	SuccessCount int   `json:"success_count"`
	FailedCount  int   `json:"failed_count"`
	FailedIDs    []int `json:"failed_ids" nullability:"nonnil"`
}

// ClaimResult 课程认领/退出认领的动作回执（#2596 B3）。claimed 让前端不必再拉一次列表
// 就能对账乐观更新（认领=true、退出=false）。
type ClaimResult struct {
	CourseID int  `json:"course_id"`
	Claimed  bool `json:"claimed"`
}

// TutorCapabilitiesDTO 当前讲师的有效能力集（#2599 / 规格 F1，形状与 admin.AdminCapabilitiesDTO 一致）。
type TutorCapabilitiesDTO struct {
	// Capabilities 有效能力键；未挂角色时为空数组（**不是 null** —— 契约由
	// apitypes 的可空性锁按 nullability 声明核对）。
	Capabilities []string `json:"capabilities" nullability:"nonnil"`
	// Granted 是否已挂角色。false = 未授权（role_id 为 NULL）或账号不存在。
	// 前端据此区分「还没分配角色」与「角色里什么都没勾」：两者都是空集，但话术不同。
	Granted bool `json:"granted"`
}
