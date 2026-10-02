// Package checkin 每日打卡独立蓝图 /api/check-in/*（ADR-0028；旧 /api/forum/check-in/* 已删除，移动端适配见 GitHub #587）。
package checkin

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 每日打卡 handler。
type handler struct {
	svc *Service
}

// newHandler 创建打卡 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/check-in 蓝图（需登录，hrwai_user）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)
	g := rg.Group("/check-in", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapCheckInUse))

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
// @Success 200 {object} response.R{data=checkin.CheckInResult} "打卡结果"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in [post]
func (h *handler) CheckIn(c *gin.Context) {
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
// @Success 200 {object} response.R{data=checkin.CheckInCalendarResult} "日历"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in/calendar [get]
func (h *handler) GetCheckInCalendar(c *gin.Context) {
	year := httpx.QueryIntDefault(c, "year", 0)
	month := httpx.QueryIntDefault(c, "month", 0)
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
// @Success 200 {object} response.R{data=checkin.CheckInRankResult} "排行榜"
// @Failure 401 {object} response.R "未认证"
// @Router /check-in/rank [get]
func (h *handler) GetCheckInRank(c *gin.Context) {
	page := httpx.QueryIntDefault(c, "page", 1)
	pageSize := httpx.QueryIntDefault(c, "page_size", 20)
	res, err := h.svc.GetCheckInRank(middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}
