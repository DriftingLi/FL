// 本文件：导师文件库 HTTP 面（#2494）——四段上传契约与文件库列表/删除的路由与 handler。
//
// 装配点：internal/api/routes_registry.go 调 tutorfile.RegisterRoutes。
// 分片 PUT 收原始字节流（application/octet-stream），单片 ≤8MiB（A2 上限）；
// 端点不走 httpx.Endpoint 的 10s 骨架——分片 body 在弱网可传数十秒（调研 §7），
// 沿 tutor 域 plain gin 形态，超时由 http.Server 全局读窗兜底。
package tutorfile

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/course"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 导师文件库 handler。
type handler struct {
	svc *Service
}

// RegisterRoutes 注册 /api/tutor/files 蓝图（需讲师能力）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := &handler{svc: svc}

	g := rg.Group("/tutor", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapTutorAccess))

	// POST /api/tutor/files/upload  创建上传会话（init）
	g.POST("/files/upload", h.InitUpload)
	// GET /api/tutor/files/upload/:session_id  断点续传清单（已收分片序号）
	g.GET("/files/upload/:session_id", h.GetUploadSession)
	// PUT /api/tutor/files/upload/:session_id/chunk/:part_number  上传分片
	g.PUT("/files/upload/:session_id/chunk/:part_number", h.UploadChunk)
	// POST /api/tutor/files/upload/:session_id/complete  拼装产出文件行
	g.POST("/files/upload/:session_id/complete", h.CompleteUpload)
	// POST /api/tutor/files/upload/:session_id/abort  放弃会话
	g.POST("/files/upload/:session_id/abort", h.AbortUpload)
	// GET /api/tutor/files  文件库列表
	g.GET("/files", h.ListFiles)
	// POST /api/tutor/files/:file_id/attach  单挂挂接到章节
	g.POST("/files/:file_id/attach", h.AttachFile)
	// POST /api/tutor/files/:file_id/detach  取消挂接回库
	g.POST("/files/:file_id/detach", h.DetachFile)
	// POST /api/tutor/files/:file_id/rename  改名（仅显示名）
	g.POST("/files/:file_id/rename", h.RenameFile)
	// POST /api/tutor/chapter/:chapter_id/files/swap  章节侧换挂（挂新 + 旧回库，spec B6）
	g.POST("/chapter/:chapter_id/files/swap", h.SwapChapterFile)
	// DELETE /api/tutor/files/:file_id  删除文件
	g.DELETE("/files/:file_id", h.DeleteFile)
}

// InitUpload 创建上传会话 POST /api/tutor/files/upload
// @Summary 创建分片上传会话
// @Description 导师个人文件库分片上传四段契约之 init：登记归属导师与预期大小/类型（spec A1）。校验单文件 ≤2GB、类型白名单四档、分片 5–8MiB、片数 ≤10,000
// @Tags 讲师端-文件库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body tutorfile.InitUploadRequest true "会话参数"
// @Success 200 {object} response.R{data=tutorfile.InitUploadResponse} "success"
// @Failure 400 {object} response.R "参数越界"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/files/upload [post]
func (h *handler) InitUpload(c *gin.Context) {
	var req InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	uid := middleware.CurrentUserID(c)
	sid, err := h.svc.InitUpload(c.Request.Context(), uid, req.FileName, req.FileSize, req.ChunkSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, InitUploadResponse{
		SessionID:   sid,
		ChunkSize:   req.ChunkSize,
		TotalChunks: totalChunksFor(req.FileSize, req.ChunkSize),
	})
}

// GetUploadSession 断点续传清单 GET /api/tutor/files/upload/:session_id
// @Summary 断点续传清单
// @Description 返回会话状态与已收分片序号（spec A8，= S3 ListParts 语义）；客户端凭清单只补缺片，服务端以会话状态为准
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "会话 ID"
// @Success 200 {object} response.R{data=tutorfile.SessionStatusDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "非本人会话"
// @Failure 404 {object} response.R "会话不存在"
// @Router /tutor/files/upload/{session_id} [get]
func (h *handler) GetUploadSession(c *gin.Context) {
	sid := c.Param("session_id")
	uid := middleware.CurrentUserID(c)
	row, parts, err := h.svc.GetUploadSession(c.Request.Context(), uid, sid)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, toSessionStatusDTO(row, parts))
}

