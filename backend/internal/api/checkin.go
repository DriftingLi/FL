// Package api 实现 HTTP handlers。
// 本文件：每日打卡独立蓝图 /api/check-in/*（ADR-0028：打卡从论坛域迁出独立模块；
// 旧 /api/forum/check-in/* 已删除，移动端适配见 GitHub #587）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// CheckInHandler 每日打卡 handler。
type CheckInHandler struct {
	svc *service.CheckInService
}

// NewCheckInHandler 创建打卡 handler。
func NewCheckInHandler(svc *service.CheckInService) *CheckInHandler {
	return &CheckInHandler{svc: svc}
}

// RegisterCheckInRoutes 注册 /api/check-in 蓝图（需登录，hrwai_user）。
func RegisterCheckInRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.CheckInService) {
	h := NewCheckInHandler(svc)
	g := rg.Group("/check-in", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// POST /api/check-in 签到（幂等；首签直记积分：基础 + 跨档阶梯）
	g.POST("", h.CheckIn)
	// GET /api/check-in/calendar?year=&month= 日历（逐日带实发分 points）
	g.GET("/calendar", h.GetCheckInCalendar)
	// GET /api/check-in/rank?page=&page_size= 排行榜
	g.GET("/rank", h.GetCheckInRank)
}

// CheckIn 每日打卡
// @Summary 每日打卡
// @Description Asia/Shanghai 每日一次；首签即发积分（基础 5 + 连击满 3/7/30 天阶梯 5/10/50），
// 返回连击/累计/今日实发分
// @Tags 学员端-每日打卡
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in [post]
func (h *CheckInHandler) CheckIn(c *gin.Context) {
	res, err := h.svc.CheckIn(middleware.CurrentUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "打卡成功", res)
}

// GetCheckInCalendar 打卡日历
// @Summary 打卡日历
// @Description 按年月查询打卡日历（逐日 {date, checked, points}）
// @Tags 学员端-每日打卡
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param year query int false "年份"
// @Param month query int false "月份 1-12"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in/calendar [get]
func (h *CheckInHandler) GetCheckInCalendar(c *gin.Context) {
	year := atoiDefault(c.Query("year"), 0)
	month := atoiDefault(c.Query("month"), 0)
	res, err := h.svc.GetCheckInCalendar(middleware.CurrentUserID(c), year, month)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// GetCheckInRank 打卡排行榜
// @Summary 打卡排行榜
// @Description 分页查询打卡排行榜
// @Tags 学员端-每日打卡
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in/rank [get]
func (h *CheckInHandler) GetCheckInRank(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	res, err := h.svc.GetCheckInRank(middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}
