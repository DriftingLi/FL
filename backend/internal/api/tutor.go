// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// TutorHandler 导师端 handler。
type TutorHandler struct {
	svc     *service.TutorService
	fileSvc *service.FileStore
}

// NewTutorHandler 创建导师端 handler。
func NewTutorHandler(svc *service.TutorService, fileSvc *service.FileStore) *TutorHandler {
	return &TutorHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterTutorRoutes 注册 /api/tutor 蓝图。
func RegisterTutorRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.TutorService, fileSvc *service.FileStore) {
	h := NewTutorHandler(svc, fileSvc)

	g := rg.Group("/tutor", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapTutorAccess))

	// GET /api/tutor/courses  导师课程列表
	g.GET("/courses", h.ListCourses)
	// GET /api/tutor/course/:course_id/chapters  课程章节列表（含文件）
	g.GET("/course/:course_id/chapters", h.GetCourseChapters)
	// GET /api/tutor/chapter/:chapter_id  章节详情（含上下章ID + 文件列表）
	g.GET("/chapter/:chapter_id", h.GetChapterDetail)
	// POST /api/tutor/chapter/:chapter_id/upload  上传章节文件
	g.POST("/chapter/:chapter_id/upload", h.UploadChapterFile)
	// POST /api/tutor/upload-image  上传图文 Markdown 中的图片（Vditor 格式）
	g.POST("/upload-image", h.UploadImage)
	// PUT /api/tutor/chapter/:chapter_id  更新章节信息
	g.PUT("/chapter/:chapter_id", h.UpdateChapterInfo)
	// DELETE /api/tutor/file/:file_id  删除章节文件
	g.DELETE("/file/:file_id", h.DeleteChapterFile)
	// POST /api/tutor/files/batch-delete  批量删除文件
	g.POST("/files/batch-delete", h.BatchDeleteChapterFiles)
}

