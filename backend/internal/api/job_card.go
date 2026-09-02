package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

type JobCardHandler struct {
	svc     *service.JobCardService
	fileSvc *service.FileStore
}

func NewJobCardHandler(svc *service.JobCardService, fileSvc *service.FileStore) *JobCardHandler {
	return &JobCardHandler{svc: svc, fileSvc: fileSvc}
}

func RegisterJobCardRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.JobCardService, fileSvc *service.FileStore) {
	h := NewJobCardHandler(svc, fileSvc)
	g := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	g.GET("", h.Get)
	g.PUT("", h.Upsert)
	g.PUT("/visibility", h.UpdateVisibility)
	g.POST("/pdf", h.UploadPDF)
	g.DELETE("/pdf", h.DeletePDF)
	g.POST("/image", h.UploadImage)
}

// Get 我的简历 GET /api/resume（未建时 404）
// @Summary 我的简历
// @Description 学员查看自己的简历卡（未建时 404，契约内空态）
// @Tags 学员端-简历卡
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "简历"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "简历不存在"
// @Router /resume [get]
func (h *JobCardHandler) Get(c *gin.Context) {
	Endpoint[resumeGetReq, service.JobCardDTO]{
		Parse: func(c *gin.Context) (*resumeGetReq, error) {
			return &resumeGetReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *resumeGetReq) (*service.JobCardDTO, error) {
			return h.svc.Get(req.UserID)
		},
		Render: func(c *gin.Context, _ *resumeGetReq, resp *service.JobCardDTO, err error) {
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.NotFound(c, "简历不存在")
					return
				}
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Upsert 保存简历 PUT /api/resume（整页保存）
// @Summary 保存简历
// @Description 学员整页保存简历卡（real_name/contact_phone/wechat/region/expected_specialty_id/expected_regions/salary/experience 等）
// @Tags 学员端-简历卡
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.JobCardInput true "简历内容"
// @Success 200 {object} response.R "已保存"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /resume [put]
func (h *JobCardHandler) Upsert(c *gin.Context) {
	Endpoint[resumeUpsertReq, service.JobCardDTO]{
		Parse: func(c *gin.Context) (*resumeUpsertReq, error) {
			uid := middleware.CurrentUserID(c)
			var body service.JobCardInput
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &resumeUpsertReq{UserID: uid, Input: body}, nil
		},
		Invoke: func(ctx context.Context, req *resumeUpsertReq) (*service.JobCardDTO, error) {
			return h.svc.Upsert(req.UserID, req.Input)
		},
		Render: func(c *gin.Context, _ *resumeUpsertReq, resp *service.JobCardDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UpdateVisibility 切换简历公开 PUT /api/resume/visibility
// @Summary 切换简历公开
// @Description 学员切换简历 visibility（hidden/open）；visibility 仅管控 L2 被动浏览面，不影响投递
// @Tags 学员端-简历卡
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "可见性 {visibility: hidden|open}"
// @Success 200 {object} response.R "已切换"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/visibility [put]
func (h *JobCardHandler) UpdateVisibility(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	var body struct {
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	dto, err := h.svc.UpdateVisibility(uid, body.Visibility)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, dto)
}

// UploadPDF 上传简历 PDF POST /api/resume/pdf
// @Summary 上传简历 PDF
// @Description 上传简历 PDF（file，仅 PDF ≤50MB）
// @Tags 学员端-简历卡
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "PDF 简历"
// @Success 200 {object} response.R "上传成功 {url}"
// @Failure 400 {object} response.R "文件错误"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/pdf [post]
func (h *JobCardHandler) UploadPDF(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	if file.Size > 50*1024*1024 {
		response.BadRequest(c, "文件大小超出限制，最大允许50MB")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.ServerError(c, "文件读取失败")
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		response.ServerError(c, "文件读取失败")
		return
	}
	url, err := h.svc.ValidateAndStorePDF(file.Filename, file.Size, content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "上传成功", gin.H{"url": url})
}

// DeletePDF 删除 PDF 附件 DELETE /api/resume/pdf（#491）
// @Summary 删除 PDF 附件
// @Description 学员删除自己简历卡的上传 PDF 附件（resume_file_url 置空）
// @Tags 学员端-简历卡
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "已删除"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/pdf [delete]
func (h *JobCardHandler) DeletePDF(c *gin.Context) {
	if err := h.svc.DeleteResumeFile(middleware.CurrentUserID(c)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "附件已删除", gin.H{})
}

// UploadImage 上传工作照 POST /api/resume/image
// @Summary 上传工作照
// @Description 上传工作照（file，图片 ≤20MB，≤6 张）
// @Tags 学员端-简历卡
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "工作照"
// @Success 200 {object} response.R "上传成功 {url}"
// @Failure 400 {object} response.R "文件错误"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/image [post]
func (h *JobCardHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	if ok, msg := h.fileSvc.ValidateImage(file.Filename, file.Size); !ok {
		response.BadRequest(c, msg)
		return
	}
	src, err := file.Open()
	if err != nil {
		response.ServerError(c, "文件读取失败")
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		response.ServerError(c, "文件读取失败")
		return
	}
	url, err := h.fileSvc.Save(content, file.Filename, "resumes/images")
	if err != nil {
		response.ServerError(c, "保存失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "上传成功", gin.H{"url": url})
}

type resumeGetReq struct {
	UserID int
}

type resumeUpsertReq struct {
	UserID int
	Input  service.JobCardInput
}

var _ = http.StatusOK
