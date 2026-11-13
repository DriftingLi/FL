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

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
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
	// PUT /api/tutor/files/upload/:session_id/chunk/:part_number  上传分片
	g.PUT("/files/upload/:session_id/chunk/:part_number", h.UploadChunk)
	// POST /api/tutor/files/upload/:session_id/complete  拼装产出文件行
	g.POST("/files/upload/:session_id/complete", h.CompleteUpload)
	// POST /api/tutor/files/upload/:session_id/abort  放弃会话
	g.POST("/files/upload/:session_id/abort", h.AbortUpload)
	// GET /api/tutor/files  文件库列表
	g.GET("/files", h.ListFiles)
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
	response.Success(c, toFileDTO(*file))
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
// @Description 当前导师的文件库行（新→旧）
// @Tags 讲师端-文件库
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=tutorfile.FileListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /tutor/files [get]
func (h *handler) ListFiles(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	files, err := h.svc.ListFiles(c.Request.Context(), uid)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	dtos := make([]FileDTO, 0, len(files))
	for _, f := range files {
		dtos = append(dtos, toFileDTO(f))
	}
	response.Success(c, FileListDTO{Files: dtos})
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
	default:
		response.ServerErrorCause(c, "文件库操作失败", err)
	}
}
