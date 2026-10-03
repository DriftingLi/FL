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
