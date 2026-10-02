// Package material 学习资料域：chapter_file 附件聚合视图（ADR-0018 低成本路径）——
// 资料 = 已发布课程下章节挂载的课件附件，不建独立资料库表。
// 覆盖：列表（可按课程过滤）/ 详情 / 下载地址（file_url 为静态直链）。
// 本包是 internal/<域> 形态的样板之一（ADR-0070）：handler.go 是 HTTP 出口，service.go 是域实现。
package material

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"

	"forklift-training/internal/timefmt"
)

// Service 学习资料服务（chapter_file 聚合视图）。
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService 构造学习资料服务。
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// MaterialDTO 资料条目（附件 + 归属课程/章节回填）。
type MaterialDTO struct {
	FileID       int    `json:"file_id"`
	ChapterID    *int   `json:"chapter_id" extensions:"x-nullable"`
	ChapterTitle string `json:"chapter_title"`
	CourseID     int    `json:"course_id"`
	CourseName   string `json:"course_name"`
	FileName     string `json:"file_name"`
	FileURL      string `json:"file_url"`
	ContentType  string `json:"content_type"`
	FileSize     int64  `json:"file_size"`
	CreatedAt    string `json:"created_at"`
}

// MaterialPageResult 资料分页结果。
type MaterialPageResult struct {
	Page      int           `json:"page"`
	Pages     int           `json:"pages"`
	Total     int64         `json:"total"`
	Materials []MaterialDTO `json:"materials" nullability:"nonnil"`
}

// materialScope 资料可见范围：章节挂载附件 + 归属课程已发布（legacy 无章节附件不含）。
func materialScope(q *gorm.DB, courseID int) *gorm.DB {
	q = q.Table("chapter_file AS cf").
		Select("cf.file_id, cf.chapter_id, cf.file_name, cf.file_url, cf.content_type, cf.file_size, cf.created_at, " +
			"ch.course_id AS course_id_v, c.name AS course_name, COALESCE(ch.title, '') AS chapter_title").
		Joins("JOIN chapter AS ch ON ch.chapter_id = cf.chapter_id").
		Joins("JOIN course AS c ON c.course_id = ch.course_id AND c.status = 1").
		Where("cf.chapter_id IS NOT NULL")
	if courseID > 0 {
		q = q.Where("ch.course_id = ?", courseID)
	}
	return q
}

// materialRow 扫描行（避免列名冲突的别名）。
type materialRow struct {
	FileID       int
	ChapterID    *int
	FileName     string
	FileURL      string
	ContentType  string
	FileSize     int64
	CreatedAt    time.Time
	CourseIDVal  int `gorm:"column:course_id_v"`
	CourseName   string
	ChapterTitle string
}

func materialToDTO(r materialRow) MaterialDTO {
	return MaterialDTO{
		FileID: r.FileID, ChapterID: r.ChapterID, ChapterTitle: r.ChapterTitle,
		CourseID: r.CourseIDVal, CourseName: r.CourseName,
		FileName: r.FileName, FileURL: r.FileURL, ContentType: r.ContentType,
		FileSize: r.FileSize, CreatedAt: timefmt.FormatISO(r.CreatedAt),
	}
}

// ListMaterials 资料列表（courseID 可选过滤，按 file_id 倒序）。
func (s *Service) ListMaterials(page, pageSize, courseID int) (*MaterialPageResult, error) {
	rows, total, page, pageSize, err := paging.QueryWithScan[materialRow](s.db, page, pageSize, 20, 100,
		"cf.file_id DESC",
		func(q *gorm.DB) *gorm.DB {
			return materialScope(q, courseID)
		})
	if err != nil {
		return nil, err
	}
	items := make([]MaterialDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, materialToDTO(r))
	}
	return &MaterialPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Materials: items,
	}, nil
}

// GetMaterial 资料详情。
func (s *Service) GetMaterial(fileID int) (*MaterialDTO, error) {
	var row materialRow
	if err := materialScope(s.db, 0).Where("cf.file_id = ?", fileID).Limit(1).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.FileID == 0 {
		return nil, errors.New("资料不存在")
	}
	dto := materialToDTO(row)
	return &dto, nil
}
