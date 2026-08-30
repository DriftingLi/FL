// Package api 学员侧简历查看留痕聚合（#374）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterResumeViewRoutes 注册学员侧查看聚合 GET /api/resume/view-stats（#374）。
// 仅学员可访问：招聘方无法读取留痕数据（避免暴露浏览习惯）。
func RegisterResumeViewRoutes(rg *gin.RouterGroup, rd RouterDeps, recruitSvc *service.RecruitService) {
	g := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	g.GET("/view-stats", func(c *gin.Context) {
		uid := middleware.CurrentUserID(c)
		cnt, err := recruitSvc.StudentViewStats(uid)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		// 仅返回聚合数，不含企业名与任何身份信息
		response.Success(c, gin.H{"count": cnt})
	})
}
