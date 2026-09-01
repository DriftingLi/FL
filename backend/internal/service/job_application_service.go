// Package service 招聘域：投递（spec #449 T3 #452）。
// 本文件当前为 T2 阶段的装配桩：T3 #452 填充「投递即授权」完整语义。
package service

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// JobApplicationService 投递服务（T3 #452 实现）。
type JobApplicationService struct {
	db              *gorm.DB
	logger          *zap.Logger
	notificationSvc *NotificationService
}

// NewJobApplicationService 创建投递服务。
func NewJobApplicationService(db *gorm.DB, logger *zap.Logger, notificationSvc *NotificationService) *JobApplicationService {
	return &JobApplicationService{db: db, logger: logger, notificationSvc: notificationSvc}
}
