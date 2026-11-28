package valuation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// TrialAdminHandler 管理试算开关、allowlist 与观测指标。
type TrialAdminHandler struct {
	store  TrialAdminStore
	logger *zap.Logger
}

// RegisterTrialAdminRoutes 注册仅管理员可用的试算控制面。
func RegisterTrialAdminRoutes(rg *gin.RouterGroup, session *security.Session, store TrialAdminStore, logger *zap.Logger) {
	h := &TrialAdminHandler{store: store, logger: logger}
	g := rg.Group("/valuation/admin/trials", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapValuationConfig))
	g.GET("/control", h.GetControl)
	g.PUT("/control", h.SetControl)
	g.GET("/policies", h.ListPolicies)
	g.PUT("/policies", h.ReplacePolicies)
	g.GET("/metrics", h.ListMetrics)
	g.GET("/metrics/export", h.ExportMetrics)
}

// @Summary 读取试算全局开关
// @Description 独立全局开关默认关闭；读取失败时试算按关闭处理。
// @Tags 估值-管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=TrialControlResponse}
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/control [get]
func (h *TrialAdminHandler) GetControl(c *gin.Context) {
	control, err := h.store.GetTrialControl(c.Request.Context())
	if err != nil {
		h.fail(c, "读取试算全局开关失败", err)
		return
	}
	response.Success(c, TrialControlResponse{TrialControl: control})
}

// @Summary 更新试算全局开关
// @Description 仅影响 trials，不影响草稿、建议、手动或正式评估；enabled 缺省为 false。
// @Tags 估值-管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body TrialControlRequest true "全局开关"
// @Success 200 {object} response.R{data=TrialControlResponse}
// @Failure 400 {object} response.R
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/control [put]
func (h *TrialAdminHandler) SetControl(c *gin.Context) {
	var req TrialControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	control, err := h.store.SetTrialControl(c.Request.Context(), req.Enabled)
	if err != nil {
		h.fail(c, "更新试算全局开关失败", err)
		return
	}
	response.Success(c, TrialControlResponse{TrialControl: control})
}

// @Summary 查询试算获批策略
// @Tags 估值-管理
// @Produce json
// @Security BearerAuth
// @Param policy_name query string false "demo 或 pilot"
// @Param brand query string false "品牌"
// @Param vehicle_type query string false "车型"
// @Param tonnage query number false "吨位"
// @Success 200 {object} response.R{data=TrialPoliciesResponse}
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/policies [get]
func (h *TrialAdminHandler) ListPolicies(c *gin.Context) {
	filter := TrialPolicyFilter{PolicyName: c.Query("policy_name"), Brand: c.Query("brand"), VehicleType: c.Query("vehicle_type")}
	if raw := c.Query("tonnage"); raw != "" {
		tonnage, err := strconv.ParseFloat(raw, 64)
		if err != nil || tonnage <= 0 {
			response.BadRequest(c, "tonnage 必须为正数")
			return
		}
		filter.Tonnage = &tonnage
	}
	policies, err := h.store.ListTrialPolicies(c.Request.Context(), filter)
	if err != nil {
		h.fail(c, "查询试算策略失败", err)
		return
	}
	response.Success(c, TrialPoliciesResponse{Policies: policies})
}

// @Summary 全量更新试算获批策略
// @Description 空列表清空 allowlist；策略版本按替换内容重新计算并生成策略哈希。
// @Tags 估值-管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body TrialPoliciesRequest true "获批品牌×车型×吨位组合与情景"
// @Success 200 {object} response.R{data=TrialPoliciesResponse}
// @Failure 400 {object} response.R
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/policies [put]
func (h *TrialAdminHandler) ReplacePolicies(c *gin.Context) {
	var req TrialPoliciesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	if req.Policies == nil {
		response.BadRequest(c, "policies 不能为 null")
		return
	}
	if err := validateTrialPolicySet(req.Policies); err != nil {
		response.BadRequest(c, "试算策略参数无效: "+err.Error())
		return
	}
	if err := h.store.ReplaceTrialPolicies(c.Request.Context(), req.Policies); err != nil {
		h.logger.Error("试算策略更新失败", zap.Error(err))
		h.fail(c, "更新试算策略失败", err)
		return
	}
	policies, err := h.store.ListTrialPolicies(c.Request.Context(), TrialPolicyFilter{})
	if err != nil {
		h.fail(c, "读取更新后的试算策略失败", err)
		return
	}
	response.Success(c, TrialPoliciesResponse{Policies: policies})
}

