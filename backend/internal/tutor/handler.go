// 本文件：讲师域 HTTP 出口（/api/tutor 蓝图）。
// 装配点：internal/api/routes_registry.go 调 tutor.RegisterRoutes(api, rd.Session, deps.TutorSvc, deps.FileSvc, uploadVditorImage)。
package tutor

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/course"
	"forklift-training/internal/filestore"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// VditorUploader 注入的 Vditor 上传适配器（实现在 backend/internal/api/vditor_upload.go，信封单点）。
type VditorUploader func(c *gin.Context, fileSvc *filestore.FileStore, saver func(content []byte, filename string) (string, error))

// handler 导师端 handler。
type handler struct {
	svc      *Service
	fileSvc  *filestore.FileStore
	uploader VditorUploader
}

// newHandler 创建导师端 handler。
func newHandler(svc *Service, fileSvc *filestore.FileStore, uploader VditorUploader) *handler {
	return &handler{svc: svc, fileSvc: fileSvc, uploader: uploader}
}

// RegisterRoutes 注册 /api/tutor 蓝图。
//
// 守卫分两组（#2595）：读组留 tutor.access，写组挂 course.author。收敛前一个 CapTutorAccess
// 盖住全部 9 端点，「只读讲师」表达不出来——要给某人课程读权就必然连同改章节与删附件一起给。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, fileSvc *filestore.FileStore, uploader VditorUploader) {
	h := newHandler(svc, fileSvc, uploader)

	g := rg.Group("/tutor", middleware.JWTAuth(session))
	read := g.Group("", middleware.CapabilityRequired(authz.CapTutorAccess))
	write := g.Group("", middleware.CapabilityRequired(authz.CapCourseAuthor))

	// GET /api/tutor/courses  导师课程列表
	read.GET("/courses", h.ListCourses)
	// GET /api/tutor/course/:course_id/chapters  课程章节列表（含文件）
	read.GET("/course/:course_id/chapters", h.GetCourseChapters)
	// GET /api/tutor/chapter/:chapter_id  章节详情（含上下章ID + 文件列表）
	read.GET("/chapter/:chapter_id", h.GetChapterDetail)
	// POST /api/tutor/chapter/:chapter_id/upload  上传章节文件
	write.POST("/chapter/:chapter_id/upload", h.UploadChapterFile)
	write.POST("/chapter/:chapter_id/submit-review", h.SubmitChapterReview)
	// POST /api/tutor/upload-image  上传图文 Markdown 中的图片（Vditor 格式）
	write.POST("/upload-image", h.UploadImage)
	// PUT /api/tutor/chapter/:chapter_id  更新章节信息
	write.PUT("/chapter/:chapter_id", h.UpdateChapterInfo)
	// DELETE /api/tutor/file/:file_id  删除章节文件
	write.DELETE("/file/:file_id", h.DeleteChapterFile)
	// POST /api/tutor/files/batch-delete  批量删除文件
	write.POST("/files/batch-delete", h.BatchDeleteChapterFiles)
	// POST /api/tutor/course/:course_id/claim  讲师自助认领该课程（#2596 B3）
	write.POST("/course/:course_id/claim", h.ClaimCourse)
	// DELETE /api/tutor/course/:course_id/claim  讲师退出认领（#2596 B3/B6）
	write.DELETE("/course/:course_id/claim", h.ReleaseCourseClaim)
	// GET /api/tutor/course/:course_id/claims  该课程的共认讲师名单（#2596 D1）
	read.GET("/course/:course_id/claims", h.ListCourseClaimants)
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
// @Success 200 {object} response.R{data=course.CoursePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/courses [get]
func (h *handler) ListCourses(c *gin.Context) {
	httpx.Endpoint[tutorCourseListReq, course.CoursePageResult]{
		Parse: func(c *gin.Context) (*tutorCourseListReq, error) {
			return &tutorCourseListReq{
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 10),
				CredentialID: httpx.QueryIDPtr(c, "credential_id"),
				SpecialtyID:  httpx.QueryIDPtr(c, "specialty_id"),
				LevelID:      httpx.QueryIDPtr(c, "level_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *tutorCourseListReq) (*course.CoursePageResult, error) {
			result, err := h.svc.GetCourses(req.Page, req.PageSize, req.CredentialID, req.SpecialtyID, req.LevelID, middleware.CurrentUserID(c))
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetCourseChapters 课程章节列表（含文件）
// @Summary 导师端课程章节
// @Description 返回课程摘要 + 章节列表（含每章文件）
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=course.TutorCourseChaptersDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /tutor/course/{course_id}/chapters [get]
func (h *handler) GetCourseChapters(c *gin.Context) {
	httpx.Endpoint[idParam, course.TutorCourseChaptersDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*course.TutorCourseChaptersDTO, error) {
			return h.svc.GetCourseChapters(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表）
// @Summary 导师端章节详情
// @Description 返回章节详情（含相邻章节ID与文件列表），字段平铺，不含 course/chapters
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=course.ChapterDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /tutor/chapter/{chapter_id} [get]
func (h *handler) GetChapterDetail(c *gin.Context) {
	httpx.Endpoint[idParam, course.ChapterDetailDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*course.ChapterDetailDTO, error) {
			return h.svc.GetChapterDetail(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(course.ErrChapterNotFound, http.StatusNotFound).Handle(c)
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
// @Success 200 {object} response.R{data=course.ChapterFileDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/chapter/{chapter_id}/upload [post]
func (h *handler) UploadChapterFile(c *gin.Context) {
	chapterID, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
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
	content, err := filestore.ReadMultipartFile(file)
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}
	result, err := h.svc.UploadChapterFile(chapterID, middleware.CurrentUserID(c), file.Filename, content)
	if err != nil {
		switch {
		case errors.Is(err, course.ErrCourseClaimRequired):
			response.Forbidden(c, err.Error())
		case errors.Is(err, course.ErrChapterNotFound):
			response.NotFound(c, err.Error())
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}
	response.SuccessWithMsg(c, "文件上传成功", result)
}

// UploadImage 上传图文 Markdown 中的图片（Vditor 格式）POST /api/tutor/upload-image
// form 字段：file（图片）、chapter_id（可选，用于按章节分目录存储 images/chapters/<chapterId>/）
// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
// @Summary 上传讲师图文内嵌图片
// @Description Vditor 编辑器内嵌图上传（需讲师能力）；chapter_id 支持 query（Vditor 走 URL）与 form（直接 multipart）两种传递方式，用于按章节分目录存储。返回 Vditor 约定的**非统一信封**响应 {msg,code,data:{errFiles,succMap}}（code=0 表示成功），与 /admin/featured-content/upload-image 同一适配器
// @Tags 讲师端-课程
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "图片文件"
// @Param chapter_id query int false "章节ID（可选，按章节分目录存储 images/chapters/<id>）"
// @Success 200 {object} map[string]any "Vditor 响应（非统一信封）"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/upload-image [post]
func (h *handler) UploadImage(c *gin.Context) {
	// 按章节分目录存储，便于删除章节时按前缀清理（历史旧目录孤儿文件不处理）
	// chapter_id 支持 query（Vditor 走 URL）与 form（直接 multipart）两种传递方式
	h.uploader(c, h.fileSvc, func(content []byte, filename string) (string, error) {
		subfolder := filestore.ChapterImageDirPrefix
		chapterIDStr := c.Query("chapter_id")
		if chapterIDStr == "" {
			chapterIDStr = c.PostForm("chapter_id")
		}
		if chapterID, err := strconv.Atoi(chapterIDStr); err == nil && chapterID > 0 {
			subfolder = filestore.ChapterImageDirPrefix + "/" + chapterIDStr
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
// @Param body body course.ChapterInput true "章节信息"
// @Success 200 {object} response.R{data=course.ChapterDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /tutor/chapter/{chapter_id} [put]
func (h *handler) UpdateChapterInfo(c *gin.Context) {
	httpx.Endpoint[chapterIDInput, course.ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			data, err := httpx.BindJSONMsg[course.ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*course.ChapterDTO, error) {
			return h.svc.UpdateChapterInfo(req.ID, middleware.CurrentUserID(c), req.Input)
		},
	}.WithSuccess(httpx.OkMsg("章节更新成功"), http.StatusInternalServerError).
		WithSentinel(course.ErrChapterNotFound, http.StatusNotFound).
		WithSentinel(course.ErrCourseClaimRequired, http.StatusForbidden).Handle(c)
}

// SubmitChapterReview 提交章节审核
// @Summary 提交章节审核
// @Description 冻结整章快照（正文 + 文件挂接清单）进在审队列（#2498 C2/C4）；同章单 pending，重复提交 400（C8）
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=course.ChapterDTO} "success"
// @Failure 400 {object} response.R "已有在审版本"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /tutor/chapter/{chapter_id}/submit-review [post]
func (h *handler) SubmitChapterReview(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	httpx.Endpoint[idParam, course.ChapterDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*course.ChapterDTO, error) {
			return h.svc.SubmitChapterReview(ctx, req.ID, uid)
		},
	}.WithSuccess(httpx.OkMsg("已提交审核"), http.StatusInternalServerError).
		WithSentinel(course.ErrChapterNotFound, http.StatusNotFound).
		WithSentinel(course.ErrCourseClaimRequired, http.StatusForbidden).
		WithSentinel(course.ErrChapterReviewPending, http.StatusBadRequest).Handle(c)
}

// DeleteChapterFile 删除章节文件
// @Summary 删除章节文件
// @Description 按文件ID删除章节附件，返回 file_id 与 deleted
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件ID"
// @Success 200 {object} response.R{data=tutor.DeleteFileResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "文件不存在"
// @Router /tutor/file/{file_id} [delete]
func (h *handler) DeleteChapterFile(c *gin.Context) {
	httpx.Endpoint[idParam, DeleteFileResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "file_id", "文件ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*DeleteFileResult, error) {
			return h.svc.DeleteChapterFileByID(req.ID, middleware.CurrentUserID(c))
		},
	}.WithSuccess(httpx.OkMsg("文件删除成功"), http.StatusInternalServerError).
		WithSentinel(ErrChapterFileNotFound, http.StatusNotFound).
		WithSentinel(course.ErrCourseClaimRequired, http.StatusForbidden).
		WithSentinel(course.ErrChapterNotFound, http.StatusNotFound).Handle(c)
}

// BatchDeleteChapterFiles 批量删除文件
// @Summary 批量删除章节文件
// @Description 批量删除章节附件，返回成功/失败条数与失败ID
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "文件ID列表" example({"file_ids":[1,2]})
// @Success 200 {object} response.R{data=tutor.BatchDeleteFilesResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/files/batch-delete [post]
func (h *handler) BatchDeleteChapterFiles(c *gin.Context) {
	httpx.Endpoint[batchDeleteFilesReq, BatchDeleteFilesResult]{
		Parse: func(c *gin.Context) (*batchDeleteFilesReq, error) {
			req, err := httpx.BindJSON[batchDeleteFilesReq](c)
			if err != nil {
				return nil, err
			}
			if len(req.FileIDs) == 0 {
				return nil, httpx.BadRequest("请选择要删除的文件")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *batchDeleteFilesReq) (*BatchDeleteFilesResult, error) {
			return h.svc.BatchDeleteChapterFiles(req.FileIDs, middleware.CurrentUserID(c))
		},
		Render: func(c *gin.Context, _ *batchDeleteFilesReq, resp *BatchDeleteFilesResult) {
			response.SuccessWithMsg(c, "成功删除"+strconv.Itoa(resp.SuccessCount)+"个文件", resp)
		},
	}.WithSentinel(course.ErrCourseClaimRequired, http.StatusForbidden).
		WithSentinel(course.ErrChapterNotFound, http.StatusNotFound).Handle(c)
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

// ===== 课程认领（#2596 / 规格 B3、B5、B6、D1）=====

// ClaimCourse 讲师自助认领课程
// @Summary 认领课程
// @Description 讲师认领该课程以获得其章节的编辑权；多讲师可共认同一门课。已认领返回 409。
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=tutor.ClaimResult} "success"
// @Failure 404 {object} response.R "课程不存在"
// @Failure 409 {object} response.R "已认领"
// @Router /tutor/course/{course_id}/claim [post]
func (h *handler) ClaimCourse(c *gin.Context) {
	httpx.Endpoint[idParam, ClaimResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*ClaimResult, error) {
			if err := h.svc.ClaimCourse(middleware.CurrentUserID(c), req.ID); err != nil {
				return nil, err
			}
			return &ClaimResult{CourseID: req.ID, Claimed: true}, nil
		},
	}.WithSuccess(httpx.OkMsg("已认领，可以开始编辑章节"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).
		WithSentinel(course.ErrCourseAlreadyClaimed, http.StatusConflict).Handle(c)
}

// ReleaseCourseClaim 讲师退出认领
// @Summary 退出认领
// @Description 只断今后的编辑权：已写章节的作者归属保留、在审内容照常审完。未认领返回 404 档。
// @Tags 讲师端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=tutor.ClaimResult} "success"
// @Failure 404 {object} response.R "未认领该课程"
// @Router /tutor/course/{course_id}/claim [delete]
func (h *handler) ReleaseCourseClaim(c *gin.Context) {
	httpx.Endpoint[idParam, ClaimResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*ClaimResult, error) {
			if err := h.svc.ReleaseCourseClaim(middleware.CurrentUserID(c), req.ID); err != nil {
				return nil, err
			}
			return &ClaimResult{CourseID: req.ID, Claimed: false}, nil
		},
	}.WithSuccess(httpx.OkMsg("已退出认领"), http.StatusInternalServerError).
		WithSentinel(course.ErrCourseNotClaimed, http.StatusNotFound).Handle(c)
}

// ListCourseClaimants 课程共认讲师名单
// @Summary 认领讲师名单
// @Description 返回该课程当前认领的讲师（按认领时间升序），供章节页展示共认者
// @Tags 讲师端-课程
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=course.CourseClaimantsResult} "success"
// @Failure 404 {object} response.R "课程不存在"
// @Router /tutor/course/{course_id}/claims [get]
func (h *handler) ListCourseClaimants(c *gin.Context) {
	httpx.Endpoint[idParam, course.CourseClaimantsResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*course.CourseClaimantsResult, error) {
			return h.svc.ListCourseClaimants(middleware.CurrentUserID(c), req.ID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}

// idParam :id 路径整型请求（原 internal/api/admin.go:878 的共享私有类型；域包不得引用装配根的私有名，
// 故在本包落一份 1 字段副本 —— 形状与语义逐字一致）。
type idParam struct{ ID int }

// chapterIDInput 章节路径 id + 章节请求体（原 internal/api/admin.go:894 的共享私有类型，同上）。
type chapterIDInput struct {
	ID    int
	Input *course.ChapterInput
}
