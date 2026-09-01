// Package service 招聘域：职位举报与强制下架（spec #449 T5 #454）。
// 先发后审：学员可举报职位，管理员可带原因强制下架；被强制下架的职位企业不能自行重新上架。
// 举报用招聘域自己的存储（job_reports），不挂到论坛举报表上（那是论坛域的两列形状）。
package service

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// 举报域业务错误哨兵（ADR-0024）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrReportJobNotFound 职位不存在（或已下架）。
	ErrReportJobNotFound = errors.New("职位不存在")
	// ErrReportNotFound 举报记录不存在。
	ErrReportNotFound = errors.New("举报记录不存在")
	// ErrReportAlreadyHandled 举报已处理。
	ErrReportAlreadyHandled = errors.New("该举报已处理")
	// ErrReportReasonRequired 举报原因必填。
	ErrReportReasonRequired = errors.New("举报原因不能为空")
)

// JobReportService 职位举报服务。
type JobReportService struct {
	db     *gorm.DB
	logger *zap.Logger
	mailer MailSender
}

// NewJobReportService 创建职位举报服务。
func NewJobReportService(db *gorm.DB, logger *zap.Logger) *JobReportService {
	return &JobReportService{db: db, logger: logger}
}

// SetMailer 注入邮件发送器（装配根经邮件单点构建后注入）。
func (s *JobReportService) SetMailer(m MailSender) { s.mailer = m }

// ReportListResult 举报队列分页结果。
type ReportListResult struct {
	Items    []ReportDTO `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ReportInput 举报入参。
type ReportInput struct {
	Reason string `json:"reason"`
}

// ReportDTO 举报展示对象。
type ReportDTO struct {
	ID            int64   `json:"id"`
	JobPostingID  int     `json:"job_posting_id"`
	JobTitle      string  `json:"job_title,omitempty"`
	StudentUserID int     `json:"student_user_id"`
	Reason        string  `json:"reason"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	HandledAt     *string `json:"handled_at,omitempty"`
}

// Report 学员举报职位；同一学员对同一职位唯一，重复举报被合并而非堆叠。
func (s *JobReportService) Report(studentUserID, jobPostingID int, reason string) (*ReportDTO, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReportReasonRequired
	}
	var job model.JobPosting
	if err := s.db.First(&job, jobPostingID).Error; err != nil {
		return nil, ErrReportJobNotFound
	}
	// 学员只能举报 open 且未被强制下架的职位
	if job.Status != "open" || job.ForcedOffline {
		return nil, ErrReportJobNotFound
	}
	now := clock.Now()
	// 唯一：已有 pending 或 handled 的举报都合并（重复举报更新原因，不新增行）
	var existing model.JobReport
	err := s.db.Where("job_posting_id = ? AND student_user_id = ?", jobPostingID, studentUserID).First(&existing).Error
	if err == nil {
		// 合并：更新原因与时间（不堆叠）
		if err := s.db.Model(&model.JobReport{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"reason":     reason,
			"updated_at": now,
		}).Error; err != nil {
			return nil, err
		}
		_ = s.db.First(&existing, existing.ID).Error
		dto := s.toDTO(&existing)
		return &dto, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	m := model.JobReport{
		JobPostingID:  jobPostingID,
		StudentUserID: studentUserID,
		Reason:        reason,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.Create(&m).Error; err != nil {
		return nil, err
	}
	dto := s.toDTO(&m)
	return &dto, nil
}

// ListPendingReports 管理端待处理举报队列（分页）。
func (s *JobReportService) ListPendingReports(page, pageSize int) ([]ReportDTO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.JobReport{}).Where("status = ?", "pending").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.JobReport
	if err := s.db.Where("status = ?", "pending").Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	dtos := make([]ReportDTO, 0, len(rows))
	for i := range rows {
		d := s.toDTO(&rows[i])
		var job model.JobPosting
		if err := s.db.First(&job, rows[i].JobPostingID).Error; err == nil {
			d.JobTitle = job.Title
		}
		dtos = append(dtos, d)
	}
	return dtos, total, nil
}

// MarkHandled 管理员把举报标记为已处理。
func (s *JobReportService) MarkHandled(reportID int64) (*ReportDTO, error) {
	var m model.JobReport
	if err := s.db.First(&m, reportID).Error; err != nil {
		return nil, ErrReportNotFound
	}
	if m.Status == "handled" {
		return nil, ErrReportAlreadyHandled
	}
	now := clock.Now()
	if err := s.db.Model(&model.JobReport{}).Where("id = ?", reportID).Updates(map[string]any{
		"status":     "handled",
		"handled_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&m, reportID).Error
	dto := s.toDTO(&m)
	return &dto, nil
}

// ForceOffline 管理员带原因强制下架职位；对学员侧立即不可见（forced_offline=true），
// 企业不能自行重新上架（ToggleStatus 已拦）。处置动作由 handler 记入审计日志。
func (s *JobReportService) ForceOffline(jobPostingID int, reason string) (*JobPostingDTO, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("下架原因不能为空")
	}
	var job model.JobPosting
	if err := s.db.First(&job, jobPostingID).Error; err != nil {
		return nil, ErrReportJobNotFound
	}
	now := clock.Now()
	if err := s.db.Model(&model.JobPosting{}).Where("id = ?", jobPostingID).Updates(map[string]any{
		"forced_offline": true,
		"offline_reason": reason,
		"status":         "closed",
		"updated_at":     now,
	}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&job, jobPostingID).Error
	// 邮件通知企业（被强制下架的原因）
	s.notifyEmployerOffline(job.ID, job.Title, reason)
	// 复用 JobPostingService 的 DTO 转换
	ps := &JobPostingService{db: s.db, logger: s.logger}
	dto := ps.toDTO(&job)
	return &dto, nil
}

// notifyEmployerOffline 邮件通知企业职位被强制下架。
func (s *JobReportService) notifyEmployerOffline(jobPostingID int, jobTitle, reason string) {
	var job model.JobPosting
	if err := s.db.First(&job, jobPostingID).Error; err != nil {
		return
	}
	var rec model.RecruiterUser
	if err := s.db.First(&rec, job.RecruiterID).Error; err != nil {
		return
	}
	if rec.ContactEmail == "" {
		return
	}
	subject := "你的职位已被强制下架"
	body := "你的职位「" + jobTitle + "」因以下原因被平台强制下架：" + reason + "。如需重新发布合规职位，请改正后新发。"
	if s.mailer != nil {
		_ = s.mailer.Send(rec.ContactEmail, subject, body)
	} else if s.logger != nil {
		s.logger.Info("job forced offline (mailer missing, log only)",
			zap.Int("job", jobPostingID), zap.String("reason", reason))
	}
}

func (s *JobReportService) toDTO(m *model.JobReport) ReportDTO {
	dto := ReportDTO{
		ID:            m.ID,
		JobPostingID:  m.JobPostingID,
		StudentUserID: m.StudentUserID,
		Reason:        m.Reason,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
	}
	if m.HandledAt != nil {
		ts := m.HandledAt.Format(time.RFC3339)
		dto.HandledAt = &ts
	}
	return dto
}
