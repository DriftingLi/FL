// Package service 实现业务服务层。
// 本文件：用户资料（昵称/头像）修改审核。
package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/storage"
	"forklift-training/pkg/response"
)

// 资料修改字段类型与状态。
const (
	ProfileFieldNickname = "nickname"
	ProfileFieldAvatar   = "avatar"

	ProfileStatusPending  = "pending"
	ProfileStatusApproved = "approved"
	ProfileStatusRejected = "rejected"
)

// ProfileChangeRequestDTO 审核请求展示对象（含用户当前资料）。
type ProfileChangeRequestDTO struct {
	ID           int64   `json:"id"`
	UserID       int     `json:"user_id"`
	Username     string  `json:"username"`
	Name         string  `json:"name"`
	Nickname     string  `json:"nickname"`
	AvatarURL    string  `json:"avatar_url"`
	FieldType    string  `json:"field_type"`
	OldValue     string  `json:"old_value"`
	NewValue     string  `json:"new_value"`
	Status       string  `json:"status"`
	RejectReason string  `json:"reject_reason"`
	ReviewedBy   *int    `json:"reviewed_by,omitempty"`
	ReviewedAt   *string `json:"reviewed_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// ProfileReviewService 资料修改审核服务。
type ProfileReviewService struct {
	db              *gorm.DB
	notificationSvc *NotificationService
	// storage 文件存储（头像文件生命周期：审核通过清理旧头像、驳回清理待审文件）
	storage storage.Storage

	logger *zap.Logger
}

// NewProfileReviewService 构造审核服务。
func NewProfileReviewService(db *gorm.DB, notificationSvc *NotificationService, st storage.Storage, logger *zap.Logger) *ProfileReviewService {
	return &ProfileReviewService{db: db, notificationSvc: notificationSvc, storage: st, logger: logger}
}

// CreateRequest 提交资料修改审核请求（不直接生效）。
func (s *ProfileReviewService) CreateRequest(userID int, fieldType, newValue string) (*ProfileChangeRequestDTO, error) {
	newValue = strings.TrimSpace(newValue)
	switch fieldType {
	case ProfileFieldNickname:
		if utf8.RuneCountInString(newValue) > 30 {
			return nil, errors.New("昵称不能超过 30 个字符")
		}
	case ProfileFieldAvatar:
		if newValue == "" {
			return nil, errors.New("头像不能为空")
		}
	default:
		return nil, errors.New("不支持的资料字段")
	}

	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	oldValue := user.Nickname
	if fieldType == ProfileFieldAvatar {
		oldValue = user.AvatarURL
	}
	if newValue == oldValue {
		return nil, errors.New("内容未发生变化")
	}

	// 同一字段存在待审请求时不允许重复提交
	var pendingCount int64
	if err := s.db.Model(&model.ProfileChangeRequest{}).
		Where("user_id = ? AND field_type = ? AND status = ?", userID, fieldType, ProfileStatusPending).
		Count(&pendingCount).Error; err != nil {
		return nil, err
	}
	if pendingCount > 0 {
		return nil, errors.New("该资料已有待审核的修改，请等待审核结果")
	}

	now := beijingNow()
	req := model.ProfileChangeRequest{
		UserID:    userID,
		FieldType: fieldType,
		OldValue:  oldValue,
		NewValue:  newValue,
		Status:    ProfileStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&req).Error; err != nil {
		return nil, err
	}
	return s.toDTO(&req, &user), nil
}

// GetPendingForUser 查询用户最新一条待审请求（无则返回 nil）。
func (s *ProfileReviewService) GetPendingForUser(userID int) (*ProfileChangeRequestDTO, error) {
	var req model.ProfileChangeRequest
	if err := s.db.Where("user_id = ? AND status = ?", userID, ProfileStatusPending).
		Order("id DESC").Limit(1).Find(&req).Error; err != nil {
		return nil, err
	}
	if req.ID == 0 {
		return nil, nil
	}
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return s.toDTO(&req, &user), nil
}

// ProfileChangeRequestPageResult 资料审核请求分页结果。
type ProfileChangeRequestPageResult struct {
	Page     int                       `json:"page"`
	Pages    int                       `json:"pages"`
	Requests []ProfileChangeRequestDTO `json:"requests"`
	Total    int64                     `json:"total"`
}

// ListRequests 分页查询审核请求（可按下单状态过滤）。
func (s *ProfileReviewService) ListRequests(status string, page, pageSize int) (*ProfileChangeRequestPageResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	q := s.db.Table("profile_change_requests AS r").
		Select("r.id, r.user_id, r.field_type, r.old_value, r.new_value, r.status, r.reject_reason, " +
			"r.reviewed_by, r.reviewed_at, r.created_at, " +
			"u.username, u.name, u.nickname, u.avatar_url").
		Joins("JOIN hrwai_users AS u ON u.id = r.user_id")
	if status != "" && status != "all" {
		q = q.Where("r.status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []struct {
		ID           int64
		UserID       int
		FieldType    string
		OldValue     string
		NewValue     string
		Status       string
		RejectReason string
		ReviewedBy   *int
		ReviewedAt   *time.Time
		CreatedAt    time.Time
		Username     string
		Name         string
		Nickname     string
		AvatarURL    string
	}
	if err := q.Order("r.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ProfileChangeRequestDTO, 0, len(rows))
	for i := range rows {
		r := rows[i]
		items = append(items, ProfileChangeRequestDTO{
			ID: r.ID, UserID: r.UserID,
			Username: r.Username, Name: r.Name,
			Nickname: r.Nickname, AvatarURL: r.AvatarURL,
			FieldType: r.FieldType, OldValue: r.OldValue, NewValue: r.NewValue,
			Status: r.Status, RejectReason: r.RejectReason,
			ReviewedBy: r.ReviewedBy,
			ReviewedAt: formatTimePtr(r.ReviewedAt),
			CreatedAt:  formatISO(r.CreatedAt),
		})
	}
	return &ProfileChangeRequestPageResult{
		Page:     page,
		Pages:    response.PageCount(total, pageSize),
		Requests: items,
		Total:    total,
	}, nil
}

// Approve 通过审核：将 new_value 应用到 hrwai_users 并标记状态。
func (s *ProfileReviewService) Approve(requestID int64, reviewerID int) (*ProfileChangeRequestDTO, error) {
	return s.review(requestID, reviewerID, ProfileStatusApproved, "")
}

// Reject 驳回审核，返回请求对象（待审头像文件由 service 在 review 内清理）。
func (s *ProfileReviewService) Reject(requestID int64, reviewerID int, reason string) (*ProfileChangeRequestDTO, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 200 {
		return nil, errors.New("驳回原因不能超过 200 个字符")
	}
	return s.review(requestID, reviewerID, ProfileStatusRejected, reason)
}

// review 执行审核状态流转：仅允许 pending → approved / rejected。
func (s *ProfileReviewService) review(requestID int64, reviewerID int, status, reason string) (*ProfileChangeRequestDTO, error) {
	var req model.ProfileChangeRequest
	if err := s.db.First(&req, requestID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("审核请求不存在")
		}
		return nil, err
	}
	if req.Status != ProfileStatusPending {
		return nil, errors.New("该请求已审核，不能重复操作")
	}

	now := beijingNow()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if status == ProfileStatusApproved {
			updates := map[string]any{}
			if req.FieldType == ProfileFieldNickname {
				updates["nickname"] = req.NewValue
			} else {
				updates["avatar_url"] = req.NewValue
			}
			if err := tx.Model(&model.HrwaiUser{}).Where("id = ?", req.UserID).
				Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.ProfileChangeRequest{}).Where("id = ?", requestID).
			Updates(map[string]any{
				"status":        status,
				"reject_reason": reason,
				"reviewed_by":   reviewerID,
				"reviewed_at":   now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		// 审核结果以站内信通知学员（与审核状态流转同事务，避免通知丢失）
		// 通知的构造与写入统一走站内信模块
		typ, title, content := s.notificationSvc.ProfileReviewNotification(&req, status, reason)
		return s.notificationSvc.CreateWithTx(tx, req.UserID, typ, title, content, "", now)
	})
	if err != nil {
		return nil, err
	}

	// 头像文件生命周期（与状态流转同处）：审核通过清理被替换的旧头像，驳回清理待审文件。
	// 尽力而为：文件删除失败不阻断审核结果返回。
	s.cleanupAvatarFiles(&req, status)

	req.Status = status
	req.RejectReason = reason
	req.ReviewedBy = &reviewerID
	req.ReviewedAt = &now
	var user model.HrwaiUser
	if err := s.db.First(&user, req.UserID).Error; err != nil {
		return nil, err
	}
	return s.toDTO(&req, &user), nil
}

// cleanupAvatarFiles 按审核结果清理头像文件（孤儿文件修复：approve 路径清理被替换旧头像）。
func (s *ProfileReviewService) cleanupAvatarFiles(req *model.ProfileChangeRequest, status string) {
	if s.storage == nil || req.FieldType != ProfileFieldAvatar {
		return
	}
	target := req.NewValue
	if status == ProfileStatusApproved {
		target = req.OldValue // 被替换的旧头像
	}
	if target == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.storage.Delete(ctx, target)
}

// toDTO 组装展示对象。
func (s *ProfileReviewService) toDTO(req *model.ProfileChangeRequest, user *model.HrwaiUser) *ProfileChangeRequestDTO {
	return &ProfileChangeRequestDTO{
		ID:           req.ID,
		UserID:       user.ID,
		Username:     user.Username,
		Name:         user.Name,
		Nickname:     user.Nickname,
		AvatarURL:    user.AvatarURL,
		FieldType:    req.FieldType,
		OldValue:     req.OldValue,
		NewValue:     req.NewValue,
		Status:       req.Status,
		RejectReason: req.RejectReason,
		ReviewedBy:   req.ReviewedBy,
		ReviewedAt:   formatTimePtr(req.ReviewedAt),
		CreatedAt:    formatISO(req.CreatedAt),
	}
}

// formatTimePtr 格式化时间指针，nil 返回 nil。
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatISO(*t)
	return &s
}
