package tutorfile

import (
	"time"

	"forklift-training/internal/model"
)

// InitUploadRequest 创建上传会话请求（spec A1）。
type InitUploadRequest struct {
	FileName  string `json:"file_name" binding:"required"`
	FileSize  int64  `json:"file_size" binding:"required"`
	ChunkSize int64  `json:"chunk_size" binding:"required"`
}

// InitUploadResponse 创建上传会话响应：客户端凭 session_id 逐片 PUT。
type InitUploadResponse struct {
	SessionID   string `json:"session_id"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
}

// UploadChunkResponse 单片接收回执。
type UploadChunkResponse struct {
	PartNumber int   `json:"part_number"`
	Size       int64 `json:"size"`
}

// FileDTO 文件库行（列表与 complete 共用形状）。
// ChapterID/ChapterTitle 只在挂接中时出现（spec B2/B7：挂接状态是管理面的一等信息）。
type FileDTO struct {
	FileID       int       `json:"file_id"`
	FileURL      string    `json:"file_url"`
	FileName     string    `json:"file_name"`
	ContentType  string    `json:"content_type"`
	FileSize     int64     `json:"file_size"`
	ChapterID    *int      `json:"chapter_id,omitempty"`
	ChapterTitle string    `json:"chapter_title,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// FileListDTO 文件库列表。
type FileListDTO struct {
	Files []FileDTO `json:"files"`
}

// SessionStatusDTO 断点续传清单（spec A8，= S3 ListParts 语义）：
// 客户端凭 session_id 查询会话状态与已收分片序号，只补缺片。
type SessionStatusDTO struct {
	SessionID     string `json:"session_id"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	ChunkSize     int64  `json:"chunk_size"`
	TotalChunks   int    `json:"total_chunks"`
	Status        string `json:"status"`
	ReceivedParts []int  `json:"received_parts"`
}

// AttachRequest 单挂挂接请求（spec B2）。
type AttachRequest struct {
	ChapterID int `json:"chapter_id" binding:"required"`
}

// RenameRequest 改名请求（spec B7）。
type RenameRequest struct {
	FileName string `json:"file_name" binding:"required"`
}

// SwapRequest 章节侧换挂请求（spec B6：挂新 + 旧回库）。
type SwapRequest struct {
	OldFileID int `json:"old_file_id" binding:"required"`
	NewFileID int `json:"new_file_id" binding:"required"`
}

// toFileDTO 模型 → 传输形状。titles 为章节标题缓存（ChapterTitles 一次查询），可为 nil。
func toFileDTO(f model.TutorFile, titles map[int]string) FileDTO {
	d := FileDTO{
		FileID:      f.FileID,
		FileURL:     f.FileURL,
		FileName:    f.FileName,
		ContentType: f.ContentType,
		FileSize:    f.FileSize,
		ChapterID:   f.ChapterID,
		CreatedAt:   f.CreatedAt,
	}
	if f.ChapterID != nil && titles != nil {
		d.ChapterTitle = titles[*f.ChapterID]
	}
	return d
}

// toSessionStatusDTO 会话行 + 已收分片 → 断点续传清单传输形状。
func toSessionStatusDTO(row *model.TutorUploadSession, received []int) SessionStatusDTO {
	return SessionStatusDTO{
		SessionID:     row.SessionID,
		FileName:      row.FileName,
		FileSize:      row.FileSize,
		ChunkSize:     row.ChunkSize,
		TotalChunks:   row.TotalChunks,
		Status:        row.Status,
		ReceivedParts: received,
	}
}
