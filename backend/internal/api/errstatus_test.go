// 域级哨兵→状态码表（#610/#611；票1b 起为端点错误面唯一出口）测试：
//   - 骨架行为：查表命中 / 未命中走 fallback / 未命中无 fallback 走 500 / Render 只写成功面 /
//     ParseError 优先于域表 / **ParseError 优先于无条件条目**（ADR-0062 票8 翻转）/ 固定文案槽 /
//     wrap 错误以 errors.Is 命中
//   - 每域表内容快照：钉住哨兵身份、状态码、固定文案、条数、顺序与 fallback，防漂移
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/contribution"
	"forklift-training/internal/forum"
	"forklift-training/internal/model"
	"forklift-training/internal/points"
	"forklift-training/internal/service"
	"forklift-training/pkg/httpx"
)

// 测试哨兵（与域表无交集，专测骨架行为）。
var (
	errSentinelA = errors.New("测试哨兵A")
	errSentinelB = errors.New("测试哨兵B")
)

// testTable 命中 A → 404，其余兜底 400。
var testTable = &httpx.ErrStatusTable{
	Entries:  []httpx.ErrStatusEntry{{Sentinel: errSentinelA, Status: http.StatusNotFound}},
	Fallback: http.StatusBadRequest,
}

// TestEndpointErrStatus_TableHit errors.Is 命中表内哨兵 → 表内状态码 + err.Error() 文案。
func TestEndpointErrStatus_TableHit(t *testing.T) {
	t.Parallel()
	e := httpx.Endpoint[int, string]{
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
	t.Parallel()
	e := httpx.Endpoint[int, string]{
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
	t.Parallel()
	e := httpx.Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, errSentinelB
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{{Sentinel: errSentinelA, Status: http.StatusNotFound}}},
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500（未设 fallback 走默认）", w.Code)
	}
}

// TestEndpointErrStatus_RenderCannotOverrideTable 票1b（ADR-0060 §1）：Render 只写成功面。
// 错误面归骨架查域表——自定义 Render 既看不到 err、也不得被调用；成功时 Render 说了算。
func TestEndpointErrStatus_RenderCannotOverrideTable(t *testing.T) {
	t.Parallel()
	rendered := 0
	render := func(c *gin.Context, _ *int, resp *string) {
		rendered++
		c.JSON(http.StatusTeapot, gin.H{"code": 418, "message": httpx.Deref(resp)})
	}
	// 错误面：testTable 命中 404，Render 不参与
	w := doEndpoint(t, httpx.Endpoint[int, string]{
		Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, errSentinelA },
		ErrStatus: testTable,
		Render:    render,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404（错误面归域表）", w.Code)
	}
	if rendered != 0 {
		t.Fatalf("错误面调用了 Render %d 次，期望 0（票1b：err 不在 Render 签名上）", rendered)
	}
	// 成功面：Render 全权
	w = doEndpoint(t, httpx.Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			v := "ok"
			return &v, nil
		},
		ErrStatus: testTable,
		Render:    render,
	})
	if w.Code != http.StatusTeapot {
		t.Fatalf("状态码 = %d, 期望 418（成功面归 Render）", w.Code)
	}
	if rendered != 1 {
		t.Fatalf("成功面 Render 调用 %d 次, 期望 1", rendered)
	}
}

// TestEndpointErrStatus_ParseError_PrecedesTable 解析错误优先于域表：
// ParseError 404 不得被 fallback 400 吞掉。
func TestEndpointErrStatus_ParseError_PrecedesTable(t *testing.T) {
	t.Parallel()
	e := httpx.Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &httpx.ParseError{Status: http.StatusNotFound, Message: "路径参数无效"}
		},
		ErrStatus: testTable,
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404（ParseError 优先于表）", w.Code)
	}
}

// TestEndpointErrStatus_ParseError_PrecedesUnconditionalEntry 票8（ADR-0062 决策 8）翻转后的优先级 1：
// `*ParseError` 恒优先于 `sentinel == nil` 的无条件条目——「整条错误面只有一个固定码」不再把 400 类
// 参数错误吞进那个固定码里（翻转前本例钉的是 500，钉法随票8 一并改）。
// 无条件条目仍然命中**其余**一切错误（业务错误与 DB 故障保持该端点既有的单一码形状）。
func TestEndpointErrStatus_ParseError_PrecedesUnconditionalEntry(t *testing.T) {
	t.Parallel()
	e := httpx.Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &httpx.ParseError{Status: http.StatusNotFound, Message: "路径参数无效"}
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusInternalServerError),
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404（*ParseError 恒优先于无条件条目）", w.Code)
	}
	if !strings.Contains(w.Body.String(), "路径参数无效") {
		t.Fatalf("文案必须是解析错误自己的话: %s", w.Body.String())
	}
	// 同一张表的非解析错误仍是那个固定码（无条件条目没有被削弱，只是不再吃 ParseError）
	w = doEndpoint(t, httpx.Endpoint[int, string]{
		Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, errSentinelB },
		ErrStatus: httpx.ErrStatusAll(http.StatusInternalServerError),
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500（无条件条目仍兜住其余错误）", w.Code)
	}
}

