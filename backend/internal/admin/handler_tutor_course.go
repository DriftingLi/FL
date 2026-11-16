// 本文件：管理端课程代分配的 HTTP 出口（#2597 / 规格 D4）。
//
// 两个端点同形：`POST/DELETE /api/admin/tutor/:tutor_id/courses`，body 都是
// `{course_ids: [...]}`。为什么不是 `/api/admin/tutor/:tutor_id/course/:course_id`：
// 抽屉是一次多选动作，批量形状与用户动作 1:1 对应；单条形状会让前端发 N 条请求、
// 并在第 3 条失败时自己造一个半完成状态。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// tutorCourseAssignReq 代分配 / 移除的请求（路径 id + 批量课程）。
type tutorCourseAssignReq struct {
	TutorID int
	Input   TutorCourseAssignInput
}

// assignCoursesErrStatus 两个端点共用的错误档位表。
//
// 讲师不存在 → 404（不能让「讲师 id 打错」看起来像服务故障）；
// 课程不存在由服务层吞成 Skipped，不走这里；
// 未命中（DB 故障）→ 500。
var assignCoursesErrStatus = &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
	{Sentinel: model.ErrTutorNotFound, Status: http.StatusNotFound},
	{Sentinel: ErrInvalidTutorID, Status: http.StatusBadRequest},
	{Sentinel: nil, Status: http.StatusInternalServerError},
}}

// parseTutorCourseAssign 两个端点共用的解析：路径讲师 id + body 的课程列表。
//
// 空列表直接 400 而不是「成功 0 条」：管理员点「确定」时一门课都没勾，是误操作，
// 不是「我确认什么都不做」——后者不该产生一次网络往返。
func parseTutorCourseAssign(c *gin.Context) (*tutorCourseAssignReq, error) {
	tutorID, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
	if err != nil {
		return nil, err
	}
	var in TutorCourseAssignInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return nil, httpx.BadRequest("请求数据无效")
	}
	if len(in.CourseIDs) == 0 {
		return nil, httpx.BadRequest("请至少选择一门课程")
	}
	return &tutorCourseAssignReq{TutorID: tutorID, Input: in}, nil
}

// @Summary 给讲师代分配课程
// @Description 管理员把一批课程分配给讲师（需 tutor.manage）；来源记「管理员」，重复分配幂等
// @Tags 管理端-导师
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "讲师 ID"
// @Param body body TutorCourseAssignInput true "课程 ID 列表" example({"course_ids":[1,2]})
// @Success 200 {object} response.R{data=TutorCourseAssignResult} "success"
// @Failure 400 {object} response.R "参数错误（含课程列表为空）"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无 tutor.manage 能力"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/courses [post]
func (h *adminHandler) AssignTutorCourses(c *gin.Context) {
	httpx.Endpoint[tutorCourseAssignReq, TutorCourseAssignResult]{
		Parse: parseTutorCourseAssign,
		Invoke: func(ctx context.Context, req *tutorCourseAssignReq) (*TutorCourseAssignResult, error) {
			return h.adminSvc.AssignCourses(ctx, req.TutorID, req.Input)
		},
		ErrStatus: assignCoursesErrStatus,
		Render: func(c *gin.Context, _ *tutorCourseAssignReq, resp *TutorCourseAssignResult) {
			response.SuccessWithMsg(c, "课程分配完成", httpx.Deref(resp))
		},
	}.Handle(c)
}

// @Summary 移除讲师的课程分配
// @Description 管理员从讲师身上移除一批课程认领（需 tutor.manage）；**不回溯**——章节作者归属与在审内容不受影响
// @Tags 管理端-导师
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "讲师 ID"
// @Param body body TutorCourseAssignInput true "课程 ID 列表" example({"course_ids":[1,2]})
// @Success 200 {object} response.R{data=TutorCourseAssignResult} "success"
// @Failure 400 {object} response.R "参数错误（含课程列表为空）"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无 tutor.manage 能力"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/courses [delete]
func (h *adminHandler) RemoveTutorCourses(c *gin.Context) {
	httpx.Endpoint[tutorCourseAssignReq, TutorCourseAssignResult]{
		Parse: parseTutorCourseAssign,
		Invoke: func(ctx context.Context, req *tutorCourseAssignReq) (*TutorCourseAssignResult, error) {
			return h.adminSvc.RemoveCourses(ctx, req.TutorID, req.Input)
		},
		ErrStatus: assignCoursesErrStatus,
		Render: func(c *gin.Context, _ *tutorCourseAssignReq, resp *TutorCourseAssignResult) {
			response.SuccessWithMsg(c, "课程移除完成", httpx.Deref(resp))
		},
	}.Handle(c)
}

// @Summary 查讲师已分配的课程
// @Description 列出某讲师已分配的课程（抽屉的「已分配」标识数据源，需 tutor.manage）
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "讲师 ID"
// @Success 200 {object} response.R{data=TutorCourseAssignmentListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无 tutor.manage 能力"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/courses [get]
func (h *adminHandler) ListTutorCourses(c *gin.Context) {
	httpx.Endpoint[idParam, TutorCourseAssignmentListDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*TutorCourseAssignmentListDTO, error) {
			return h.adminSvc.ListTutorCourseAssignments(ctx, req.ID)
		},
		ErrStatus: assignCoursesErrStatus,
		Render: func(c *gin.Context, _ *idParam, resp *TutorCourseAssignmentListDTO) {
			response.Success(c, httpx.Deref(resp))
		},
	}.Handle(c)
}
