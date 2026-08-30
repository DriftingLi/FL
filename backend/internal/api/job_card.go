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
	g.POST("/image", h.UploadImage)
}

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