// TestEndpointErrStatus_ParseError_PrecedesSentinelTableEntries 域表具名条目命中不了 *ParseError：
// 解析错误不包装业务哨兵，故「ParseError 先判」与「表先扫」对具名条目逐字等价（本例锁住这一点，
// 免得翻转被读成「域表语义变了」）。
func TestEndpointErrStatus_ParseError_PrecedesSentinelTableEntries(t *testing.T) {
	t.Parallel()
	w := doEndpoint(t, httpx.Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, httpx.BadRequest("查询参数无效")
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: errSentinelA, Status: http.StatusNotFound, Message: "主题不存在"},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400（解析错误既不进哨兵条目、也不进无条件条目）", w.Code)
	}
}

// TestEndpointErrStatus_FixedMessageEntry 票1b 的固定文案槽：message 非空即渲染该文案而非 err.Error()
// （收编自旧闭包的 response.Xxx(c, "字面量") 与 gorm.ErrRecordNotFound → 404「主题不存在」两族）。
func TestEndpointErrStatus_FixedMessageEntry(t *testing.T) {
	t.Parallel()
	// 「哨兵 + 尾部无条件条目」= 旧 if-chain 的形状：先命中先用，尾部 else 兜一切
	// （票8 起解析错误不再归它兜，见 TestEndpointErrStatus_ParseError_PrecedesUnconditionalEntry）
	tbl := &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
		{Sentinel: errSentinelA, Status: http.StatusNotFound, Message: "主题不存在"},
		{Sentinel: nil, Status: http.StatusBadRequest, Message: "查询用户列表失败"},
	}}
	w := doEndpoint(t, httpx.Endpoint[int, string]{
		Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, errors.New("底层噪声") },
		ErrStatus: tbl,
	})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "查询用户列表失败") {
		t.Fatalf("未命中哨兵应落尾部无条件条目: %d %s", w.Code, w.Body.String())
	}
	// 哨兵命中 → 固定文案（err.Error() 不外泄）
	w = doEndpoint(t, httpx.Endpoint[int, string]{
		Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, fmt.Errorf("ctx: %w", errSentinelA) },
		ErrStatus: tbl,
	})
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "主题不存在") {
		t.Fatalf("哨兵条目应先于无条件条目命中: %d %s", w.Code, w.Body.String())
	}
	// 真哨兵仍优先于 fallback（解析错误永远先判，不进 fallback）
	w = doEndpoint(t, httpx.Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: errSentinelA, Status: http.StatusNotFound},
		}, Fallback: http.StatusBadRequest},
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("状态码 = %d, 期望 401（ParseError 是规则 1，不进表也不进 fallback）", w.Code)
	}
}

// assertTableSnapshot 钉住域表全集：条数、顺序、哨兵身份、状态码、固定文案、fallback。
// 表内容变更时本测试即红——须有意识地同步更新快照（防状态码语义漂移）。
func assertTableSnapshot(t *testing.T, name string, got *httpx.ErrStatusTable, want []httpx.ErrStatusEntry, wantFallback int) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s 表不存在", name)
	}
	if got.Fallback != wantFallback {
		t.Fatalf("%s fallback = %d, 期望 %d", name, got.Fallback, wantFallback)
	}
	if len(got.Entries) != len(want) {
		t.Fatalf("%s 条目数 = %d, 期望 %d（表内容漂移，须同步快照）", name, len(got.Entries), len(want))
	}
	for i, w := range want {
		g := got.Entries[i]
		if g.Sentinel != w.Sentinel {
			t.Fatalf("%s entries[%d] 哨兵身份漂移: got %v, want %v", name, i, g.Sentinel, w.Sentinel)
		}
		if g.Status != w.Status {
			t.Fatalf("%s entries[%d] %v 状态码 = %d, 期望 %d", name, i, g.Sentinel, g.Status, w.Status)
		}
		if g.Message != w.Message {
			t.Fatalf("%s entries[%d] %v 固定文案 = %q, 期望 %q", name, i, g.Sentinel, g.Message, w.Message)
		}
		if g.Sentinel == nil {
			t.Fatalf("%s entries[%d] 为无条件条目（sentinel==nil）：域表是「一语义一码」的具名集合，"+
				"无条件条目会把不属于任何哨兵的错误（DB 故障在内）压成同一个码，"+
				"只允许出现在端点自带的 errStatusAll / WithSuccess 表里", name, i)
		}
	}
}

