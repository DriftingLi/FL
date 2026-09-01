// Package service 招聘域：企业处理投递（spec #449 T4 #453）。
// 已读、漂移、拒绝与徽标：按职位分页查看投递（越权 403）、打开详情即记录已读、
// 候选人走既有唯一脱敏路径、回显投递时刻简历更新时间（版本指针）、标记不合适→rejected（30 天冷却）。
package service

import (
	"errors"
	"time"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// RecruiterApplicationListResult 企业侧投递列表结果。
type RecruiterApplicationListResult struct {
	Items       []ApplicationDTO `json:"items"`
	Total       int64            `json:"total"`
	Page        int              `json:"page"`
	PageSize    int              `json:"page_size"`
	UnreadCount int64            `json:"unread_count"`
	JobTitle    string           `json:"job_title"`
}

// ListForRecruiter 企业侧按职位分页查看投递（只能看自己职位的投递，越权 → ErrApplyNotYours）。
// 列表返回未读投递数（判据：企业尚未打开过该投递）；候选人仍走既有唯一脱敏路径
// （姓名打码、无手机号/微信/PDF/证书原图——明文只经既有的联系方式端点取得）。
func (s *JobApplicationService) ListForRecruiter(recruiterID, jobPostingID, page, pageSize int) (*RecruiterApplicationListResult, error) {
	var job model.JobPosting
	if err := s.db.First(&job, jobPostingID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if job.RecruiterID != recruiterID {
		return nil, ErrApplyNotYours
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.JobApplication{}).Where("job_posting_id = ?", jobPostingID).Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.JobApplication
	if err := s.db.Where("job_posting_id = ?", jobPostingID).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	var unread int64
	if err := s.db.Model(&model.JobApplication{}).Where("job_posting_id = ? AND employer_viewed_at IS NULL", jobPostingID).Count(&unread).Error; err != nil {
		return nil, err
	}
	items := make([]ApplicationDTO, 0, len(rows))
	for i := range rows {
		d := s.toDTO(&rows[i])
		// 脱敏学员信息（唯一脱敏路径：姓名打码，无手机/微信/PDF/证书原图）
		var card model.JobCard
		if err := s.db.First(&card, "user_id = ?", rows[i].StudentUserID).Error; err == nil {
			d.StudentRealNameMasked = MaskRealName(card.RealName)
			d.StudentResumeUpdatedAt = card.UpdatedAt.Format(time.RFC3339)
			d.ResumeUpdatedAtSnapshot = rows[i].ResumeUpdatedAt.Format(time.RFC3339)
		}
		items = append(items, d)
	}
	return &RecruiterApplicationListResult{
		Items:       items,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		UnreadCount: unread,
		JobTitle:    job.Title,
	}, nil
}

// GetForRecruiter 企业查看投递详情：记录已读（employer_viewed_at），返回脱敏候选人信息 + 简历更新时间指针。
func (s *JobApplicationService) GetForRecruiter(recruiterID int, applicationID int64) (*ApplicationDTO, error) {
	var app model.JobApplication
	if err := s.db.First(&app, applicationID).Error; err != nil {
		return nil, ErrApplyNotFound
	}
	if app.RecruiterID != recruiterID {
		return nil, ErrApplyNotYours
	}
	now := clock.Now()
	if app.EmployerViewAt == nil {
		if err := s.db.Model(&model.JobApplication{}).Where("id = ?", applicationID).Updates(map[string]any{
			"employer_viewed_at": now,
			"updated_at":         now,
		}).Error; err != nil {
			return nil, err
		}
		_ = s.db.First(&app, applicationID).Error
	}
	dto := s.toDTO(&app)
	var job model.JobPosting
	if err := s.db.First(&job, app.JobPostingID).Error; err == nil {
		dto.JobTitle = job.Title
	}
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", app.StudentUserID).Error; err == nil {
		dto.StudentRealNameMasked = MaskRealName(card.RealName)
		dto.StudentResumeUpdatedAt = card.UpdatedAt.Format(time.RFC3339)
		dto.ResumeUpdatedAtSnapshot = app.ResumeUpdatedAt.Format(time.RFC3339)
	}
	return &dto, nil
}

// Reject 企业标记投递为不合适 → 终态 rejected；同一学员对该职位 30 天内再投被拒（冷却）。
// 企业不能把已拒绝的投递改回待处理以外的状态（仅 applied 可拒）；学员不能替企业标记（handler 角色守卫）。
func (s *JobApplicationService) Reject(recruiterID int, applicationID int64) (*ApplicationDTO, error) {
	var app model.JobApplication
	if err := s.db.First(&app, applicationID).Error; err != nil {
		return nil, ErrApplyNotFound
	}
	if app.RecruiterID != recruiterID {
		return nil, ErrApplyNotYours
	}
	if app.Status != "applied" {
		return nil, errors.New("仅投递中的申请可标记不合适")
	}
	now := clock.Now()
	if err := s.db.Model(&model.JobApplication{}).Where("id = ? AND status = ?", applicationID, "applied").Updates(map[string]any{
		"status":      "rejected",
		"rejected_at": now,
		"updated_at":  now,
	}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&app, applicationID).Error
	dto := s.toDTO(&app)
	return &dto, nil
}
