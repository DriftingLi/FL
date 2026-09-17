// Package api 管理端巡检视图（#376）。
//
// 注解口径（#1097）：分页信封的运行时类型是 paging.ItemsPage[T]（#1095 的装配单点），
// 但 CI 钉住的 swag v1.16.4 展不开泛型实例化（注解里写 paging.ItemsPage[...] 会退化成空对象），
// 故两条列表端点的 Success 注解用内联 object{...} 声明信封字段、行类型仍 ref 到 service.*DTO。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// InspectionHandler 管理端巡检 handler（#1097：三条读路径归位 InspectionService，handler 只解析/渲染）。
// pointsSvc 按需注入：积分流水查询归位 service 层（#401），handler 不再裸查 PointsLedger。
type InspectionHandler struct {
	svc       *service.InspectionService
	pointsSvc *service.PointsService
}

// NewInspectionHandler 创建管理端巡检 handler。
func NewInspectionHandler(svc *service.InspectionService, pointsSvc *service.PointsService) *InspectionHandler {
	return &InspectionHandler{svc: svc, pointsSvc: pointsSvc}
}

// RegisterAdminInspectionRoutes 注册管理端巡检相关路由（#376）。
// 读路径全部经 service 出 typed DTO：api 层不再持有 *gorm.DB（ADR-0056 §3 静态守卫）。
func RegisterAdminInspectionRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.InspectionService, pointsSvc *service.PointsService) {
	h := NewInspectionHandler(svc, pointsSvc)
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapInspectionRead))
	// 巡检计数：删除已解决帖计数
	g.GET("/inspection/deleted-after-accepted", h.DeletedAfterAcceptedCount)
	// 问答积分流水按原因筛选（admin 全量；查询归位 PointsService.GetLedger，#401）
	g.GET("/points/ledger", h.PointsLedger)
	// 招聘企业账号的查看与申请记录（滥用收口靠禁用位）
	g.GET("/recruit/views", h.ListRecruitViews)
	g.GET("/recruit/requests", h.ListRecruitRequests)
}

