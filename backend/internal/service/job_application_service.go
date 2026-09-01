// Package service 招聘域：投递即授权（spec #449 T3 #452）。
// 核心不变式：投递在学员点下那一刻，在同一事务内写入投递记录 + 写入/复活一条 approved 的
// 联系方式交换授权（source=application），明文的载体仍然只有联系方式交换一个——GetContact
// 与授权撤回的实现一行都不改（这是验收点，不是巧合）。
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// 投递域业务错误哨兵（ADR-0024）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrApplyNotFound 投递记录不存在。
	ErrApplyNotFound = errors.New("投递记录不存在")
	// ErrApplyNotYours 不能操作他人的投递。
	ErrApplyNotYours = errors.New("无权操作该投递")
	// ErrApplyJobInactive 职位已下架或强制下架，不能投递。
	ErrApplyJobInactive = errors.New("该职位当前不可投递")
	// ErrApplyDuplicated 同一学员对同一职位在 applied 期间只能有一条。
	ErrApplyDuplicated = errors.New("你已投递过该职位，请勿重复投递")
	// ErrApplyCooldown 被企业标记不合适后 30 天内不能再次投递。
	ErrApplyCooldown = errors.New("该职位 30 天内暂不能再次投递")
	// ErrApplyDailyLimit 学员每日投递上限（默认 10）。
	ErrApplyDailyLimit = errors.New("今日投递已达上限（10 个）")
	// ErrApplyResumeIncomplete 缺真实姓名或缺联系电话的简历不能投递（否则企业收到空简历）。
	ErrApplyResumeIncomplete = errors.New("简历缺少真实姓名或联系电话，无法投递")
)

// JobApplicationService 投递服务。
type JobApplicationService struct {
	db              *gorm.DB
	logger          *zap.Logger
	notificationSvc *NotificationService
	mailer          MailSender
	dailyLimit      int
}

// NewJobApplicationService 创建投递服务。mailer 可为 nil（测试或未配置时降级为日志）。
func NewJobApplicationService(db *gorm.DB, logger *zap.Logger, notificationSvc *NotificationService) *JobApplicationService {
	return &JobApplicationService{db: db, logger: logger, notificationSvc: notificationSvc, dailyLimit: 10}
}

// SetDailyLimit 测试用：覆盖每日投递上限。
func (s *JobApplicationService) SetDailyLimit(n int) { s.dailyLimit = n }

// SetMailer 注入邮件发送器（装配根经邮件单点构建后注入）。
func (s *JobApplicationService) SetMailer(m MailSender) { s.mailer = m }

