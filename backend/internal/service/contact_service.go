// Package service 联系方式交换闭环（#375）。
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
	"forklift-training/pkg/paging"
)

// 联系方式交换域业务错误哨兵（ADR-0024）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrContactNoAuth 无有效授权（无 approved 授权或授权已失效）。
	ErrContactNoAuth = errors.New("无有效授权")
	// ErrStudentGone 学员不存在或已注销。
	ErrStudentGone = errors.New("学员不存在或已注销")
)

// contactDecisionWindow 裁决窗口长度：pending 等学员裁决的时限（ADR-0061 §2）。
// **签发时快照的唯一来源**——改它只影响之后新签发的 pending，存量行的 expires_at 保持各自
// 签发时的值（旧行的窗口是当时对学员做过的承诺，不被新常量追溯改写）。
const contactDecisionWindow = 14 * 24 * time.Hour

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
	DecidedAt     *string `json:"decided_at,omitempty" extensions:"x-optional"`
	// ExpiresAt 裁决窗口的关闭时刻，**只对 pending 有意义**（ADR-0061 §2）：非 pending 行可能缺失，
	// 消费方不得把它读成「授权的到期时刻」（approved 是永久授权）。
	ExpiresAt *string `json:"expires_at,omitempty" extensions:"x-optional"`
	// 企业信息（学员侧可见）
	CompanyName string `json:"company_name,omitempty" extensions:"x-optional"`
	ContactName string `json:"contact_name,omitempty" extensions:"x-optional"`
	// 企业联系信息（#487：仅 status=approved 时透出——电话/邮箱/微信；其余状态一律缺失）
	ContactPhone string `json:"contact_phone,omitempty" extensions:"x-optional"`
	ContactEmail string `json:"contact_email,omitempty" extensions:"x-optional"`
	Wechat       string `json:"wechat,omitempty" extensions:"x-optional"`
	// Source 授权来源（recruiter 企业发起 / application 投递产生）
	Source string `json:"source,omitempty" extensions:"x-optional"`
}