// @Summary 巡检计数：删除已解决帖
// @Description 管理员查看「楼主删除自己已解决的帖子」累计计数（system_settings.deleted_after_accepted，行缺失 = 0）
// @Tags 管理端-巡检
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.InspectionCountDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/inspection/deleted-after-accepted [get]
// DeletedAfterAcceptedCount 巡检计数 GET /api/admin/inspection/deleted-after-accepted
func (h *InspectionHandler) DeletedAfterAcceptedCount(c *gin.Context) {
	Endpoint[struct{}, service.InspectionCountDTO]{
		Invoke: func(_ context.Context, _ *struct{}) (*service.InspectionCountDTO, error) {
			return h.svc.DeletedAfterAcceptedCount()
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.InspectionCountDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// pointsLedgerReq 积分流水查询参数（#411）。
type pointsLedgerReq struct {
	Page     int
	PageSize int
	UserID   int
	Reason   string
	RefType  string
}

// @Summary 积分流水（管理端）
// @Description 管理员按原因 / 业务域 / 用户筛选积分流水（不传 ref_type = 跨域全量；页大小上限 100）
// @Tags 管理端-巡检
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param reason query string false "积分原因（如 accepted_bonus / rollback）"
// @Param ref_type query string false "业务域（forum_topic / task / course / ai_chat 等）；不传 = 跨域全量"
// @Param user_id query int false "用户 ID（>0 生效）"
// @Success 200 {object} response.R{data=service.PointsLedgerResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/points/ledger [get]
// PointsLedger 问答积分流水 GET /api/admin/points/ledger?page=&page_size=&reason=&ref_type=&user_id=
func (h *InspectionHandler) PointsLedger(c *gin.Context) {
	Endpoint[pointsLedgerReq, service.PointsLedgerResult]{
		Parse: func(c *gin.Context) (*pointsLedgerReq, error) {
			return &pointsLedgerReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
				// user_id 非法/缺省 → 0 = 不过滤用户
				UserID: atoiDefault(c.Query("user_id"), 0),
				Reason: c.Query("reason"),
				// #411：按业务域（ref_type）过滤；不传 = 跨域全量（管理员知情切换）
				RefType: c.Query("ref_type"),
			}, nil
		},
		Invoke: func(_ context.Context, req *pointsLedgerReq) (*service.PointsLedgerResult, error) {
			return h.pointsSvc.GetLedger(req.UserID, req.Page, req.PageSize, req.Reason, req.RefType)
		},
		Render: func(c *gin.Context, _ *pointsLedgerReq, resp *service.PointsLedgerResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// @Summary 简历查看留痕列表
// @Description 管理员分页查询招聘企业查看学员简历的留痕（可按招聘者/学员过滤；只呈现事实字段，不含联系方式明文）
// @Tags 管理端-巡检
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param recruiter_id query int false "招聘者 ID（>0 生效）"
// @Param student_user_id query int false "学员用户 ID（>0 生效）"
// @Success 200 {object} response.R{data=object{items=[]service.RecruitResumeViewDTO,page=int,page_size=int,total=int}} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/recruit/views [get]
// ListRecruitViews 简历查看留痕列表 GET /api/admin/recruit/views
func (h *InspectionHandler) ListRecruitViews(c *gin.Context) {
	Endpoint[service.InspectionViewsParams, paging.ItemsPage[service.RecruitResumeViewDTO]]{
		Parse: func(c *gin.Context) (*service.InspectionViewsParams, error) {
			return &service.InspectionViewsParams{
				RecruiterID:  atoiDefault(c.Query("recruiter_id"), 0),
				ResumeUserID: atoiDefault(c.Query("student_user_id"), 0),
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 20),
			}, nil
		},
		Invoke: func(_ context.Context, req *service.InspectionViewsParams) (*paging.ItemsPage[service.RecruitResumeViewDTO], error) {
			return h.svc.ListRecruitViews(*req)
		},
		Render: func(c *gin.Context, _ *service.InspectionViewsParams, resp *paging.ItemsPage[service.RecruitResumeViewDTO], err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// @Summary 联系方式交换申请列表
// @Description 管理员分页查询联系方式交换申请（可按招聘者/学员/状态过滤）
// @Tags 管理端-巡检
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param recruiter_id query int false "招聘者 ID（>0 生效）"
// @Param student_user_id query int false "学员用户 ID（>0 生效）"
// @Param status query string false "申请状态（pending/approved/rejected/expired/revoked）"
// @Success 200 {object} response.R{data=object{items=[]service.ContactRequestRowDTO,page=int,page_size=int,total=int}} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/recruit/requests [get]
// ListRecruitRequests 联系方式交换申请列表 GET /api/admin/recruit/requests
func (h *InspectionHandler) ListRecruitRequests(c *gin.Context) {
	Endpoint[service.InspectionRequestsParams, paging.ItemsPage[service.ContactRequestRowDTO]]{
		Parse: func(c *gin.Context) (*service.InspectionRequestsParams, error) {
			return &service.InspectionRequestsParams{
				RecruiterID:   atoiDefault(c.Query("recruiter_id"), 0),
				StudentUserID: atoiDefault(c.Query("student_user_id"), 0),
				Status:        c.Query("status"),
				Page:          atoiDefault(c.Query("page"), 1),
				PageSize:      atoiDefault(c.Query("page_size"), 20),
			}, nil
		},
		Invoke: func(_ context.Context, req *service.InspectionRequestsParams) (*paging.ItemsPage[service.ContactRequestRowDTO], error) {
			return h.svc.ListRecruitRequests(*req)
		},
		Render: func(c *gin.Context, _ *service.InspectionRequestsParams, resp *paging.ItemsPage[service.ContactRequestRowDTO], err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}
