// Package service 招聘域：职位举报与强制下架（spec #449 T5 #454）。
// 本文件当前为 T2 阶段的装配桩：T5 #454 填充举报/下架完整语义。
package service

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// JobReportService 职位举报服务（T5 #454 实现）。
type JobReportService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewJobReportService 创建职位举报服务。
func NewJobReportService(db *gorm.DB, logger *zap.Logger) *JobReportService {
	return &JobReportService{db: db, logger: logger}
}