// ContactRequestListResult 交换申请分页结果：**真返回类型**（#1095 前只服务 swagger，运行时出字节的是 gin.H）。
//
// 字段声明序 = 旧 gin.H map 输出的键序（encoding/json 对 map 按 key 排序：items < page < page_size < total），
// 故换成 typed DTO 后响应字节逐字节不变（ADR-0009 §2；字节锁见 envelope_dto_shape_test.go 与信封登记表）。
type ContactRequestListResult struct {
	Items    []ContactRequestDTO `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Total    int64               `json:"total"`
}

// ContactPlainDTO 明文联系方式及其补齐面（GET /api/recruit/resumes/{id}/contact）。
//
// 同样与 handler 既有内联 map 的字段集一致（仅注解层的形状声明，不改响应构造）。
// Photos / ResumeCertifications 是 JSONB 直出的数组，用 service.JSONArray 承接
// （json.RawMessage 会让 swag 解析失败，见该类型的注释）。
type ContactPlainDTO struct {
	RealName             string    `json:"real_name"`
	ContactPhone         string    `json:"contact_phone"`
	Wechat               string    `json:"wechat"`
	ResumeFileURL        string    `json:"resume_file_url"`
	Photos               JSONArray `json:"photos" swaggertype:"array,string"`
	ResumeCertifications JSONArray `json:"resume_certifications" swaggertype:"array,object"`
}

// toDTO 转换 DB 行为 DTO，带企业信息（学员侧用）。
func (s *ContactService) toDTO(m *model.ContactRequest) ContactRequestDTO {
	var decided *string
	if m.DecidedAt != nil {
		s := m.DecidedAt.Format(time.RFC3339)
		decided = &s
	}
	// 窗口时刻可空：非 pending 行不再输出日期（旧代码会输出零值时间，见 ADR-0061 §2）。
	var window *string
	if m.ExpiresAt != nil {
		w := m.ExpiresAt.Format(time.RFC3339)
		window = &w
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
		ExpiresAt:     window,
	}
	// 回填企业信息（尽力而为，不让查询失败阻塞）
	// #487：仅已批准时透出联系信息——谓词单点在 contact_authz.go（GrantsPlaintext）
	var rec model.RecruiterUser
	if err := s.db.First(&rec, m.RecruiterID).Error; err == nil {
		dto.CompanyName = rec.CompanyName
		dto.ContactName = rec.ContactName
		if ContactGrantState(m.Status).GrantsPlaintext() {
			dto.ContactPhone = rec.ContactPhone
			dto.ContactEmail = rec.ContactEmail
			dto.Wechat = rec.Wechat
		}
	}
	dto.Source = m.Source
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
	// 判唯一前先把这一对**已闭窗却仍挂 pending** 的行落态（ADR-0061 §2）：应用层计数与库层偏索引
	// 都只认 status，不落态就会让窗口早已关闭的行把企业长期挡在「已存在待处理的申请」里，
	// 唯一的出口是学员碰巧点开。守护只兜没人触碰的行，不兜正在重试的这一次。
	if _, err := s.expireClosed(now, s.pairScope(recruiterID, studentUserID)); err != nil {
		return nil, err
	}
	// pending 唯一：同一企业对同一学员在 pending 期间只能有一条
	var pendingCnt int64
	if err := s.db.Model(&model.ContactRequest{}).Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, string(ContactGrantPending)).Count(&pendingCnt).Error; err != nil {
		return nil, err
	}
	if pendingCnt > 0 {
		var existing model.ContactRequest
		_ = s.db.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, string(ContactGrantPending)).First(&existing).Error
		dto := s.toDTO(&existing)
		return &dto, errors.New("已存在待处理的申请")
	}
	// 30 天冷却：被拒绝或被撤回后 30 天内不能再申请
	var lastRejected *model.ContactRequest
	var last model.ContactRequest
	if err := s.db.Where("recruiter_id = ? AND student_user_id = ? AND status IN ?", recruiterID, studentUserID, []string{string(ContactGrantRejected), string(ContactGrantRevoked)}).Order("decided_at DESC").First(&last).Error; err == nil {
		lastRejected = &last
		if last.DecidedAt != nil && now.Sub(*last.DecidedAt) < 30*24*time.Hour {
			return nil, errors.New("该学员 30 天内拒绝或撤回过申请，冷却期内不能重复申请")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	_ = lastRejected
	// 日限：单个企业每日发起申请数有上限（默认 20）
	dayStart := clock.DayStart(now)
	var todayCnt int64
	if err := s.db.Model(&model.ContactRequest{}).Where("recruiter_id = ? AND created_at >= ?", recruiterID, dayStart).Count(&todayCnt).Error; err != nil {
		return nil, err
	}
	if todayCnt >= int64(s.dailyLimit) {
		return nil, errors.New("今日申请已达上限")
	}
	expiresAt := now.Add(contactDecisionWindow)
	mdl := model.ContactRequest{
		RecruiterID:   recruiterID,
		StudentUserID: studentUserID,
		Message:       msg,
		Status:        string(ContactGrantPending),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     &expiresAt,
	}
	if err := s.db.Create(&mdl).Error; err != nil {
		// 撞库层偏索引（迁移 000010:16 `WHERE status='pending'`）= 并发对手刚写入一条 pending。
		// 这里**回读确认**再复用同一句文案，而不是按错误码分类：本仓 gorm 未开 TranslateError
		// （拿到的是驱动原始错误），且 sqlite 测试库由 AutoMigrate 建表、根本没有这条索引。
		// 回读为真时该文案与计数分支说的是同一件事——对手那条 pending 确实存在。
		var loserCnt int64
		if e := s.pairScope(recruiterID, studentUserID).
			Where("status = ?", string(ContactGrantPending)).
			Count(&loserCnt).Error; e == nil && loserCnt > 0 {
			return nil, errors.New("已存在待处理的申请")
		}
		return nil, err
	}
	// 站内信通知学员（不含企业电话；尽力而为，接收器 nil 与失败均吞；ADR-0027 C1 收编）
	s.notificationSvc.TryCreateContactRequestEvent(NewContactRequestEvent(studentUserID, rec.CompanyName, rec.ContactName, msg, mdl.ID, recruiterID))
	dto := s.toDTO(&mdl)
	return &dto, nil
}

// EnsureApproved 投递即授权（ADR-0025/ADR-0027 C5）：事务内将企业对该学员的
// 联系方式交换授权状态迁移为 approved，三分支收口 contact 域单点——
//  1. 已有 pending 申请 → 覆盖为 approved（学员主动投递优先于待决申请）；
//  2. 无 pending → 新写一条 approved（source=application，投递产生的授权）；
//  3. 已有 revoked 的授权 → 复活为 approved（学员重新投递即重新授权）。
//
// 与既有投递事务语义一致：三分支顺序执行（1/2 为互斥分支，3 独立判定），
// 仅在事务内调用（tx 传入，不带新事务边界）。
//
// 窗口列的处置（ADR-0061 §2）：分支 2 新建的 approved **不写** `expires_at`——裁决窗口只属于
// pending，给永久授权写一个期限就是让它替不存在的事实说话；分支 1 覆盖 pending 时保留该行
// 原窗口值，作为「它当初的期限是这天」的历史留痕，此后无人再读。
func (s *ContactService) EnsureApproved(tx *gorm.DB, recruiterID, studentUserID int, message string, now time.Time) error {
	// 1/2. pending 覆盖 or 新建 approved
	var pending model.ContactRequest
	pendingErr := tx.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, string(ContactGrantPending)).First(&pending).Error
	switch {
	case pendingErr == nil:
		if err := tx.Model(&model.ContactRequest{}).Where("id = ?", pending.ID).Updates(map[string]any{
			"status":     string(ContactGrantApproved),
			"decided_at": now,
			"updated_at": now,
			"source":     string(ContactGrantSourceApplication),
		}).Error; err != nil {
			return err
		}
	case errors.Is(pendingErr, gorm.ErrRecordNotFound):
		req := model.ContactRequest{
			RecruiterID:   recruiterID,
			StudentUserID: studentUserID,
			Message:       message,
			Status:        string(ContactGrantApproved),
			Source:        string(ContactGrantSourceApplication),
			CreatedAt:     now,
			UpdatedAt:     now,
			DecidedAt:     &now,
		}
		if err := tx.Create(&req).Error; err != nil {
			return err
		}
	default:
		return pendingErr
	}
	// 3. 已存在 revoked 的授权 → 复活为 approved（学员重新投递即重新授权）
	var revoked model.ContactRequest
	revokedErr := tx.Where("recruiter_id = ? AND student_user_id = ? AND status = ?", recruiterID, studentUserID, string(ContactGrantRevoked)).Order("decided_at DESC").First(&revoked).Error
	if revokedErr == nil {
		return tx.Model(&model.ContactRequest{}).Where("id = ?", revoked.ID).Updates(map[string]any{
			"status":     string(ContactGrantApproved),
			"decided_at": now,
			"updated_at": now,
		}).Error
	}
	if !errors.Is(revokedErr, gorm.ErrRecordNotFound) {
		return revokedErr
	}
	return nil
}

// ListForRecruiter 招聘方查看我的申请列表。
func (s *ContactService) ListForRecruiter(recruiterID, page, pageSize int) ([]ContactRequestDTO, int64, error) {
	rows, total, _, _, err := paging.QueryWithMax[model.ContactRequest](s.db, page, pageSize, 20, 50,
		"created_at DESC", func(q *gorm.DB) *gorm.DB {
			return q.Where("recruiter_id = ?", recruiterID)
		})
	if err != nil {
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
	rows, total, _, _, err := paging.QueryWithMax[model.ContactRequest](s.db, page, pageSize, 20, 50,
		"created_at DESC", func(q *gorm.DB) *gorm.DB {
			return q.Where("student_user_id = ?", studentUserID)
		})
	if err != nil {
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
	if req.Status != string(ContactGrantPending) {
		return nil, errors.New("仅 pending 申请可同意")
	}
	// 窗口已闭：当场落态再拒（ADR-0061 §2 的按行出口）。pending 行必带窗口（迁移 000039 的 CHECK），
	// 但列可空 ⇒ nil 只可能来自脏数据，此时按「未闭窗」放行、交给上面的状态判定兜，不 panic。
	if req.ExpiresAt != nil && clock.Now().After(*req.ExpiresAt) {
		_, _ = s.expireClosed(clock.Now(), s.db.Where("id = ?", req.ID))
		return nil, errors.New("申请已过期")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, string(ContactGrantPending)).Updates(map[string]any{"status": string(ContactGrantApproved), "decided_at": now, "updated_at": now}).Error; err != nil {
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
	if req.Status != string(ContactGrantPending) {
		return nil, errors.New("仅 pending 申请可拒绝")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, string(ContactGrantPending)).Updates(map[string]any{"status": string(ContactGrantRejected), "decided_at": now, "updated_at": now}).Error; err != nil {
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
	if req.Status != string(ContactGrantApproved) {
		return nil, errors.New("仅已同意的申请可撤回")
	}
	now := clock.Now()
	if err := s.db.Model(&model.ContactRequest{}).Where("id = ? AND status = ?", req.ID, string(ContactGrantApproved)).Updates(map[string]any{"status": string(ContactGrantRevoked), "decided_at": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&req, requestID).Error
	dto := s.toDTO(&req)
	return &dto, nil
}

// pairScope 限定到某一对 (企业, 学员) 的查询起点。调用方拿到的是一条**未执行**的链，
// 可继续叠加谓词——定向落态与冲突回读共用它，避免同一对关系的限定写两遍。
func (s *ContactService) pairScope(recruiterID, studentUserID int) *gorm.DB {
	return s.db.Where("recruiter_id = ? AND student_user_id = ?", recruiterID, studentUserID)
}

// expireClosed 落态的**唯一执行点**：把「窗口已关闭却仍挂 pending」的行置为 expired。
// scope 为 nil = 全表（守护），否则由 caller 传入限定链（按对 / 按行）——三种收敛速度
// （周期 / 企业重试 / 学员裁决）共用同一条语句，同一事实不再有三个写法。
// 谓词自带 `status = pending` 且只向前推进，重复调用幂等。
func (s *ContactService) expireClosed(now time.Time, scope *gorm.DB) (int64, error) {
	if now.IsZero() {
		now = clock.Now()
	}
	if scope == nil {
		scope = s.db
	}
	res := scope.Model(&model.ContactRequest{}).
		Where("status = ? AND expires_at <= ?", string(ContactGrantPending), now).
		Updates(map[string]any{"status": string(ContactGrantExpired), "updated_at": now})
	return res.RowsAffected, res.Error
}

// ExpirePending 全表收敛超时 pending（守护 runner 周期调用）。返回本次落态条数。
func (s *ContactService) ExpirePending(now time.Time) (int64, error) {
	return s.expireClosed(now, nil)
}

// GetContact 明文联系方式与 PDF 仅在有效授权下返回（存在已批准授权，实时校验，无缓存）。
// 授权判据与「学员注销即失效」收口在 contact_authz.go（ADR-0053 §3）。
// 返回的 JobCardDTO 包含明文 phone/wechat/real_name/resume_file_url。
func (s *ContactService) GetContact(recruiterID, studentUserID int) (*JobCardDTO, error) {
	if _, err := contactGrantEffectiveOf(s.db, recruiterID, studentUserID); err != nil {
		return nil, err
	}
	// 读取简历卡（实时，无缓存）
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", studentUserID).Error; err != nil {
		return nil, errors.New("简历不存在")
	}
	dto := toJobCardDTO(&card)
	return &dto, nil
}

// RevokeApplicationGrant 撤投的连带迁移：把「投递产生」的已批准授权置为已撤回
// （此后明文端点即无有效授权）。与 EnsureApproved 对称——授权的全部迁移只剩这两个出口。
// 只在事务内调用（tx 传入，不带新事务边界）。
func (s *ContactService) RevokeApplicationGrant(tx *gorm.DB, recruiterID, studentUserID int, now time.Time) error {
	return tx.Model(&model.ContactRequest{}).
		Where("recruiter_id = ? AND student_user_id = ? AND status = ? AND source = ?",
			recruiterID, studentUserID, string(ContactGrantApproved), string(ContactGrantSourceApplication)).
		Updates(map[string]any{
			"status":     string(ContactGrantRevoked),
			"decided_at": now,
			"updated_at": now,
		}).Error
}

// SetDailyLimit 测试用：覆盖每日上限。
func (s *ContactService) SetDailyLimit(n int) { s.dailyLimit = n }
