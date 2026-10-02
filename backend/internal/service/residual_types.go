// 留在 internal/service 的 typed 契约（P2 波 3b-1 拆自 course_types.go）：
// 它们定义在课程域文件里，但归属域不是课程域，随课程域搬包会让别域反向依赖课程包。
package service

// GradingStatsDTO 阅卷统计（原 GetGradingStats map 的 typed 形态）。
// 归属阅卷域；全仓当前零生产引用，本波不做「顺手删」（删它要同步 nullability_lock 的债务算式）。
type GradingStatsDTO struct {
	Days       int      `json:"days"`
	Labels     []string `json:"labels" nullability:"nullable"`
	Data       []int64  `json:"data" nullability:"nullable"`
	TotalCount int64    `json:"total_count"`
	ActiveDays int      `json:"active_days"`
}

// DeleteFileResult 删除章节文件结果（原 map{"file_id","deleted"} 的 typed 形态）。
// 归属讲师域：消费方是 tutor_service.go 与 api/tutor.go，domains.go 把本枚与下一枚登记为 tutor 域 Roots。
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