// UploadChunk 上传分片 PUT /api/tutor/files/upload/:session_id/chunk/:part_number
// @Summary 上传分片
// @Description 四段契约之 chunk：body 为原始分片字节（spec A1/A8）。同序号重传幂等覆盖；除末片外各片须恰为 init 声明的 chunk_size
// @Tags 讲师端-文件库
// @Accept octet-stream
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "会话 ID"
// @Param part_number path int true "分片序号（1 起）"
// @Success 200 {object} response.R{data=tutorfile.UploadChunkResponse} "success"
// @Failure 400 {object} response.R "参数越界"
// @Failure 404 {object} response.R "会话不存在"
// @Failure 409 {object} response.R "会话状态不允许"
// @Router /tutor/files/upload/{session_id}/chunk/{part_number} [put]
func (h *handler) UploadChunk(c *gin.Context) {
	sid := c.Param("session_id")
	partNumber, err := httpx.PathInt(c, "part_number", "分片序号无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 单片上限 8MiB（A2）；超出直接拒绝，不把 9MiB 读进内存。
	body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxChunkSize)
	content, err := io.ReadAll(body)
	if err != nil {
		response.BadRequest(c, "分片内容无效或超过单片上限")
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.UploadChunk(c.Request.Context(), uid, sid, partNumber, content); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, UploadChunkResponse{PartNumber: partNumber, Size: int64(len(content))})
}

// CompleteUpload 拼装产出文件行 POST /api/tutor/files/upload/:session_id/complete
// @Summary 完成上传
// @Description 四段契约之 complete：按分片序号拼装落存储、产出文件库行（spec A1/A5）。缺片、abort/已完成会话拒绝
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "会话 ID"
// @Success 200 {object} response.R{data=tutorfile.FileDTO} "success"
// @Failure 400 {object} response.R "缺片或参数不合法"
// @Failure 404 {object} response.R "会话不存在"
// @Failure 409 {object} response.R "会话状态不允许"
// @Router /tutor/files/upload/{session_id}/complete [post]
func (h *handler) CompleteUpload(c *gin.Context) {
	sid := c.Param("session_id")
	uid := middleware.CurrentUserID(c)
	file, err := h.svc.CompleteUpload(c.Request.Context(), uid, sid)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, toFileDTO(*file, nil))
}

// AbortUpload 放弃上传会话 POST /api/tutor/files/upload/:session_id/abort
// @Summary 放弃上传会话
// @Description 四段契约之 abort：清理半成品（spec A1）。重复调用幂等；已完成会话拒绝
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "会话 ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "会话不存在"
// @Failure 409 {object} response.R "会话状态不允许"
// @Router /tutor/files/upload/{session_id}/abort [post]
func (h *handler) AbortUpload(c *gin.Context) {
	sid := c.Param("session_id")
	uid := middleware.CurrentUserID(c)
	if err := h.svc.AbortUpload(c.Request.Context(), uid, sid); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "上传已放弃", nil)
}

