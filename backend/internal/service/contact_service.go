// Package service 联系方式交换闭环（#375）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/daemon"
	"forklift-training/internal/model"
)

// 联系方式交换域业务错误哨兵（ADR-0024）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrContactNoAuth 无有效授权（无 approved 授权或授权已失效）。
	ErrContactNoAuth = errors.New("无有效授权")
	// ErrStudentGone 学员不存在或已注销。
	ErrStudentGone = errors.New("学员不存在或已注销")
)

// ContactService 联系方式交换申请服务（L3）。
type ContactService struct {
	db              *gorm.DB
	logger          *zap.Logger
	notificationSvc *NotificationService
	mailer          MailSender
	dailyLimit      int
}

// NewContactService 构造服务。mailer 可为 nil（测试或未配置时降级为日志）。
func NewContactService(db *gorm.DB, logger *zap.Logger, notificationSvc *NotificationService, mailer MailSender) *ContactService {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &ContactService{db: db, logger: logger, notificationSvc: notificationSvc, mailer: mailer, dailyLimit: 20}
}

// ContactRequestDTO 申请展示对象（对招聘方与学员侧复用，部分字段按角色过滤）。
type ContactRequestDTO struct {
	ID            int64   `json:"id"`
	RecruiterID   int     `json:"recruiter_id"`
	StudentUserID int     `json:"student_user_id"`
	Message       string  `json:"message"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	DecidedAt     *string `json:"decided_at,omitempty"`
	ExpiresAt     string  `json:"expires_at"`
	// 企业信息（学员侧可见，不含电话）
	CompanyName string `json:"company_name,omitempty"`
	ContactName string `json:"contact_name,omitempty"`
}

// toDTO 转换 DB 行为 DTO，带企业信息（学员侧用）。
func (s *ContactService) toDTO(m *model.ContactRequest) ContactRequestDTO {
	var decided *string
	if m.DecidedAt != nil {
		s := m.DecidedAt.Format(time.RFC3339)
		decided = &s
	}
	dto := ContactRequestDTO{
		ID:            m.ID,
		RecruiterID:   m.RecruiterID,
		StudentUserID: m.StudentUserID,
		Message:       m.Message,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.Format(time.RFC3339),
		DecidedAt:     decided,
		ExpiresAt:     m.ExpiresAt.Format(time.RFC3339),
	}
	// 回填企业信息（尽力而为，不让查询失败阻塞）
	var rec model.RecruiterUser
	if err := s.db.First(&rec, m.RecruiterID).Error; err == nil {
		dto.CompanyName = rec.CompanyName
		dto.ContactName = rec.ContactName
	}
	return dto
}

// Create 创建申请（招聘方发起）。
func (s *ContactService) Create(recruiterID, studentUserID int, message string) (*ContactRequestDTO, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil, errors.New("附言不能为空")
	}
	if len([]rune(msg)) > 200 {
		return nil, errors.New("附言不能超过 200 字")
	}
	if recruiterID <= 0 || studentUserID <= 0 {
		return nil, errors.New("参数错误")
	}
	// 学生是否存在（已注销则 fail）
	var stu model.HrwaiUser
	if err := s.db.First(&stu, studentUserID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}
	// 招聘者是否存在且启用
	var rec model.RecruiterUser
	if err := s.db.First(&rec, recruiterID).Error; err != nil {
		return nil, errors.New("招聘者不存在")
	}
	if rec.Status != 1 {
		return nil, errors.New("招聘者账号已禁用")
	}
	// 学员简历是否公开？（可选：不校验，允许向 hidden 发，但 L2 不可见时申请仍可发起？ spec 未限制，此处不拦）
	now := clock.Now()
	// pending 唯一：同一企业对同一学员在 pending 期间只能有一条
	var pendingCnt int64
	if err := s.db.Model(&model.ContactRequest{}).Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, "pending").Count(&pendingCnt).Error; err != nil {
		return nil, err
	}
	if pendingCnt > 0 {
		var existing model.ContactRequest
		_ = s.db.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, "pending").First(&existing).Error
		dto := s.toDTO(&existing)
		return &dto, errors.New("已存在待处理的申请")
	}
	// 30 天冷却：被拒绝或被撤回后 30 天内不能再申请
	var lastRejected *model.ContactRequest
	var last model.ContactRequest
	if err := s.db.Where("recruiter_id = ? AND student_user_id = ? AND status IN ?", recruiterID, studentUserID, []string{"rejected", "revoked"}).Order("decided_at DESC").First(&last).Error; err == nil {
		lastRejected = &last
		if last.DecidedAt != nil && now.Sub(*last.DecidedAt) < 30*24*time.Hour {
			return nil, errors.New("该学员 30 天内拒绝或撤回过申请，冷却期内不能重复申请")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	_ = lastRejected
	// 日限：单个企业每日发起申请数有上限（默认 20）
	loc := clock.Location()
	y, m, d := now.In(loc).Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	var todayCnt int64
	if err := s.db.Model(&model.ContactRequest{}).Where("recruiter_id = ? AND created_at >= ?", recruiterID, dayStart).Count(&todayCnt).Error; err != nil {
		return nil, err
	}
	if todayCnt >= int64(s.dailyLimit) {
		return nil, errors.New("今日申请已达上限")
	}
	expiresAt := now.Add(14 * 24 * time.Hour)
	mdl := model.ContactRequest{
		RecruiterID:   recruiterID,
		StudentUserID: studentUserID,
		Message:       msg,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     expiresAt,
	}
	if err := s.db.Create(&mdl).Error; err != nil {
		return nil, err
	}
	// 站内信通知学员（不含企业电话）
	if s.notificationSvc != nil {
		payloadMap := map[string]any{"contact_request_id": mdl.ID, "recruiter_id": recruiterID}
		b, _ := json.Marshal(payloadMap)
		content := fmt.Sprintf("企业「%s」联系人 %s 申请查看你的联系方式，附言：%s", rec.CompanyName, rec.ContactName, msg)
		_ = s.notificationSvc.Create(studentUserID, "contact_request", "收到联系方式交换申请", content, "/training/resume", model.JSONB(b))
	}
	dto := s.toDTO(&mdl)
	return &dto, nil
}

// ListForRecruiter 招聘方查看我的申请列表。
func (s *ContactService) ListForRecruiter(recruiterID, page, pageSize int) ([]ContactRequestDTO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.ContactRequest{}).Where("recruiter_id = ?", recruiterID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.ContactRequest
	if err := s.db.Where("recruiter_id = ?", recruiterID).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	dtos := make([]ContactRequestDTO, 0, len(rows))
	for i := range rows {
		dtos = append(dtos, s.toDTO(&rows[i]))
	}
	return dtos, total, nil
}

// ListForStudent 学员侧查看收到的申请。
func (s *ContactService) ListForStudent(studentUserID, page, pageSize int) ([]ContactRequestDTO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.ContactRequest{}).Where("student_user_id = ?", studentUserID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.ContactRequest
	if err := s.db.Where("student_user_id = ?", studentUserID).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	dtos := make([]ContactRequestDTO, 0, len(rows))
	for i := range rows {
		dtos = append(dtos, s.toDTO(&rows[i]))
	}
	return dtos, total, nil
}

// Approve 学员同意申请。
func (s *ContactService) Approve(studentUserID int, requestID int64) (*ContactRequestDTO, error) {
	var req model.ContactRequest
	if err := s.db.First(&req, requestID).Error; err != nil {
		return nil, errors.New("申请不存在")
	}
	if req.StudentUserID != studentUserID {
		return nil, errors.New("无权操作")
	}
	if req.Status != "pending" {
		return nil, errors.New("仅 pending 申请可同意")
	}
	if clock.Now().After(req.ExpiresAt) {
		// 已过期，自动标记 expired
		_ = s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, "pending").Updates(map[string]any{"status": "expired", "updated_at": clock.Now()}).Error
		return nil, errors.New("申请已过期")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, "pending").Updates(map[string]any{"status": "approved", "decided_at": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	// 重新加载
	_ = s.db.First(&req, requestID).Error
	// 邮件通知招聘方（发到企业联系邮箱）
	var rec model.RecruiterUser
	if err := s.db.First(&rec, req.RecruiterID).Error; err == nil && s.mailer != nil && rec.ContactEmail != "" {
		subject := "联系方式交换申请已同意"
		body := fmt.Sprintf("学员已同意你的联系方式交换申请（ID: %d），你现在可以查看对方的联系方式与 PDF。", req.ID)
		_ = s.mailer.Send(rec.ContactEmail, subject, body)
	} else if s.mailer == nil && s.logger != nil {
		s.logger.Info("contact request approved (mailer missing, log only)", zap.Int64("request_id", req.ID), zap.Int("recruiter", req.RecruiterID))
	}
	dto := s.toDTO(&req)
	return &dto, nil
}

// Reject 学员拒绝申请。
func (s *ContactService) Reject(studentUserID int, requestID int64) (*ContactRequestDTO, error) {
	var req model.ContactRequest
	if err := s.db.First(&req, requestID).Error; err != nil {
		return nil, errors.New("申请不存在")
	}
	if req.StudentUserID != studentUserID {
		return nil, errors.New("无权操作")
	}
	if req.Status != "pending" {
		return nil, errors.New("仅 pending 申请可拒绝")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, "pending").Updates(map[string]any{"status": "rejected", "decided_at": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&req, requestID).Error
	dto := s.toDTO(&req)
	return &dto, nil
}

// Revoke 学员撤回已同意的授权（实时生效）。
func (s *ContactService) Revoke(studentUserID int, requestID int64) (*ContactRequestDTO, error) {
	var req model.ContactRequest
	if err := s.db.First(&req, requestID).Error; err != nil {
		return nil, errors.New("申请不存在")
	}
	if req.StudentUserID != studentUserID {
		return nil, errors.New("无权操作")
	}
	if req.Status != "approved" {
		return nil, errors.New("仅已同意的申请可撤回")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, "approved").Updates(map[string]any{"status": "revoked", "decided_at": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&req, requestID).Error
	dto := s.toDTO(&req)
	return &dto, nil
}

// ExpirePending 将超时的 pending 申请标记为 expired（由守护 runner 周期调用）。
// 返回本次过期的条数。
func (s *ContactService) ExpirePending(now time.Time) (int64, error) {
	if now.IsZero() {
		now = clock.Now()
	}
	res := s.db.Model(&model.ContactRequest{}).Where("status = ? AND expires_at <= ?", "pending", now).Updates(map[string]any{"status": "expired", "updated_at": now})
	return res.RowsAffected, res.Error
}

// GetContact 明文联系方式与 PDF 仅在有效授权下返回（approved 且未 revoked/expired，实时校验，无缓存）。
// 返回的 JobCardDTO 包含明文 phone/wechat/real_name/resume_file_url。
func (s *ContactService) GetContact(recruiterID, studentUserID int) (*JobCardDTO, error) {
	// 校验授权存在且为 approved
	var req model.ContactRequest
	err := s.db.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, "approved").Order("decided_at DESC").First(&req).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContactNoAuth
		}
		return nil, err
	}
	// 学员是否已注销（hrwai_users 不存在则授权一并失效）
	var stu model.HrwaiUser
	if err := s.db.First(&stu, studentUserID).Error; err != nil {
		return nil, ErrStudentGone
	}
	// 读取简历卡（实时，无缓存）
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", studentUserID).Error; err != nil {
		return nil, errors.New("简历不存在")
	}
	dto := toJobCardDTO(&card)
	return &dto, nil
}

// SetDailyLimit 测试用：覆盖每日上限。
func (s *ContactService) SetDailyLimit(n int) { s.dailyLimit = n }

// StartExpireRunner 启动 pending 14 天过期守护（进程内定时任务、panic 恢复、jitter 错峰、context 取消贯穿）。
func (s *ContactService) StartExpireRunner(ctx context.Context, interval time.Duration, logger *zap.Logger) *daemon.Runner {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = s.logger
	}
	runner := daemon.NewRunner("contact-request-expire", interval, logger, func(runCtx context.Context) {
		if _, err := s.ExpirePending(clock.Now()); err != nil && logger != nil {
			logger.Warn("contact expire 失败", zap.Error(err))
		}
	}, daemon.WithJitter(interval/10))
	runner.Start(ctx)
	return runner
}