// ListCourses 导师课程列表
// @Summary 导师课程列表
// @Description 导师端课程分页列表（可选按证件/专业方向/等级过滤）
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param credential_id query int false "目标证件ID"
// @Param specialty_id query int false "专业方向ID"
// @Param level_id query int false "等级ID"
// @Success 200 {object} response.R{data=service.CoursePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/courses [get]
func (h *TutorHandler) ListCourses(c *gin.Context) {
	Endpoint[tutorCourseListReq, service.CoursePageResult]{
		Parse: func(c *gin.Context) (*tutorCourseListReq, error) {
			return &tutorCourseListReq{
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 10),
				CredentialID: queryIDPtr(c, "credential_id"),
				SpecialtyID:  queryIDPtr(c, "specialty_id"),
				LevelID:      queryIDPtr(c, "level_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *tutorCourseListReq) (*service.CoursePageResult, error) {
			result, err := h.svc.GetCourses(req.Page, req.PageSize, req.CredentialID, req.SpecialtyID, req.LevelID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *tutorCourseListReq, resp *service.CoursePageResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetCourseChapters 课程章节列表（含文件）
// @Summary 导师端课程章节
// @Description 返回课程摘要 + 章节列表（含每章文件）
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=service.TutorCourseChaptersDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /tutor/course/{course_id}/chapters [get]
func (h *TutorHandler) GetCourseChapters(c *gin.Context) {
	Endpoint[idParam, service.TutorCourseChaptersDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.TutorCourseChaptersDTO, error) {
			return h.svc.GetCourseChapters(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.TutorCourseChaptersDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表）
// @Summary 导师端章节详情
// @Description 返回章节详情（含相邻章节ID与文件列表），字段平铺，不含 course/chapters
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=service.ChapterDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /tutor/chapter/{chapter_id} [get]
func (h *TutorHandler) GetChapterDetail(c *gin.Context) {
	Endpoint[idParam, service.ChapterDetailDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.ChapterDetailDTO, error) {
			return h.svc.GetChapterDetail(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.ChapterDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UploadChapterFile 上传章节文件
// @Summary 上传章节文件
// @Description multipart 上传章节课件附件，返回落库后的文件条目
// @Tags 讲师端-课程
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Param file formData file true "课件文件"
// @Success 200 {object} response.R{data=service.ChapterFileDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/chapter/{chapter_id}/upload [post]
func (h *TutorHandler) UploadChapterFile(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	if file.Filename == "" {
		response.BadRequest(c, "未选择文件")
		return
	}
	content, err := service.ReadMultipartFile(file)
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}
	result, err := h.svc.UploadChapterFile(chapterID, file.Filename, content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "文件上传成功", result)
}

// UploadImage 上传图文 Markdown 中的图片（Vditor 格式）POST /api/tutor/upload-image
// form 字段：file（图片）、chapter_id（可选，用于按章节分目录存储 images/chapters/<chapterId>/）
// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
func (h *TutorHandler) UploadImage(c *gin.Context) {
	// 按章节分目录存储，便于删除章节时按前缀清理（历史旧目录孤儿文件不处理）
	// chapter_id 支持 query（Vditor 走 URL）与 form（直接 multipart）两种传递方式
	uploadVditorImage(c, h.fileSvc, func(content []byte, filename string) (string, error) {
		subfolder := service.ChapterImageDirPrefix
		chapterIDStr := c.Query("chapter_id")
		if chapterIDStr == "" {
			chapterIDStr = c.PostForm("chapter_id")
		}
		if chapterID, err := strconv.Atoi(chapterIDStr); err == nil && chapterID > 0 {
			subfolder = service.ChapterImageDirPrefix + "/" + chapterIDStr
		}
		return h.fileSvc.Save(content, filename, subfolder)
	})
}

// UpdateChapterInfo 更新章节信息
// @Summary 更新章节信息
// @Description 更新章节标题/内容/时长/排序等（nil 字段表示不改动）
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Param body body service.ChapterInput true "章节信息"
// @Success 200 {object} response.R{data=service.ChapterDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /tutor/chapter/{chapter_id} [put]
func (h *TutorHandler) UpdateChapterInfo(c *gin.Context) {
	Endpoint[chapterIDInput, service.ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			data, err := bindJSONMsg[service.ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*service.ChapterDTO, error) {
			return h.svc.UpdateChapterInfo(req.ID, req.Input)
		},
		Render: func(c *gin.Context, _ *chapterIDInput, resp *service.ChapterDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "章节更新成功", resp)
		},
	}.Handle(c)
}

// DeleteChapterFile 删除章节文件
// @Summary 删除章节文件
// @Description 按文件ID删除章节附件，返回 file_id 与 deleted
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件ID"
// @Success 200 {object} response.R{data=service.DeleteFileResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "文件不存在"
// @Router /tutor/file/{file_id} [delete]
func (h *TutorHandler) DeleteChapterFile(c *gin.Context) {
	Endpoint[idParam, service.DeleteFileResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "file_id", "文件ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.DeleteFileResult, error) {
			return h.svc.DeleteChapterFileByID(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.DeleteFileResult, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "文件删除成功", resp)
		},
	}.Handle(c)
}

// BatchDeleteChapterFiles 批量删除文件
// @Summary 批量删除章节文件
// @Description 批量删除章节附件，返回成功/失败条数与失败ID
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "文件ID列表" example({"file_ids":[1,2]})
// @Success 200 {object} response.R{data=service.BatchDeleteFilesResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/files/batch-delete [post]
func (h *TutorHandler) BatchDeleteChapterFiles(c *gin.Context) {
	Endpoint[batchDeleteFilesReq, service.BatchDeleteFilesResult]{
		Parse: func(c *gin.Context) (*batchDeleteFilesReq, error) {
			req, err := bindJSON[batchDeleteFilesReq](c)
			if err != nil {
				return nil, err
			}
			if len(req.FileIDs) == 0 {
				return nil, badRequest("请选择要删除的文件")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *batchDeleteFilesReq) (*service.BatchDeleteFilesResult, error) {
			return h.svc.BatchDeleteChapterFiles(req.FileIDs), nil
		},
		Render: func(c *gin.Context, _ *batchDeleteFilesReq, resp *service.BatchDeleteFilesResult, _ error) {
			response.SuccessWithMsg(c, "成功删除"+strconv.Itoa(resp.SuccessCount)+"个文件", resp)
		},
	}.Handle(c)
}

// batchDeleteFilesReq 批量删除文件请求体。
type batchDeleteFilesReq struct {
	FileIDs []int `json:"file_ids"`
}

// tutorCourseListReq 导师课程列表查询参数。
type tutorCourseListReq struct {
	Page         int
	PageSize     int
	CredentialID *int
	SpecialtyID  *int
	LevelID      *int
}
