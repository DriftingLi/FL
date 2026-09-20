// Package service 管理端巡检读面（#376 巡检视图 / #1097 归位 service）。
package service

import (
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// deletedAfterAcceptedSettingKey 「删除已解决帖」巡检计数的 system_settings 键。
// 写入方唯一：ForumService.incrementDeletedAfterAccepted（楼主自删已解决帖时 +1）。
const deletedAfterAcceptedSettingKey = "deleted_after_accepted"

// inspectionDefaultPageSize 巡检列表默认页大小（沿用搬迁前 handler 的默认值）。
const inspectionDefaultPageSize = 20

// InspectionService 管理端巡检读面：简历查看记录 / 联系方式交换申请 / 删除已解决帖计数。
//
// 三条读路径原先以闭包注册在 api 层、直持 *gorm.DB（ADR-0056 §3 前 api 层唯一的数据访问面）：
// 这里收 typed 请求 + typed DTO + paging.Query* + 错误上抛，handler 只做参数解析与信封渲染。
type InspectionService struct {
	db *gorm.DB
}

// NewInspectionService 创建巡检读面服务。
func NewInspectionService(db *gorm.DB) *InspectionService {
	return &InspectionService{db: db}
}

// InspectionViewsParams 简历查看记录查询参数（过滤位 <=0 表示不过滤，与搬迁前一致）。
type InspectionViewsParams struct {
	RecruiterID  int
	ResumeUserID int
	Page         int
	PageSize     int
}

// InspectionRequestsParams 联系方式交换申请查询参数（Status 空串表示不过滤）。
type InspectionRequestsParams struct {
	RecruiterID   int
	StudentUserID int
	Status        string
	Page          int
	PageSize      int
}

// RecruitResumeViewDTO 简历查看记录行。
//
// 字段声明序 = model.RecruitResumeView（ADR-0009 §2）：搬出裸 model 后响应字节逐字节不变；
// 加列不再自动进响应（新列出参必须显式改这里）。
type RecruitResumeViewDTO struct {
	ID           int64     `json:"id"`
	RecruiterID  int       `json:"recruiter_id"`
	ResumeUserID int       `json:"resume_user_id"`
	ViewedAt     time.Time `json:"viewed_at"`
}

// ContactRequestRowDTO 联系方式交换申请**行投影**（管理端巡检）。
//
// 字段声明序 = model.ContactRequest（ADR-0009 §2），故与 ContactRequestDTO 是两个类型：
// 后者是展示投影（时间为字符串、带企业信息、多处 omitempty），巡检读面必须逐字节保持
// 裸 model 的既有输出，不能复用。
type ContactRequestRowDTO struct {
	ID            int64     `json:"id"`
	RecruiterID   int       `json:"recruiter_id"`
	StudentUserID int       `json:"student_user_id"`
	Message       string    `json:"message"`
	Status        string    `json:"status"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// DecidedAt 未决申请为 nil：**键整个不出现**（omitempty）→ x-optional；
	// 漏标会让 swag 把它渲染成必填（ADR-0056 §11 / #1100 的契约撒谎面）。
	DecidedAt *time.Time `json:"decided_at,omitempty" extensions:"x-optional"`
	// ExpiresAt 裁决窗口，**仅 pending 有值**（ADR-0061 §2 / 迁移 000039 的 CHECK）。
	// 与 DecidedAt 同形：标量非空会让新产生的 approved 行输出 0001-01-01T00:00:00Z 的假日期。
	ExpiresAt *time.Time `json:"expires_at,omitempty" extensions:"x-optional"`
}

// InspectionCountDTO 巡检单值计数（GET /admin/inspection/deleted-after-accepted）。
// 形状 = 搬迁前 handler 内联的 gin.H{"count": n}。
type InspectionCountDTO struct {
	Count int `json:"count"`
}

// ListRecruitViews 简历查看记录分页（过滤位 >0 才生效；按 viewed_at 倒序）。
// 查询失败一律上抛（ADR-0056 §1）：不再渲染成 200 + 空列表。
func (s *InspectionService) ListRecruitViews(p InspectionViewsParams) (*paging.ItemsPage[RecruitResumeViewDTO], error) {
	rows, total, page, pageSize, err := paging.Query[model.RecruitResumeView](s.db, p.Page, p.PageSize,
		inspectionDefaultPageSize, "viewed_at DESC", func(q *gorm.DB) *gorm.DB {
			if p.RecruiterID > 0 {
				q = q.Where("recruiter_id = ?", p.RecruiterID)
			}
			if p.ResumeUserID > 0 {
				q = q.Where("resume_user_id = ?", p.ResumeUserID)
			}
			return q
		})
	if err != nil {
		return nil, err
	}
	items := make([]RecruitResumeViewDTO, 0, len(rows))
	for i := range rows {
		items = append(items, RecruitResumeViewDTO{
			ID:           rows[i].ID,
			RecruiterID:  rows[i].RecruiterID,
			ResumeUserID: rows[i].ResumeUserID,
			ViewedAt:     rows[i].ViewedAt,
		})
	}
	return &paging.ItemsPage[RecruitResumeViewDTO]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// ListRecruitRequests 联系方式交换申请分页（过滤位 >0 / Status 非空才生效；按 created_at 倒序）。
// 查询失败一律上抛（ADR-0056 §1）。
func (s *InspectionService) ListRecruitRequests(p InspectionRequestsParams) (*paging.ItemsPage[ContactRequestRowDTO], error) {
	rows, total, page, pageSize, err := paging.Query[model.ContactRequest](s.db, p.Page, p.PageSize,
		inspectionDefaultPageSize, "created_at DESC", func(q *gorm.DB) *gorm.DB {
			if p.RecruiterID > 0 {
				q = q.Where("recruiter_id = ?", p.RecruiterID)
			}
			if p.StudentUserID > 0 {
				q = q.Where("student_user_id = ?", p.StudentUserID)
			}
			if p.Status != "" {
				q = q.Where("status = ?", p.Status)
			}
			return q
		})
	if err != nil {
		return nil, err
	}
	items := make([]ContactRequestRowDTO, 0, len(rows))
	for i := range rows {
		items = append(items, ContactRequestRowDTO{
			ID:            rows[i].ID,
			RecruiterID:   rows[i].RecruiterID,
			StudentUserID: rows[i].StudentUserID,
			Message:       rows[i].Message,
			Status:        rows[i].Status,
			Source:        rows[i].Source,
			CreatedAt:     rows[i].CreatedAt,
			UpdatedAt:     rows[i].UpdatedAt,
			DecidedAt:     rows[i].DecidedAt,
			ExpiresAt:     rows[i].ExpiresAt,
		})
	}
	return &paging.ItemsPage[ContactRequestRowDTO]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// DeletedAfterAcceptedCount 「楼主删除自己已解决的帖子」累计计数。
// 计数行缺失（从未发生过）= 0，不是故障；其余读库错误一律上抛（ADR-0056 §1）。
func (s *InspectionService) DeletedAfterAcceptedCount() (*InspectionCountDTO, error) {
	var setting model.SystemSetting
	err := s.db.Where("key = ?", deletedAfterAcceptedSettingKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &InspectionCountDTO{Count: 0}, nil
	}
	if err != nil {
		return nil, err
	}
	v, _ := strconv.Atoi(setting.Value)
	return &InspectionCountDTO{Count: v}, nil
}
