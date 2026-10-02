// Package inspection 管理端巡检视图（#376）。
//
// 注解口径（#1097）：分页信封的运行时类型是 paging.ItemsPage[T]（#1095 的装配单点），
// 但 CI 钉住的 swag v1.16.4 展不开泛型实例化（注解里写 paging.ItemsPage[...] 会退化成空对象），
// 故两条列表端点的 Success 注解用内联 object{...} 声明信封字段、行类型仍 ref 到 inspection.*DTO。
package inspection

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/paging"
)

// handler 管理端巡检 handler（#1097：三条读路径归位域实现，handler 只解析/渲染）。
// pointsSvc 按需注入：积分流水查询归位积分域（#401），handler 不再裸查 PointsLedger。
type handler struct {
	svc       *Service
	pointsSvc *points.Service
}

// newHandler 创建管理端巡检 handler。
func newHandler(svc *Service, pointsSvc *points.Service) *handler {
	return &handler{svc: svc, pointsSvc: pointsSvc}
}

// RegisterRoutes 注册管理端巡检相关路由（#376）。
// 读路径全部经域实现出 typed DTO：handler 不再持有 *gorm.DB（ADR-0056 §3 静态守卫）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, pointsSvc *points.Service) {
	h := newHandler(svc, pointsSvc)
	g := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapInspectionRead))
	// 巡检计数：删除已解决帖计数
	g.GET("/inspection/deleted-after-accepted", h.DeletedAfterAcceptedCount)
	// 问答积分流水按原因筛选（admin 全量；查询归位 points.Service.GetLedger，#401）
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
// @Success 200 {object} response.R{data=inspection.InspectionCountDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/inspection/deleted-after-accepted [get]
// DeletedAfterAcceptedCount 巡检计数 GET /api/admin/inspection/deleted-after-accepted
func (h *handler) DeletedAfterAcceptedCount(c *gin.Context) {
	httpx.Endpoint[struct{}, InspectionCountDTO]{
		Invoke: func(_ context.Context, _ *struct{}) (*InspectionCountDTO, error) {
			return h.svc.DeletedAfterAcceptedCount()
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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
// @Success 200 {object} response.R{data=points.PointsLedgerResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/points/ledger [get]
// PointsLedger 问答积分流水 GET /api/admin/points/ledger?page=&page_size=&reason=&ref_type=&user_id=
func (h *handler) PointsLedger(c *gin.Context) {
	httpx.Endpoint[pointsLedgerReq, points.PointsLedgerResult]{
		Parse: func(c *gin.Context) (*pointsLedgerReq, error) {
			return &pointsLedgerReq{
				Page:     httpx.QueryIntDefault(c, "page", 1),
				PageSize: httpx.QueryIntDefault(c, "page_size", 20),
				// user_id 非法/缺省 → 0 = 不过滤用户
				UserID: httpx.QueryIntDefault(c, "user_id", 0),
				Reason: c.Query("reason"),
				// #411：按业务域（ref_type）过滤；不传 = 跨域全量（管理员知情切换）
				RefType: c.Query("ref_type"),
			}, nil
		},
		Invoke: func(_ context.Context, req *pointsLedgerReq) (*points.PointsLedgerResult, error) {
			return h.pointsSvc.GetLedger(req.UserID, req.Page, req.PageSize, req.Reason, req.RefType)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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
// @Success 200 {object} response.R{data=object{items=[]inspection.RecruitResumeViewDTO,page=int,page_size=int,total=int}} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/recruit/views [get]
// ListRecruitViews 简历查看留痕列表 GET /api/admin/recruit/views
func (h *handler) ListRecruitViews(c *gin.Context) {
	httpx.Endpoint[InspectionViewsParams, paging.ItemsPage[RecruitResumeViewDTO]]{
		Parse: func(c *gin.Context) (*InspectionViewsParams, error) {
			return &InspectionViewsParams{
				RecruiterID:  httpx.QueryIntDefault(c, "recruiter_id", 0),
				ResumeUserID: httpx.QueryIntDefault(c, "student_user_id", 0),
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 20),
			}, nil
		},
		Invoke: func(_ context.Context, req *InspectionViewsParams) (*paging.ItemsPage[RecruitResumeViewDTO], error) {
			return h.svc.ListRecruitViews(*req)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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
// @Success 200 {object} response.R{data=object{items=[]inspection.ContactRequestRowDTO,page=int,page_size=int,total=int}} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/recruit/requests [get]
// ListRecruitRequests 联系方式交换申请列表 GET /api/admin/recruit/requests
func (h *handler) ListRecruitRequests(c *gin.Context) {
	httpx.Endpoint[InspectionRequestsParams, paging.ItemsPage[ContactRequestRowDTO]]{
		Parse: func(c *gin.Context) (*InspectionRequestsParams, error) {
			return &InspectionRequestsParams{
				RecruiterID:   httpx.QueryIntDefault(c, "recruiter_id", 0),
				StudentUserID: httpx.QueryIntDefault(c, "student_user_id", 0),
				Status:        c.Query("status"),
				Page:          httpx.QueryIntDefault(c, "page", 1),
				PageSize:      httpx.QueryIntDefault(c, "page_size", 20),
			}, nil
		},
		Invoke: func(_ context.Context, req *InspectionRequestsParams) (*paging.ItemsPage[ContactRequestRowDTO], error) {
			return h.svc.ListRecruitRequests(*req)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}
