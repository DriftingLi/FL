// 真题套卷 API（学员端）：列表 / 兑换 / 按卷练习 / 按卷考试（ADR-0022）。
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
)

// RealExamHandler 真题套卷 handler。
type RealExamHandler struct {
	svc    *service.RealExamService
	points *service.PointsService
}

// NewRealExamHandler 创建真题套卷 handler。
func NewRealExamHandler(svc *service.RealExamService, points *service.PointsService) *RealExamHandler {
	return &RealExamHandler{svc: svc, points: points}
}

// RegisterRealExamRoutes 注册 /api/real-exam 蓝图。
func RegisterRealExamRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.RealExamService, points *service.PointsService) {
	h := NewRealExamHandler(svc, points)

	g := rg.Group("/real-exam", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapRealExamTake), middleware.CredentialScoped(rd.CredentialScope))

	// GET /api/real-exam/papers  当前证件的套卷列表（含兑换状态与单价）
	g.GET("/papers", h.ListPapers)
	// POST /api/real-exam/papers/:paper_id/redeem  积分兑换单套卷
	g.POST("/papers/:paper_id/redeem", h.Redeem)
	// GET /api/real-exam/papers/:paper_id/practice  按卷练习开始/续练
	g.GET("/papers/:paper_id/practice", h.StartPractice)
	// POST /api/real-exam/papers/:paper_id/exam  按卷开考（复用模拟考链路）
	g.POST("/papers/:paper_id/exam", h.StartExam)
}

// listPapersReq 套卷列表请求（credential_id 由前端按当前证件注入）。
type listPapersReq struct {
	UserID       int
	CredentialID int
}

