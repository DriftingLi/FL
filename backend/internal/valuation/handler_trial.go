package valuation

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/pkg/response"
)

// TrialSnapshotReader 一次读取并冻结目录、全部系数与显式策略。
type TrialSnapshotReader interface {
	LoadTrialSnapshot(ctx context.Context, req TrialRequest) (*TrialSnapshot, error)
}

// TrialHandler 参考试算 HTTP 出口；不依赖评估写入器或报告生成器。
type TrialHandler struct {
	reader    TrialSnapshotReader
	logger    *zap.Logger
	appEnv    string
	recorders []TrialMetricRecorder
}

func NewTrialHandler(reader TrialSnapshotReader, logger *zap.Logger, appEnv string, recorders ...TrialMetricRecorder) *TrialHandler {
	return &TrialHandler{reader: reader, logger: logger, appEnv: appEnv, recorders: recorders}
}

// RegisterTrialRoutes 注册默认关闭的参考试算入口。
func RegisterTrialRoutes(r *gin.Engine, reader TrialSnapshotReader, logger *zap.Logger, appEnv string, recorders ...TrialMetricRecorder) {
	r.POST("/api/valuation/trials", NewTrialHandler(reader, logger, appEnv, recorders...).Create)
}

// @Summary 参考试算
// @Description 使用同一冻结目录、系数快照与 V2 公式计算参考区间。全局开关默认关闭，仅显式 demo/pilot 且组合已配置时可用；不写 evaluations、不生成报告。
// @Tags 估值-评估
// @Accept json
// @Produce json
// @Param body body TrialRequest true "参考试算请求"
// @Success 200 {object} response.R{data=TrialResponse} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "参考试算未开放"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/trials [post]
func (h *TrialHandler) Create(c *gin.Context) {
	started := time.Now()
	var req TrialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误: "+err.Error())
		return
	}
	if prepared, _, ok := preflightTrial(req, nil, h.appEnv, false); !ok {
		h.recordMetrics(c, req, prepared, time.Since(started))
		if prepared.ReasonCode == TrialReasonPolicyDisabled || prepared.ReasonCode == TrialReasonGlobalDisabled || prepared.ReasonCode == TrialReasonPolicyNotConfigured || prepared.ReasonCode == TrialReasonPolicyForbidden {
			response.Forbidden(c, prepared.Message)
			return
		}
		response.Success(c, prepared)
		return
	}
	if h.reader == nil {
		h.logger.Error("参考试算快照读取器未装配")
		h.recordMetrics(c, req, trialNeedsInfo(newTrialResponse(req), TrialReasonTrialError, "参考试算暂不可用", nil), time.Since(started))
		response.ServerError(c, "参考试算暂不可用")
		return
	}
	snapshot, err := h.reader.LoadTrialSnapshot(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("读取参考试算快照失败", zap.Error(err))
		h.recordMetrics(c, req, trialNeedsInfo(newTrialResponse(req), TrialReasonTrialError, "参考试算快照读取失败", nil), time.Since(started))
		response.ServerError(c, "参考试算快照读取失败")
		return
	}
	result, err := CalculateTrial(c.Request.Context(), req, snapshot, h.appEnv)
	if err != nil {
		h.logger.Error("参考试算失败", zap.Error(err))
		h.recordMetrics(c, req, trialNeedsInfo(newTrialResponse(req), TrialReasonTrialError, "参考试算失败", nil), time.Since(started))
		response.ServerError(c, "参考试算失败")
		return
	}
	h.recordMetrics(c, req, result, time.Since(started))
	if result.ReasonCode == TrialReasonPolicyDisabled || result.ReasonCode == TrialReasonGlobalDisabled || result.ReasonCode == TrialReasonPolicyNotConfigured || result.ReasonCode == TrialReasonPolicyForbidden {
		response.Forbidden(c, result.Message)
		return
	}
	response.Success(c, result)
}

func (h *TrialHandler) recordMetrics(c *gin.Context, req TrialRequest, result TrialResponse, duration time.Duration) {
	if len(h.recorders) == 0 {
		return
	}
	event := NewTrialMetricEvent(req, result, duration.Milliseconds())
	for _, recorder := range h.recorders {
		if recorder == nil {
			continue
		}
		if err := recorder.RecordTrialMetric(c.Request.Context(), event); err != nil {
			h.logger.Error("记录参考试算指标失败", zap.Error(err))
		}
	}
}
