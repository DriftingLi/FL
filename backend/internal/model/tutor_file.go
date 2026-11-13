package model

import "time"

// 导师个人文件库（#2494）：与 chapter_file 的关系——文件库行是独立实体（归属单一导师），
// 章节挂接/换挂（单挂）由后续票引入；本表只管「传得进、存得住、看得见、删得掉」。
// 上传走四段契约（init/chunk/complete/abort），会话与分片凭据落 upload_session / upload_part。

// TutorFile 导师文件库行。命名沿 `<name>_<毫秒时间戳>.<ext>` 契约（filestore.Save 同源）。
type TutorFile struct {
	FileID      int       `gorm:"column:file_id;primaryKey" json:"file_id"`
	TutorID     int       `gorm:"column:tutor_id" json:"tutor_id"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	ContentType string    `gorm:"column:content_type;default:document" json:"content_type"`
	FileSize    int64     `gorm:"column:file_size;default:0" json:"file_size"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (TutorFile) TableName() string { return "tutor_file" }

// TutorUploadSession 一次分片上传会话（init → … → complete/abort）。
// Status: active / completed / aborted（abort 后 complete 拒绝）。
type TutorUploadSession struct {
	SessionID   string    `gorm:"column:session_id;primaryKey" json:"session_id"`
	TutorID     int       `gorm:"column:tutor_id" json:"tutor_id"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	FileSize    int64     `gorm:"column:file_size" json:"file_size"`
	ChunkSize   int64     `gorm:"column:chunk_size" json:"chunk_size"`
	TotalChunks int       `gorm:"column:total_chunks" json:"total_chunks"`
	ContentType string    `gorm:"column:content_type" json:"content_type"`
	StorageKey  string    `gorm:"column:storage_key" json:"storage_key"`
	UploadID    string    `gorm:"column:upload_id" json:"upload_id"`
	Status      string    `gorm:"column:status;default:active" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (TutorUploadSession) TableName() string { return "tutor_upload_session" }

// TutorUploadPart 已接收分片凭据（序号 + 存储侧 etag + 实际字节数）。
// (session_id, part_number) 主键天然幂等覆盖。
type TutorUploadPart struct {
	SessionID  string    `gorm:"column:session_id;primaryKey" json:"session_id"`
	PartNumber int       `gorm:"column:part_number;primaryKey" json:"part_number"`
	ETag       string    `gorm:"column:etag" json:"etag"`
	Size       int64     `gorm:"column:size" json:"size"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (TutorUploadPart) TableName() string { return "tutor_upload_part" }