// ApplicationDTO 投递展示对象。
type ApplicationDTO struct {
	ID              int64   `json:"id"`
	JobPostingID    int     `json:"job_posting_id"`
	JobTitle        string  `json:"job_title,omitempty"`
	RecruiterID     int     `json:"recruiter_id"`
	StudentUserID   int     `json:"student_user_id"`
	Status          string  `json:"status"`
	ResumeUpdatedAt string  `json:"resume_updated_at"`
	EmployerViewAt  *string `json:"employer_viewed_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	// 企业信息（学员侧可见）
	CompanyName string `json:"company_name,omitempty"`
	// 学员信息（企业侧可见，走脱敏路径）
	StudentRealNameMasked  string `json:"student_real_name_masked,omitempty"`
	StudentResumeUpdatedAt string `json:"student_resume_updated_at,omitempty"`
	// 投递那一刻的简历更新时间（版本指针），前端据此提示「收到后简历已更新」
	ResumeUpdatedAtSnapshot string `json:"resume_updated_at_snapshot,omitempty"`
}

// ApplicationListResult 我的投递分页结果。
type ApplicationListResult struct {
	Items    []ApplicationDTO `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// toDTO 转换 DB 行为 DTO。
func (s *JobApplicationService) toDTO(m *model.JobApplication) ApplicationDTO {
	dto := ApplicationDTO{
		ID:              m.ID,
		JobPostingID:    m.JobPostingID,
		RecruiterID:     m.RecruiterID,
		StudentUserID:   m.StudentUserID,
		Status:          m.Status,
		ResumeUpdatedAt: m.ResumeUpdatedAt.Format(time.RFC3339),
		CreatedAt:       m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       m.UpdatedAt.Format(time.RFC3339),
	}
	if m.EmployerViewAt != nil {
		ts := m.EmployerViewAt.Format(time.RFC3339)
		dto.EmployerViewAt = &ts
	}
	var rec model.RecruiterUser
	if err := s.db.First(&rec, m.RecruiterID).Error; err == nil {
		dto.CompanyName = rec.CompanyName
	}
	return dto
}

// applyMessage 投递产生的授权附言：系统固定句式（含职位名），保证审计可读。
func applyMessage(jobTitle string) string {
	return fmt.Sprintf("学员投递职位「%s」产生的联系方式授权", jobTitle)
}

// Apply 学员投递职位（投递即授权）。
// 同一事务内：
//  1. 写入投递记录（applied）；
//  2. 写入/复活一条 approved 的联系方式交换授权（source=application）；
//     该企业对该学员已有 pending 申请时，把它覆盖为 approved（学员主动投递优先于待决申请）；
//  3. 投递产生的授权不计入企业日限（那是防企业骚扰的）。
func (s *JobApplicationService) Apply(studentUserID, jobPostingID int) (*ApplicationDTO, error) {
	var job model.JobPosting
	if err := s.db.First(&job, jobPostingID).Error; err != nil {
		return nil, ErrApplyJobInactive
	}
	if job.Status != "open" || job.ForcedOffline {
		return nil, ErrApplyJobInactive
	}
	// 学员是否存在且未注销
	var stu model.HrwaiUser
	if err := s.db.First(&stu, studentUserID).Error; err != nil {
		return nil, ErrStudentGone
	}
	// 简历完整性：缺真实姓名或缺联系电话 → 拒（否则企业收到空简历）
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", studentUserID).Error; err != nil {
		return nil, ErrApplyResumeIncomplete
	}
	if strings.TrimSpace(card.RealName) == "" || strings.TrimSpace(card.ContactPhone) == "" {
		return nil, ErrApplyResumeIncomplete
	}
	// applied 期间唯一（业务层判定；部分唯一索引在生产由迁移保证，sqlite 契约测试靠本判定）
	var dup int64
	if err := s.db.Model(&model.JobApplication{}).Where("job_posting_id = ? AND student_user_id = ? AND status = ?", jobPostingID, studentUserID, "applied").Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, ErrApplyDuplicated
	}
	// 被拒绝后 30 天冷却（复用联系方式交换的数值与「拒绝是有记忆的」哲学）
	var lastRejected *model.JobApplication
	var last model.JobApplication
	if err := s.db.Where("job_posting_id = ? AND student_user_id = ? AND status = ?", jobPostingID, studentUserID, "rejected").Order("rejected_at DESC").First(&last).Error; err == nil {
		lastRejected = &last
		if last.RejectedAt != nil && clock.Now().Sub(*last.RejectedAt) < 30*24*time.Hour {
			return nil, ErrApplyCooldown
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	_ = lastRejected
	// 学员每日投递上限
	loc := clock.Location()
	now := clock.Now()
	y, m, d := now.In(loc).Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	var todayCnt int64
	if err := s.db.Model(&model.JobApplication{}).Where("student_user_id = ? AND created_at >= ?", studentUserID, dayStart).Count(&todayCnt).Error; err != nil {
		return nil, err
	}
	if todayCnt >= int64(s.dailyLimit) {
		return nil, ErrApplyDailyLimit
	}

	var dto ApplicationDTO
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 投递记录
		app := model.JobApplication{
			JobPostingID:    jobPostingID,
			RecruiterID:     job.RecruiterID,
			StudentUserID:   studentUserID,
			Status:          "applied",
			ResumeUpdatedAt: card.UpdatedAt,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&app).Error; err != nil {
			return err
		}
		// 2. 授权：同一事务内写入/复活一条 approved 的联系方式交换记录（source=application）
		//    该企业对该学员已有 pending 申请 → 覆盖为 approved（学员主动投递优先于待决申请）。
		var pending model.ContactRequest
		pendingErr := tx.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", job.RecruiterID, studentUserID, "pending").First(&pending).Error
		if pendingErr == nil {
			// 覆盖 pending → approved
			if err := tx.Model(&model.ContactRequest{}).Where("id = ?", pending.ID).Updates(map[string]any{
				"status":     "approved",
				"decided_at": now,
				"updated_at": now,
				"source":     "application",
			}).Error; err != nil {
				return err
			}
		} else if errors.Is(pendingErr, gorm.ErrRecordNotFound) {
			// 无 pending → 新写一条 approved（投递产生的授权）
			req := model.ContactRequest{
				RecruiterID:   job.RecruiterID,
				StudentUserID: studentUserID,
				Message:       applyMessage(job.Title),
				Status:        "approved",
				Source:        "application",
				CreatedAt:     now,
				UpdatedAt:     now,
				DecidedAt:     &now,
				ExpiresAt:     now.Add(14 * 24 * time.Hour),
			}
			if err := tx.Create(&req).Error; err != nil {
				return err
			}
		} else {
			return pendingErr
		}
		// 3. 已存在 revoked 的授权 → 复活为 approved（学员重新投递即重新授权）
		var revoked model.ContactRequest
		revokedErr := tx.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", job.RecruiterID, studentUserID, "revoked").Order("decided_at DESC").First(&revoked).Error
		if revokedErr == nil {
			if err := tx.Model(&model.ContactRequest{}).Where("id = ?", revoked.ID).Updates(map[string]any{
				"status":     "approved",
				"decided_at": now,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		} else if !errors.Is(revokedErr, gorm.ErrRecordNotFound) {
			return revokedErr
		}
		dto = s.toDTO(&app)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 邮件通知企业对外邮箱（新投递；投递即授权，企业可直接联系）
	s.notifyEmployer(job.RecruiterID, job.Title, studentUserID)
	return &dto, nil
}

// notifyEmployer 邮件通知企业（发到企业联系邮箱；mailer 为 nil 时降级日志）。
func (s *JobApplicationService) notifyEmployer(recruiterID int, jobTitle string, studentUserID int) {
	var rec model.RecruiterUser
	if err := s.db.First(&rec, recruiterID).Error; err != nil {
		return
	}
	if rec.ContactEmail == "" {
		return
	}
	subject := "收到新的职位投递"
	body := fmt.Sprintf("你的职位「%s」收到一名学员的投递（学员 ID: %d）。投递即授权，你现在可以查看对方的联系方式。", jobTitle, studentUserID)
	if s.mailer != nil {
		_ = s.mailer.Send(rec.ContactEmail, subject, body)
	} else if s.logger != nil {
		s.logger.Info("job application created (mailer missing, log only)",
			zap.Int("recruiter", recruiterID), zap.Int("student", studentUserID), zap.String("job", jobTitle))
	}
}

// Withdraw 学员撤回投递。revokeContact 默认 false：撤回投递不连带收回联系方式授权；
// 仅当显式带上连带意图才置授权 revoked（此后明文端点 403）。
// 撤回后可立即重新投递同一职位。
func (s *JobApplicationService) Withdraw(studentUserID int, applicationID int64, revokeContact bool) (*ApplicationDTO, error) {
	var app model.JobApplication
	if err := s.db.First(&app, applicationID).Error; err != nil {
		return nil, ErrApplyNotFound
	}
	if app.StudentUserID != studentUserID {
		return nil, ErrApplyNotYours
	}
	if app.Status != "applied" {
		return nil, errors.New("仅投递中的申请可撤回")
	}
	now := clock.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.JobApplication{}).Where("id = ? AND status = ?", applicationID, "applied").Updates(map[string]any{
			"status":       "withdrawn",
			"withdrawn_at": now,
			"updated_at":   now,
		}).Error; err != nil {
			return err
		}
		if revokeContact {
			// 连带：把投递产生的 approved 授权置 revoked（此后明文端点 403）
			if err := tx.Model(&model.ContactRequest{}).
				Where("recruiter_id = ? AND student_user_id = ? AND status = ? AND source = ?", app.RecruiterID, studentUserID, "approved", "application").
				Updates(map[string]any{"status": "revoked", "decided_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.db.First(&app, applicationID).Error
	dto := s.toDTO(&app)
	return &dto, nil
}

// ListForStudent 学员「我的投递」列表。
func (s *JobApplicationService) ListForStudent(studentUserID, page, pageSize int) ([]ApplicationDTO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.JobApplication{}).Where("student_user_id = ?", studentUserID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.JobApplication
	if err := s.db.Where("student_user_id = ?", studentUserID).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	dtos := make([]ApplicationDTO, 0, len(rows))
	for i := range rows {
		d := s.toDTO(&rows[i])
		// 职位标题
		var job model.JobPosting
		if err := s.db.First(&job, rows[i].JobPostingID).Error; err == nil {
			d.JobTitle = job.Title
		}
		dtos = append(dtos, d)
	}
	return dtos, total, nil
}