// ListPapers 套卷列表
// @Summary 真题套卷列表
// @Description 按当前证件返回上架套卷，附兑换状态与单价
// @Tags 学员端-真题
// @Produce json
// @Security BearerAuth
// @Param credential_id query int true "目标证件ID"
// @Success 200 {object} response.R{data=[]service.RealExamPaperDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /real-exam/papers [get]
func (h *RealExamHandler) ListPapers(c *gin.Context) {
	Endpoint[listPapersReq, []service.RealExamPaperDTO]{
		Parse: func(c *gin.Context) (*listPapersReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			return &listPapersReq{
				UserID:       userID,
				CredentialID: middleware.CredentialIDValue(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listPapersReq) (*[]service.RealExamPaperDTO, error) {
			result := h.svc.ListPapers(req.UserID, req.CredentialID)
			return &result, nil
		},
	}.Handle(c)
}

// paperActionReq 单卷操作请求（redeem/exam/practice 共用路径参数）。
type paperActionReq struct {
	UserID  int
	PaperID int
}

func parsePaperAction(c *gin.Context) (*paperActionReq, error) {
	id, ok := requiredPositiveID(c.Param("paper_id"))
	if !ok {
		return nil, badRequest("真题卷ID无效")
	}
	uid, _ := c.Get(string(middleware.CtxUserID))
	userID, _ := uid.(int)
	return &paperActionReq{UserID: userID, PaperID: id}, nil
}

// Redeem 积分兑换单套真题卷
// @Summary 兑换真题卷
// @Description 按套扣积分并写入权益，重复兑换报"已兑换"
// @Tags 学员端-真题
// @Produce json
// @Security BearerAuth
// @Param paper_id path int true "真题卷ID"
// @Success 200 {object} response.R{data=service.RedeemResult} "success"
// @Failure 400 {object} response.R "参数错误或积分不足"
// @Failure 401 {object} response.R "未认证"
// @Router /real-exam/papers/{paper_id}/redeem [post]
func (h *RealExamHandler) Redeem(c *gin.Context) {
	Endpoint[paperActionReq, service.RedeemResult]{
		Parse: func(c *gin.Context) (*paperActionReq, error) { return parsePaperAction(c) },
		Invoke: func(ctx context.Context, req *paperActionReq) (*service.RedeemResult, error) {
			return h.points.RedeemRealPaper(ctx, req.UserID, req.PaperID)
		},
		// 同域同判（ADR-0062 票9）：本端点抛的就是积分域那批哨兵（积分不足 / 已兑换 / 卷不可用），
		// 故直接复用 pointsErrStatus——旧写法 errStatusAll(404) 让同一事件在
		// /points/shop/course/:id/redeem 与这里给出两个码，消费端只能靠文案猜语义。
		ErrStatus: pointsErrStatus,
	}.Handle(c)
}

// StartPractice 按卷练习开始/续练
// @Summary 按卷练习
// @Description 返回卷内题目（固定卷序）与断点游标；未兑换拒绝
// @Tags 学员端-真题
// @Produce json
// @Security BearerAuth
// @Param paper_id path int true "真题卷ID"
// @Success 200 {object} response.R{data=service.PracticeStartResultDTO} "success"
// @Failure 400 {object} response.R "参数错误或未兑换"
// @Failure 401 {object} response.R "未认证"
// @Router /real-exam/papers/{paper_id}/practice [get]
func (h *RealExamHandler) StartPractice(c *gin.Context) {
	Endpoint[paperActionReq, service.PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*paperActionReq, error) { return parsePaperAction(c) },
		Invoke: func(ctx context.Context, req *paperActionReq) (*service.PracticeStartResultDTO, error) {
			return h.svc.StartPaperPractice(req.UserID, req.PaperID)
		},
		// A 批把这里登记为「正解在 service 侧升哨兵，升完再换表」——第②批 B 段做的正是那件事。
		// 三件事各归其位：卷不可用=404；未兑换=400（真题卷的存在性是公开的，这里不套
		// ADR-0062 决策 3 的「按不存在答」）；卷内无已发布题=400；其余（含查不动）=500。
		ErrStatus: &errStatusTable{entries: []errStatusEntry{
			{sentinel: service.ErrRealPaperUnavailable, status: http.StatusNotFound},
			{sentinel: service.ErrRealPaperNotRedeemed, status: http.StatusBadRequest},
			{sentinel: service.ErrRealPaperEmpty, status: http.StatusBadRequest},
			{sentinel: nil, status: http.StatusInternalServerError},
		}},
	}.Handle(c)
}

// StartExam 按卷开考
// @Summary 按卷开考
// @Description 固定题集+卷时长建 mock_exam 记录；之后的保存/交卷/结果复用 /mock-exam 端点
// @Tags 学员端-真题
// @Produce json
// @Security BearerAuth
// @Param paper_id path int true "真题卷ID"
// @Success 200 {object} response.R{data=service.MockExamStartDTO} "success"
// @Failure 400 {object} response.R "参数错误或未兑换"
// @Failure 401 {object} response.R "未认证"
// @Router /real-exam/papers/{paper_id}/exam [post]
func (h *RealExamHandler) StartExam(c *gin.Context) {
	Endpoint[paperActionReq, service.MockExamStartDTO]{
		Parse: func(c *gin.Context) (*paperActionReq, error) { return parsePaperAction(c) },
		Invoke: func(ctx context.Context, req *paperActionReq) (*service.MockExamStartDTO, error) {
			return h.svc.StartPaperExam(req.UserID, req.PaperID)
		},
		// 与 StartPractice 同一判定（同一张表、同一理由，见 StartPractice 处注释）。
		ErrStatus: &errStatusTable{entries: []errStatusEntry{
			{sentinel: service.ErrRealPaperUnavailable, status: http.StatusNotFound},
			{sentinel: service.ErrRealPaperNotRedeemed, status: http.StatusBadRequest},
			{sentinel: service.ErrRealPaperEmpty, status: http.StatusBadRequest},
			{sentinel: nil, status: http.StatusInternalServerError},
		}},
	}.Handle(c)
}
