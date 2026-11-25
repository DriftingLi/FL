package course

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

type visibilityRequest struct {
	ID      int
	Visible bool
}

// @Summary 设置课程的公开开关
// @Tags 课程管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Param body body publicscope.VisibilityInput true "是否允许公开"
// @Success 200 {object} response.R{data=publicscope.Eligibility} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "内容不存在"
// @Router /admin/course/{course_id}/public-visibility [put]
func (h *adminHandler) SetPublicVisibility(c *gin.Context) {
	if middleware.CurrentRole(c) != string(authz.RoleAdmin) {
		response.Forbidden(c, "仅管理员可修改公开开关")
		return
	}
	httpx.Endpoint[visibilityRequest, publicscope.Eligibility]{
		Parse: func(c *gin.Context) (*visibilityRequest, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			input, err := httpx.BindJSON[publicscope.VisibilityInput](c)
			if err != nil {
				return nil, err
			}
			return &visibilityRequest{ID: id, Visible: *input.Visible}, nil
		},
		Invoke: func(ctx context.Context, req *visibilityRequest) (*publicscope.Eligibility, error) {
			return h.courseSvc.setPublicVisibility(ctx, req.ID, req.Visible)
		},
	}.WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}
