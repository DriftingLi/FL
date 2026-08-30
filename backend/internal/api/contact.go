// Package api 联系方式交换闭环（#375）。
package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterContactRoutes 注册联系方式交换相关路由（#375）。
func RegisterContactRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ContactService) {
	// 招聘方：发起与查看我的申请 + 读取明文
	recruitG := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	recruitG.POST("/contact-requests", func(c *gin.Context) {
		var body struct {
			StudentUserID int    `json:"student_user_id"`
			Message       string `json:"message"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		recruiterID := middleware.CurrentUserID(c)
		dto, err := svc.Create(recruiterID, body.StudentUserID, body.Message)
		if err != nil {
			// pending 唯一时返回既有状态（code 400 但带 data）
			if dto != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "申请已提交", dto)
	})
	recruitG.GET("/contact-requests", func(c *gin.Context) {
		recruiterID := middleware.CurrentUserID(c)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		items, total, err := svc.ListForRecruiter(recruiterID, page, pageSize)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
	})
	recruitG.GET("/resumes/:id/contact", func(c *gin.Context) {
		idStr := c.Param("id")
		uid, err := strconv.Atoi(idStr)
		if err != nil || uid <= 0 {
			response.BadRequest(c, "学员 ID 无效")
			return
		}
		recruiterID := middleware.CurrentUserID(c)
		dto, err := svc.GetContact(recruiterID, uid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "无有效授权" || err.Error() == "学员不存在或已注销" {
				response.Forbidden(c, err.Error())
				return
			}
			response.BadRequest(c, err.Error())
			return
		}
		// 仅返回明文字段（phone/wechat/real_name/pdf），不返回其他已脱敏字段（保持 L2 口径）
		response.Success(c, gin.H{
			"real_name":       dto.RealName,
			"contact_phone":   dto.ContactPhone,
			"wechat":          dto.Wechat,
			"resume_file_url": dto.ResumeFileURL,
		})
	})

	// 学员侧：查看收到的申请 + 同意/拒绝/撤回
	studentG := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.GET("/contact-requests", func(c *gin.Context) {
		studentID := middleware.CurrentUserID(c)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		items, total, err := svc.ListForStudent(studentID, page, pageSize)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
	})
	studentG.POST("/contact-requests/:id/approve", func(c *gin.Context) {
		id, err := pathInt64(c, "id", "申请 ID 无效")
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		studentID := middleware.CurrentUserID(c)
		dto, err := svc.Approve(studentID, id)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, dto)
	})
	studentG.POST("/contact-requests/:id/reject", func(c *gin.Context) {
		id, err := pathInt64(c, "id", "申请 ID 无效")
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		studentID := middleware.CurrentUserID(c)
		dto, err := svc.Reject(studentID, id)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, dto)
	})
	studentG.POST("/contact-requests/:id/revoke", func(c *gin.Context) {
		id, err := pathInt64(c, "id", "申请 ID 无效")
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		studentID := middleware.CurrentUserID(c)
		dto, err := svc.Revoke(studentID, id)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, dto)
	})
}