// TestErrStatusTable_Snapshot_Points 积分域表快照（#610：已领取类 400、不存在类 404、兜底 400；
// #1098 追加：扣罚目标不存在 404、通知写失败 500）。
func TestErrStatusTable_Snapshot_Points(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "points.ErrStatus", points.ErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: points.ErrTaskNotFound, Status: http.StatusNotFound},
		{Sentinel: model.ErrHrwaiUserNotFound, Status: http.StatusNotFound},
		{Sentinel: model.ErrCourseNotFound, Status: http.StatusBadRequest},
		{Sentinel: points.ErrCourseNotRedeemable, Status: http.StatusBadRequest},
		{Sentinel: points.ErrAlreadyClaimed, Status: http.StatusBadRequest},
		{Sentinel: points.ErrDailyClaimLimit, Status: http.StatusBadRequest},
		{Sentinel: points.ErrInsufficientPoints, Status: http.StatusBadRequest},
		{Sentinel: points.ErrAlreadyRedeemed, Status: http.StatusBadRequest},
		{Sentinel: points.ErrPenaltyNotifyFailed, Status: http.StatusInternalServerError},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_Contribution 投稿域表快照（#611：不存在 404、其余 400、未设 fallback → 500；
// #1361 追加暂存文件四校验的落档：越权 403 / 类型与已登记 400 / 文件不存在 404）。
func TestErrStatusTable_Snapshot_Contribution(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "contribution.ErrStatus", contribution.ErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: contribution.ErrContributionNotFound, Status: http.StatusNotFound},
		{Sentinel: contribution.ErrContributionFileMissing, Status: http.StatusNotFound},
		{Sentinel: contribution.ErrContributionStagedNotOwner, Status: http.StatusForbidden},
		{Sentinel: contribution.ErrContributionNotOwner, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionNotPending, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionNotApproved, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionQuotaDaily, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionQuotaPending, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionNoCredential, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionTitleRequired, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionIntroRequired, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFilesRequired, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFilesTooMany, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFileTooLarge, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionTotalTooLarge, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFileInvalid, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFileExtNotAllowed, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionFileAlreadyClaimed, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionRejectReason, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionArchiveReason, Status: http.StatusBadRequest},
		{Sentinel: contribution.ErrContributionInvalidReportReason, Status: http.StatusBadRequest},
	}, 0)
}

// TestErrStatusTable_Snapshot_Application 投递域表快照（#611）。
func TestErrStatusTable_Snapshot_Application(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "applicationErrStatus", applicationErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: service.ErrApplyJobInactive, Status: http.StatusNotFound},
		{Sentinel: service.ErrJobNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrApplyNotYours, Status: http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_JobReport 举报治理域表快照（#611）。
func TestErrStatusTable_Snapshot_JobReport(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "jobReportErrStatus", jobReportErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: service.ErrReportJobNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrReportNotFound, Status: http.StatusNotFound},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_RecruiterApplication 企业侧投递域表快照（#611）。
func TestErrStatusTable_Snapshot_RecruiterApplication(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "recruiterApplicationErrStatus", recruiterApplicationErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: service.ErrJobNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrApplyNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrApplyNotYours, Status: http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_Job 职位域表快照（#611）。
func TestErrStatusTable_Snapshot_Job(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "jobErrStatus", jobErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: service.ErrJobNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrJobNotYours, Status: http.StatusForbidden},
	}, http.StatusBadRequest)
}

// TestErrStatusTable_Snapshot_QuestionBank 题库域表快照（#611；第十二波票 6 补写面哨兵族并撤 fallback——未命中即 500）。
func TestErrStatusTable_Snapshot_QuestionBank(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "questionBankErrStatus", questionBankErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: service.ErrQuestionNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrQuestionCredentialNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrQuestionTypeInvalid, Status: http.StatusBadRequest},
		{Sentinel: service.ErrQuestionContentRequired, Status: http.StatusBadRequest},
		{Sentinel: service.ErrQuestionAnswerRequired, Status: http.StatusBadRequest},
		{Sentinel: service.ErrQuestionOptionsRequired, Status: http.StatusBadRequest},
		{Sentinel: service.ErrQuestionAnswerInvalid, Status: http.StatusBadRequest},
		{Sentinel: service.ErrSubmitNotDraft, Status: http.StatusBadRequest},
		{Sentinel: service.ErrRejectReasonRequired, Status: http.StatusBadRequest},
	}, 0)
}

