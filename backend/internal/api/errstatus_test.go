// 域级哨兵→状态码表（#610/#611）测试：
//   - 骨架行为：查表命中 / 未命中走 fallback / 未命中无 fallback 走 500 / 自定义 Render 覆盖域表 /
//     ParseError 优先于域表 / wrap 错误以 errors.Is 命中
//   - 每域表内容快照：钉住哨兵身份、状态码、条数、顺序与 fallback，防漂移
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
)

// 测试哨兵（与域表无交集，专测骨架行为）。
var (
	errSentinelA = errors.New("测试哨兵A")
	errSentinelB = errors.New("测试哨兵B")
)

// testTable 命中 A → 404，其余兜底 400。
var testTable = &errStatusTable{
	entries:  []errStatusEntry{{errSentinelA, http.StatusNotFound}},
	fallback: http.StatusBadRequest,
}

// TestEndpointErrStatus_TableHit errors.Is 命中表内哨兵 → 表内状态码 + err.Error() 文案。
func TestEndpointErrStatus_TableHit(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, fmt.Errorf("service wrap: %w", errSentinelA) // wrap 后仍须命中
		},
		ErrStatus: testTable,
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404（表内哨兵状态码）", w.Code)
	}
	if !strings.Contains(w.Body.String(), "测试哨兵A") {
		t.Fatalf("文案应为 err.Error(): %s", w.Body.String())
	}
}

// TestEndpointErrStatus_TableMiss_Fallback 未命中 → fallback 状态码。
func TestEndpointErrStatus_TableMiss_Fallback(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, errSentinelB
		},
		ErrStatus: testTable,
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400（fallback）", w.Code)
	}
}

// TestEndpointErrStatus_TableMiss_NoFallback_500 未命中且未设 fallback → 500 默认信封。
func TestEndpointErrStatus_TableMiss_NoFallback_500(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, errSentinelB
		},
		ErrStatus: &errStatusTable{entries: []errStatusEntry{{errSentinelA, http.StatusNotFound}}},
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500（未设 fallback 走默认）", w.Code)
	}
}

// TestEndpointErrStatus_CustomRender_OverridesTable 自定义 Render 优先级高于域表：
// 表会判 404，但 Render 全权渲染时域表不生效。
func TestEndpointErrStatus_CustomRender_OverridesTable(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, errSentinelA // 命中表内 404，但 Render 覆盖之
		},
		Render: func(c *gin.Context, _ *int, _ *string, err error) {
			c.JSON(http.StatusTeapot, map[string]any{"code": 418, "message": err.Error()})
		},
		ErrStatus: testTable,
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusTeapot {
		t.Fatalf("状态码 = %d, 期望 418（自定义 Render 覆盖域表）", w.Code)
	}
}

// TestEndpointErrStatus_ParseError_PrecedesTable 解析错误优先于域表：
// ParseError 404 不得被 fallback 400 吞掉。
func TestEndpointErrStatus_ParseError_PrecedesTable(t *testing.T) {
	e := Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &ParseError{Status: http.StatusNotFound, Message: "路径参数无效"}
		},
		ErrStatus: testTable,
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404（ParseError 优先于表）", w.Code)
	}
}

// assertTableSnapshot 钉住域表全集：条数、顺序、哨兵身份、状态码、fallback。
// 表内容变更时本测试即红——须有意识地同步更新快照（防状态码语义漂移）。
func assertTableSnapshot(t *testing.T, name string, got *errStatusTable, want []errStatusEntry, wantFallback int) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s 表不存在", name)
	}
	if got.fallback != wantFallback {
		t.Fatalf("%s fallback = %d, 期望 %d", name, got.fallback, wantFallback)
	}
	if len(got.entries) != len(want) {
		t.Fatalf("%s 条目数 = %d, 期望 %d（表内容漂移，须同步快照）", name, len(got.entries), len(want))
	}
	for i, w := range want {
		g := got.entries[i]
		if g.sentinel != w.sentinel {
			t.Fatalf("%s entries[%d] 哨兵身份漂移: got %v, want %v", name, i, g.sentinel, w.sentinel)
		}
		if g.status != w.status {
			t.Fatalf("%s entries[%d] %v 状态码 = %d, 期望 %d", name, i, g.sentinel, g.status, w.status)
		}
	}
}