// @Summary 查询试算观测指标
// @Tags 估值-管理
// @Produce json
// @Security BearerAuth
// @Param from query string false "RFC3339 开始时间"
// @Param to query string false "RFC3339 结束时间"
// @Param policy_name query string false "策略名"
// @Param brand query string false "品牌"
// @Param vehicle_type query string false "车型"
// @Param tonnage query number false "吨位"
// @Param scenario query string false "complete、masked 或 year_range"
// @Param formula_hash query string false "公式哈希"
// @Param policy_version query string false "策略版本"
// @Param limit query int false "返回组数，1-1000，默认 200"
// @Success 200 {object} response.R{data=TrialMetricsResponse}
// @Failure 400 {object} response.R
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/metrics [get]
func (h *TrialAdminHandler) ListMetrics(c *gin.Context) {
	filter, err := parseTrialMetricFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, err := h.store.ListTrialMetrics(c.Request.Context(), filter)
	if err != nil {
		h.fail(c, "查询试算指标失败", err)
		return
	}
	response.Success(c, TrialMetricsResponse{Items: items})
}

// @Summary 导出试算观测指标 JSON
// @Tags 估值-管理
// @Produce json
// @Security BearerAuth
// @Param from query string false "RFC3339 开始时间"
// @Param to query string false "RFC3339 结束时间"
// @Param policy_name query string false "策略名"
// @Param brand query string false "品牌"
// @Param vehicle_type query string false "车型"
// @Param tonnage query number false "吨位"
// @Param scenario query string false "complete、masked 或 year_range"
// @Param formula_hash query string false "公式哈希"
// @Param policy_version query string false "策略版本"
// @Param limit query int false "返回组数，1-1000，默认 200"
// @Success 200 {array} TrialMetricSummary "JSON 附件"
// @Failure 400 {object} response.R
// @Failure 401 {object} response.R
// @Failure 403 {object} response.R
// @Failure 500 {object} response.R
// @Router /valuation/admin/trials/metrics/export [get]
func (h *TrialAdminHandler) ExportMetrics(c *gin.Context) {
	filter, err := parseTrialMetricFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, err := h.store.ListTrialMetrics(c.Request.Context(), filter)
	if err != nil {
		h.fail(c, "导出试算指标失败", err)
		return
	}
	payload, err := json.Marshal(items)
	if err != nil {
		h.fail(c, "序列化试算指标失败", err)
		return
	}
	c.Header("Content-Disposition", contentDisposition("valuation-trial-metrics.json"))
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

func parseTrialMetricFilter(c *gin.Context) (TrialMetricFilter, error) {
	filter := TrialMetricFilter{
		From: c.Query("from"), To: c.Query("to"), PolicyName: c.Query("policy_name"),
		Brand: c.Query("brand"), VehicleType: c.Query("vehicle_type"), Scenario: c.Query("scenario"),
		FormulaHash: c.Query("formula_hash"), PolicyVersion: c.Query("policy_version"), Limit: 200,
	}
	for name, value := range map[string]string{"from": filter.From, "to": filter.To} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return filter, fmt.Errorf("%s 必须为 RFC3339 时间", name)
			}
		}
	}
	if filter.From != "" && filter.To != "" {
		from, _ := time.Parse(time.RFC3339, filter.From)
		to, _ := time.Parse(time.RFC3339, filter.To)
		if from.After(to) {
			return filter, fmt.Errorf("from 不能晚于 to")
		}
	}
	if raw := c.Query("tonnage"); raw != "" {
		tonnage, err := strconv.ParseFloat(raw, 64)
		if err != nil || tonnage <= 0 {
			return filter, fmt.Errorf("tonnage 必须为正数")
		}
		filter.Tonnage = &tonnage
	}
	if filter.Scenario != "" && filter.Scenario != "complete" && filter.Scenario != "masked" && filter.Scenario != "year_range" {
		return filter, fmt.Errorf("scenario 必须为 complete、masked 或 year_range")
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 1000 {
			return filter, fmt.Errorf("limit 必须在 1-1000 内")
		}
		filter.Limit = limit
	}
	return filter, nil
}

func (h *TrialAdminHandler) fail(c *gin.Context, message string, err error) {
	if h.store == nil {
		response.ServerError(c, "试算管理存储未装配")
		return
	}
	h.logger.Error(message, zap.Error(err))
	response.ServerError(c, message)
}
