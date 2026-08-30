// Package api 管理端巡检视图（#376）。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// RegisterAdminInspectionRoutes 注册管理端巡检相关路由（#376）。
func RegisterAdminInspectionRoutes(rg *gin.RouterGroup, rd RouterDeps, db *gorm.DB) {
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	// 巡检计数：删除已解决帖计数
	g.GET("/inspection/deleted-after-accepted", func(c *gin.Context) {
		var setting model.SystemSetting
		if err := db.Where("key = ?", "deleted_after_accepted").First(&setting).Error; err != nil {
			response.Success(c, gin.H{"count": 0})
			return
		}
		v, _ := strconv.Atoi(setting.Value)
		response.Success(c, gin.H{"count": v})
	})
	// 问答积分流水按原因筛选（admin 全量）
	g.GET("/points/ledger", func(c *gin.Context) {
		reason := c.Query("reason")
		userIDStr := c.Query("user_id")
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		if pageSize > 100 {
			pageSize = 100
		}
		q := db.Model(&model.PointsLedger{})
		if reason != "" {
			q = q.Where("reason = ?", reason)
		}
		if userIDStr != "" {
			if uid, err := strconv.Atoi(userIDStr); err == nil && uid > 0 {
				q = q.Where("user_id = ?", uid)
			}
		}
		var total int64
		if err := q.Count(&total).Error; err != nil {
			response.ServerError(c, err.Error())
			return
		}
		var rows []model.PointsLedger
		if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
	})
	// 招聘企业账号的查看与申请记录（滥用收口靠禁用位）
	g.GET("/recruit/views", func(c *gin.Context) {
		recruiterIDStr := c.Query("recruiter_id")
		studentIDStr := c.Query("student_user_id")
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		q := db.Model(&model.RecruitResumeView{})
		if recruiterIDStr != "" {
			if id, err := strconv.Atoi(recruiterIDStr); err == nil && id > 0 {
				q = q.Where("recruiter_id = ?", id)
			}
		}
		if studentIDStr != "" {
			if id, err := strconv.Atoi(studentIDStr); err == nil && id > 0 {
				q = q.Where("resume_user_id = ?", id)
			}
		}
		var total int64
		_ = q.Count(&total).Error
		var rows []model.RecruitResumeView
		_ = q.Order("viewed_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
		response.Success(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
	})
	g.GET("/recruit/requests", func(c *gin.Context) {
		recruiterIDStr := c.Query("recruiter_id")
		studentIDStr := c.Query("student_user_id")
		status := c.Query("status")
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		q := db.Model(&model.ContactRequest{})
		if recruiterIDStr != "" {
			if id, err := strconv.Atoi(recruiterIDStr); err == nil && id > 0 {
				q = q.Where("recruiter_id = ?", id)
			}
		}
		if studentIDStr != "" {
			if id, err := strconv.Atoi(studentIDStr); err == nil && id > 0 {
				q = q.Where("student_user_id = ?", id)
			}
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		var total int64
		_ = q.Count(&total).Error
		var rows []model.ContactRequest
		_ = q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
		response.Success(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
	})
}