// ListFiles 文件库列表 GET /api/tutor/files
// @Summary 导师文件库列表
// @Description 当前导师的文件库行（新→旧）；支持类型/挂接状态/创建时间区间筛选（spec B7）
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param content_type query string false "类型筛选（video/document/ppt/image）"
// @Param attached query bool false "挂接状态筛选（true=已挂 / false=未挂）"
// @Param created_from query string false "创建时间下界（RFC3339 或 YYYY-MM-DD，闭区间）"
// @Param created_to query string false "创建时间上界（RFC3339 或 YYYY-MM-DD，闭区间；日期按当天末）"
// @Success 200 {object} response.R{data=tutorfile.FileListDTO} "success"
// @Failure 400 {object} response.R "筛选参数无效"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/files [get]
func (h *handler) ListFiles(c *gin.Context) {
	filter, err := parseFileFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	files, err := h.svc.ListFiles(c.Request.Context(), uid, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	titles := h.svc.ChapterTitles(c.Request.Context(), files)
	dtos := make([]FileDTO, 0, len(files))
	for _, f := range files {
		dtos = append(dtos, toFileDTO(f, titles))
	}
	response.Success(c, FileListDTO{Files: dtos})
}

// AttachFile 单挂挂接 POST /api/tutor/files/:file_id/attach
// @Summary 挂接文件到章节
// @Description 单挂挂接（spec B2）：一个文件同一时间只挂一章；已挂其他章拒绝并明示（先解挂再挂）
// @Tags 讲师端-文件库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件 ID"
// @Param body body tutorfile.AttachRequest true "章节参数"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数无效"
// @Failure 403 {object} response.R "非本人文件"
// @Failure 404 {object} response.R "文件或章节不存在"
// @Failure 409 {object} response.R "已挂接到其他章节"
// @Router /tutor/files/{file_id}/attach [post]
func (h *handler) AttachFile(c *gin.Context) {
	fileID, err := httpx.PathInt(c, "file_id", "文件ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var req AttachRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ChapterID <= 0 {
		response.BadRequest(c, "请求数据无效")
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.AttachFile(c.Request.Context(), uid, fileID, req.ChapterID); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已挂接到章节", nil)
}

// DetachFile 取消挂接回库 POST /api/tutor/files/:file_id/detach
// @Summary 取消挂接
// @Description 取消挂接回库（spec B2）：回到在库未挂状态；重复调用幂等
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件 ID"
// @Success 200 {object} response.R "success"
// @Failure 403 {object} response.R "非本人文件"
// @Failure 404 {object} response.R "文件不存在"
// @Router /tutor/files/{file_id}/detach [post]
func (h *handler) DetachFile(c *gin.Context) {
	fileID, err := httpx.PathInt(c, "file_id", "文件ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.DetachFile(c.Request.Context(), uid, fileID); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已取消挂接，文件回到文件库", nil)
}

// RenameFile 改名 POST /api/tutor/files/:file_id/rename
// @Summary 文件改名
// @Description 改显示名（spec B7）；存储 key 与命名契约不动
// @Tags 讲师端-文件库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件 ID"
// @Param body body tutorfile.RenameRequest true "新名称"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "名称为空"
// @Failure 403 {object} response.R "非本人文件"
// @Failure 404 {object} response.R "文件不存在"
// @Router /tutor/files/{file_id}/rename [post]
func (h *handler) RenameFile(c *gin.Context) {
	fileID, err := httpx.PathInt(c, "file_id", "文件ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.RenameFile(c.Request.Context(), uid, fileID, req.FileName); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已改名", nil)
}

// SwapChapterFile 章节侧换挂 POST /api/tutor/chapter/:chapter_id/files/swap
// @Summary 章节侧换文件
// @Description 换挂发生在章节侧（spec B6）：挂新文件、旧文件回库，一步原子；库侧无「替换」动作
// @Tags 讲师端-文件库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节 ID"
// @Param body body tutorfile.SwapRequest true "新旧文件"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数无效"
// @Failure 403 {object} response.R "非本人文件"
// @Failure 404 {object} response.R "文件不存在"
// @Failure 409 {object} response.R "旧文件未挂该章或新文件已挂别章"
// @Router /tutor/chapter/{chapter_id}/files/swap [post]
func (h *handler) SwapChapterFile(c *gin.Context) {
	chapterID, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OldFileID <= 0 || req.NewFileID <= 0 {
		response.BadRequest(c, "请求数据无效")
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.SwapChapterFile(c.Request.Context(), uid, chapterID, req.OldFileID, req.NewFileID); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已换文件，旧文件回到文件库", nil)
}

// DeleteFile 删除文件 DELETE /api/tutor/files/:file_id
// @Summary 删除文件库文件
// @Description 删除文件行与存储对象（仅归属导师）
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Param file_id path int true "文件 ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "文件不存在"
// @Router /tutor/files/{file_id} [delete]
func (h *handler) DeleteFile(c *gin.Context) {
	fileID, err := httpx.PathInt(c, "file_id", "文件ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.svc.DeleteFile(c.Request.Context(), uid, fileID); err != nil {
		writeServiceError(c, err)
		return
	}
	response.SuccessWithMsg(c, "文件已删除", nil)
}

// parseFileFilter 解析管理面筛选查询参数（spec B7）：
// content_type 精确档位；attached=true/false；created_from/to 支持 RFC3339 或 YYYY-MM-DD
// （日期形的上界按当天末，闭区间）。无效参数报错由调用方转 400。
func parseFileFilter(c *gin.Context) (FileFilter, error) {
	var f FileFilter
	f.ContentType = c.Query("content_type")
	if v := c.Query("attached"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return f, errors.New("attached 参数无效")
		}
		f.Attached = &b
	}
	from, err := parseFilterTime(c.Query("created_from"), false)
	if err != nil {
		return f, err
	}
	to, err := parseFilterTime(c.Query("created_to"), true)
	if err != nil {
		return f, err
	}
	f.CreatedFrom, f.CreatedTo = from, to
	return f, nil
}

// parseFilterTime 时间筛选值解析：RFC3339 或 YYYY-MM-DD（endOfDay 时日期按当天 23:59:59）。
func parseFilterTime(v string, endOfDay bool) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		if endOfDay {
			return t.Add(24*time.Hour - time.Second), nil
		}
		return t, nil
	}
	return time.Time{}, errors.New("时间筛选参数无效（RFC3339 或 YYYY-MM-DD）")
}

// writeServiceError 服务哨兵错误 → 状态码映射（本域错误面单点）。
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, ErrInvalidState):
		response.Conflict(c, err.Error())
	case errors.Is(err, ErrInvalidParam):
		response.BadRequest(c, err.Error())
	case errors.Is(err, ErrQuotaFileCount), errors.Is(err, ErrQuotaTotalSize):
		// 配额双臂（spec B3）：400 沿投稿配额先例，文案自带当前用量。
		response.BadRequest(c, err.Error())
	case errors.Is(err, ErrFileInUse):
		// 删除随引用走（spec B5）：409 状态冲突，文案自带解挂指引。
		response.Conflict(c, err.Error())
	case errors.Is(err, course.ErrChapterNotFound):
		response.NotFound(c, err.Error())
	default:
		response.ServerErrorCause(c, "文件库操作失败", err)
	}
}
