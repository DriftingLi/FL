// Package api 管理端巡检视图（#376）。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterAdminInspectionRoutes 注册管理端巡检相关路由（#376）。
// pointsSvc 按需注入：积分流水查询归位 service 层（#401），handler 不再裸查 PointsLedger。
func RegisterAdminInspectionRoutes(rg *gin.RouterGroup, rd RouterDeps, db *gorm.DB, pointsSvc *service.PointsService) {
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
	// 问答积分流水按原因筛选（admin 全量；查询归位 PointsService.GetLedger，#401）
	g.GET("/points/ledger", func(c *gin.Context) {
		reason := c.Query("reason")
		// #411：按业务域（ref_type）过滤；不传 = 跨域全量（管理员知情切换）
		refType := c.Query("ref_type")
		// user_id 非法/缺省 → 0 = 不过滤用户
		userID := atoiDefault(c.Query("user_id"), 0)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		res, err := pointsSvc.GetLedger(userID, page, pageSize, reason, refType)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, res)
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
