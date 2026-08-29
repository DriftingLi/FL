// Package api 实现 HTTP handlers。
// 本文件：企业招聘端受保护接口（第四角色 recruiter，host-only cookie 隔离，返回空列表占位）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/response"
)

// RegisterRecruitRoutes 注册 /api/recruit 蓝图（企业招聘者工作区，角色守卫）。
func RegisterRecruitRoutes(rg *gin.RouterGroup, rd RouterDeps) {
	g := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	g.GET("/resumes", listRecruitResumes)
	g.GET("/me", recruitMe)
}

// listRecruitResumes 招聘端简历列表（占位，返回空列表） GET /api/recruit/resumes
func listRecruitResumes(c *gin.Context) {
	response.Success(c, map[string]any{"items": []any{}, "total": 0})
}

// recruitMe 招聘者当前用户信息 GET /api/recruit/me（复用 /auth/me 的 ProfileDTO 形状，但仅 recruiter 可访问）
func recruitMe(c *gin.Context) {
	response.Success(c, map[string]any{
		"user_id": middleware.CurrentUserID(c),
		"account": middleware.CurrentAccount(c),
		"role":    middleware.CurrentRole(c),
	})
}