// TestErrStatusTable_Snapshot_Points 积分域表快照（#610：已领取类 400、不存在类 404、兜底 400）。
func TestErrStatusTable_Snapshot_Points(t *testing.T) {
	assertTableSnapshot(t, "pointsErrStatus", pointsErrStatus, []errStatusEntry{
		{service.ErrTaskNotFound, http.StatusNotFound},
		{service.ErrCourseNotFound, http.StatusBadRequest},
		{service.ErrCourseNotRedeemable, http.StatusBadRequest},
		{service.ErrAlreadyClaimed, http.StatusBadRequest},
		{service.ErrDailyClaimLimit, http.StatusBadRequest},
		{service.ErrInsufficientPoints, http.StatusBadRequest},
		{service.ErrAlreadyRedeemed, http.StatusBadRequest},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_Contribution 投稿域表快照（#611：不存在 404、其余 400、未设 fallback → 500）。
func TestErrStatusTable_Snapshot_Contribution(t *testing.T) {
	assertTableSnapshot(t, "contributionErrStatus", contributionErrStatus, []errStatusEntry{
		{service.ErrContributionNotFound, http.StatusNotFound},
		{service.ErrContributionNotOwner, http.StatusBadRequest},
		{service.ErrContributionNotPending, http.StatusBadRequest},
		{service.ErrContributionNotApproved, http.StatusBadRequest},
		{service.ErrContributionQuotaDaily, http.StatusBadRequest},
		{service.ErrContributionQuotaPending, http.StatusBadRequest},
		{service.ErrContributionNoCredential, http.StatusBadRequest},
		{service.ErrContributionTitleRequired, http.StatusBadRequest},
		{service.ErrContributionIntroRequired, http.StatusBadRequest},
		{service.ErrContributionFilesRequired, http.StatusBadRequest},
		{service.ErrContributionFilesTooMany, http.StatusBadRequest},
		{service.ErrContributionFileTooLarge, http.StatusBadRequest},
		{service.ErrContributionTotalTooLarge, http.StatusBadRequest},
		{service.ErrContributionFileInvalid, http.StatusBadRequest},
		{service.ErrContributionRejectReason, http.StatusBadRequest},
		{service.ErrContributionArchiveReason, http.StatusBadRequest},
		{service.ErrContributionInvalidReportReason, http.StatusBadRequest},
	}, 0)
}

// TestErrStatusTable_Snapshot_Application 投递域表快照（#611）。
func TestErrStatusTable_Snapshot_Application(t *testing.T) {
	assertTableSnapshot(t, "applicationErrStatus", applicationErrStatus, []errStatusEntry{
		{service.ErrApplyJobInactive, http.StatusNotFound},
		{service.ErrJobNotFound, http.StatusNotFound},
		{service.ErrApplyNotYours, http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_JobReport 举报治理域表快照（#611）。
func TestErrStatusTable_Snapshot_JobReport(t *testing.T) {
	assertTableSnapshot(t, "jobReportErrStatus", jobReportErrStatus, []errStatusEntry{
		{service.ErrReportJobNotFound, http.StatusNotFound},
		{service.ErrReportNotFound, http.StatusNotFound},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_RecruiterApplication 企业侧投递域表快照（#611）。
func TestErrStatusTable_Snapshot_RecruiterApplication(t *testing.T) {
	assertTableSnapshot(t, "recruiterApplicationErrStatus", recruiterApplicationErrStatus, []errStatusEntry{
		{service.ErrJobNotFound, http.StatusNotFound},
		{service.ErrApplyNotFound, http.StatusNotFound},
		{service.ErrApplyNotYours, http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_Job 职位域表快照（#611）。
func TestErrStatusTable_Snapshot_Job(t *testing.T) {
	assertTableSnapshot(t, "jobErrStatus", jobErrStatus, []errStatusEntry{
		{service.ErrJobNotFound, http.StatusNotFound},
		{service.ErrJobNotYours, http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_QuestionBank 题库域表快照（#611）。
func TestErrStatusTable_Snapshot_QuestionBank(t *testing.T) {
	assertTableSnapshot(t, "questionBankErrStatus", questionBankErrStatus, []errStatusEntry{
		{service.ErrQuestionNotFound, http.StatusNotFound},
	}, http.StatusBadRequest)
}
