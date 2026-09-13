// Package service 测试：投稿域 DTO shape-lock（spec #952 片一）。
//
// 与 mock_exam_dto_test.go / practice_mode_dto_shape_test.go 同构。它同时是**注解层可空性标注的
// 证据**：x-optional（键可能不存在）与 x-nullable（键在、值为 null）标错时，生成物会与真实响应
// 分头漂移，而前端 type-check 抓不到这类错误——所以把「哪些 key 恒在」钉在这里。
package service

import "testing"

// 投稿条目：author 恒在（omitempty 对结构体取值无效，encoding/json 不省略零值结构体）；
// files / reject_reason 是 omitempty，未装配时**键不存在**（→ 注解层 x-optional）。
func TestContributionItemDTOShapeLock(t *testing.T) {
	summary := ContributionItemDTO{
		ID: 1, CredentialID: 2, Title: "叉车液压手册", Intro: "一线维修整理",
		Status: ContributionStatusApproved, IsAnonymous: false, DownloadsCount: 12,
		CreatedAt: "2026-09-01 10:00:00",
		Author:    ContributionAuthor{UserID: 7},
	}
	assertShapeLock(t, summary,
		"id", "credential_id", "title", "intro", "status", "is_anonymous",
		"downloads_count", "created_at", "author",
	)

	withFiles := summary
	withFiles.Files = []ContributionFileDTO{{FileID: 9, FileName: "a.pdf", FileURL: "/u/a.pdf", FileSize: 1024, ContentType: "document"}}
	withFiles.RejectReason = "资料不合规"
	assertShapeLock(t, withFiles,
		"id", "credential_id", "title", "intro", "status", "is_anonymous",
		"downloads_count", "created_at", "author", "files", "reject_reason",
	)
}

// 投稿文件：暂存上传（UploadFile）尚未落库，file_id 键不存在；详情里的文件来自 DB，键在。
// 两种形态共用同一 DTO，所以 file_id 只能是 x-optional，不能是 x-nullable。
func TestContributionFileDTOShapeLock(t *testing.T) {
	assertShapeLock(t, ContributionFileDTO{FileName: "a.pdf", FileURL: "/u/a.pdf", FileSize: 1024, ContentType: "document"},
		"file_name", "file_url", "file_size", "content_type",
	)
	assertShapeLock(t, ContributionFileDTO{FileID: 9, FileName: "a.pdf", FileURL: "/u/a.pdf", FileSize: 1024, ContentType: "document"},
		"file_id", "file_name", "file_url", "file_size", "content_type",
	)
}

// 分页信封与举报条目：全字段恒在（contribution_title 查不到标题时是空串，不是缺键）。
func TestContributionEnvelopeShapeLock(t *testing.T) {
	assertShapeLock(t, ContributionPageResult{Items: []ContributionItemDTO{}, Total: 0, Page: 1, PageSize: 20},
		"items", "total", "page", "page_size",
	)
	assertShapeLock(t, ContributionReportPageResult{Items: []ContributionReportItemDTO{}, Total: 0, Page: 1, PageSize: 20},
		"items", "total", "page", "page_size",
	)
	assertShapeLock(t, ContributionReportItemDTO{ID: 1, ReporterID: 2, ContributionID: 3, Reason: "piracy", Status: 0, CreatedAt: "2026-09-01 10:00:00"},
		"id", "reporter_id", "contribution_id", "contribution_title", "reason", "status", "created_at",
	)
	assertShapeLock(t, DownloadResult{IsNew: true, TierAwarded: 0}, "is_new", "tier_awarded")
}
