package questionbank

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

// @Summary 设置题目的公开开关
// @Tags 题目管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body publicscope.VisibilityInput true "是否允许公开"
// @Success 200 {object} response.R{data=publicscope.Eligibility} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "内容不存在"
// @Router /question-bank/questions/{question_id}/public-visibility [put]
func (h *handler) SetPublicVisibility(c *gin.Context) {
	if middleware.CurrentRole(c) != string(authz.RoleAdmin) {
		response.Forbidden(c, "仅管理员可修改公开开关")
		return
	}
	httpx.Endpoint[visibilityRequest, publicscope.Eligibility]{
		Parse: func(c *gin.Context) (*visibilityRequest, error) {
			id, err := httpx.PathInt(c, "question_id", "题目ID无效")
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
			return h.svc.setPublicVisibility(ctx, req.ID, req.Visible)
		},
	}.WithSentinel(ErrQuestionNotFound, http.StatusNotFound).Handle(c)
}

func (s *Service) setPublicVisibility(ctx context.Context, id int, visible bool) (*publicscope.Eligibility, error) {
	db := s.db.WithContext(ctx)
	updated := db.Table("question").Where("id = ?", id).UpdateColumn("public_hidden", !visible)
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected == 0 {
		return nil, ErrQuestionNotFound
	}
	var record model.Question
	if err := db.First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	item, err := s.decorateEditorQuestion(&record, false)
	if err != nil {
		return nil, err
	}
	return item.PublicEligibility, nil
}