// TestQuestionBankErrStatus_Spectrum 票 6：题库域表 400/404/500 档位断言（DB 故障未命中 → 500）。
func TestQuestionBankErrStatus_Spectrum(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"题目不存在 404", service.ErrQuestionNotFound, http.StatusNotFound},
		{"证件不存在 404", service.ErrQuestionCredentialNotFound, http.StatusNotFound},
		{"证件不存在 wrap 后仍命中 404", fmt.Errorf("ctx: %w", service.ErrQuestionCredentialNotFound), http.StatusNotFound},
		{"非 draft 提交 400", service.ErrSubmitNotDraft, http.StatusBadRequest},
		{"驳回缺理由 400", service.ErrRejectReasonRequired, http.StatusBadRequest},
		{"题型无效含列表 400", fmt.Errorf("%w，支持的题型：%s", service.ErrQuestionTypeInvalid, "single_choice"), http.StatusBadRequest},
		{"DB 故障未命中 500", errors.New("dial tcp: db down"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := httpx.Endpoint[int, string]{
				Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, c.err },
				ErrStatus: questionBankErrStatus,
			}
			w := doEndpoint(t, e)
			if w.Code != c.want {
				t.Fatalf("状态码 = %d, 期望 %d（err=%v）", w.Code, c.want, c.err)
			}
		})
	}
}

// TestErrStatusTable_Snapshot_Forum 第十二波票 5：论坛域表快照
// （存在性 404 / 所有权 403 / 状态前置与校验 400 / 未设 fallback → 未命中即 500）。
func TestErrStatusTable_Snapshot_Forum(t *testing.T) {
	t.Parallel()
	assertTableSnapshot(t, "forum.ErrStatus", forum.ErrStatus, []httpx.ErrStatusEntry{
		{Sentinel: forum.ErrTopicNotFound, Status: http.StatusNotFound},
		{Sentinel: forum.ErrReplyNotFound, Status: http.StatusNotFound},
		{Sentinel: forum.ErrForumReportNotFound, Status: http.StatusNotFound},
		{Sentinel: service.ErrChapterNotFound, Status: http.StatusNotFound},
		{Sentinel: forum.ErrNotTopicOwner, Status: http.StatusForbidden},
		{Sentinel: forum.ErrNotTopicAuthor, Status: http.StatusForbidden},
		{Sentinel: forum.ErrNotReplyAuthor, Status: http.StatusForbidden},
		{Sentinel: forum.ErrAcceptOwnReply, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrAcceptNotQuestion, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrCancelAcceptNotQuestion, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrAcceptExperienceTopic, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrDesignateAcceptedTopic, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrUnfeatureExperienceTopic, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrCategoryLockedByAccept, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrQuestionChapterConflict, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrParentReplyMismatch, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrReplyTopicMismatch, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrContentFormatInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrCategoryInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrSolvedArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrFeaturedArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrExperienceArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrSolvedFilterScope, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrChapterIDRequired, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrTitleLength, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrContentLength, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrReplyContentLength, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrImagesTooMany, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrImageURLInvalid, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrReportReasonLength, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrReportTarget, Status: http.StatusBadRequest},
		{Sentinel: forum.ErrReportStatusValue, Status: http.StatusBadRequest},
	}, 0)
}

// TestForumErrStatus_Spectrum 票 5：域表 400/403/404/500 全谱表驱动断言——
// 四档各有代表哨兵、%w 包装详情仍能命中、未命中（DB 故障形态）落 500。
func TestForumErrStatus_Spectrum(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"存在性：主题 404", forum.ErrTopicNotFound, http.StatusNotFound},
		{"存在性：回复 404", forum.ErrReplyNotFound, http.StatusNotFound},
		{"存在性：举报 404", forum.ErrForumReportNotFound, http.StatusNotFound},
		{"存在性：章节 404", service.ErrChapterNotFound, http.StatusNotFound},
		{"所有权：楼主动作 403", forum.ErrNotTopicOwner, http.StatusForbidden},
		{"所有权：删主题 403", forum.ErrNotTopicAuthor, http.StatusForbidden},
		{"所有权：删回复 403", forum.ErrNotReplyAuthor, http.StatusForbidden},
		{"状态前置：自采纳 400", forum.ErrAcceptOwnReply, http.StatusBadRequest},
		{"状态前置：撤精镜像 400", forum.ErrUnfeatureExperienceTopic, http.StatusBadRequest},
		{"校验：包装详情命中 400", fmt.Errorf("%w: discussion2", forum.ErrCategoryInvalid), http.StatusBadRequest},
		{"校验：图片张数包装 400", fmt.Errorf("%w（最多 9 张）", forum.ErrImagesTooMany), http.StatusBadRequest},
		{"未命中：DB 故障 500", errors.New("dial tcp 127.0.0.1: db down"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := httpx.Endpoint[int, string]{
				Invoke:    func(ctx context.Context, req *int) (*string, error) { return nil, c.err },
				ErrStatus: forum.ErrStatus,
			}
			w := doEndpoint(t, e)
			if w.Code != c.want {
				t.Fatalf("状态码 = %d, 期望 %d（err=%v）", w.Code, c.want, c.err)
			}
		})
	}
}
