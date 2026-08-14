// Package api 实现 HTTP handlers。
// 本文件：Vditor 图片上传适配器——File 读取 → 校验 → 保存 → Vditor 信封响应的单点实现。
// tutor.go 与 featured.go 的 UploadImage 只注入保存目标（saver）+ 各自差异，信封协议不再复制。
package api

import (
	"io"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
)

// vditorUploadSaver 上传保存目标：content → 返回可访问 URL。
// tutor 按章节分目录存储（images/chapters/<id>），featured 存封面目录（SaveImage），差异经此 hook 注入。
type vditorUploadSaver func(content []byte, filename string) (string, error)

// vditorError 构造 Vditor 期望的错误响应体（协议单点：code=1 + errFiles/succMap 信封）。
func vditorError(msg string, errFiles []string) gin.H {
	return gin.H{"msg": msg, "code": 1, "data": gin.H{"errFiles": errFiles, "succMap": map[string]string{}}}
}

// uploadVditorImage 统一的 Vditor 图片上传适配器。
// 公共骨架：FormFile 读取 → 空文件名守卫 → Open/ReadAll → ValidateImageFile → 保存（saver 注入）→ 信封。
// 返回 Vditor 期望格式：{ msg:"", code:0, data:{ errFiles:[], succMap:{"name":"url"} } }。
func uploadVditorImage(c *gin.Context, fileSvc *service.FileService, saver vditorUploadSaver) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(200, vditorError("未找到上传文件", []string{}))
		return
	}
	if file.Filename == "" {
		c.JSON(200, vditorError("未选择文件", []string{}))
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(200, vditorError("文件打开失败", []string{file.Filename}))
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		c.JSON(200, vditorError("文件读取失败", []string{file.Filename}))
		return
	}
	if ok, msg := fileSvc.ValidateImageFile(file.Filename, file.Size); !ok {
		c.JSON(200, vditorError(msg, []string{file.Filename}))
		return
	}
	url, err := saver(content, file.Filename)
	if err != nil {
		c.JSON(200, vditorError("文件保存失败", []string{file.Filename}))
		return
	}
	c.JSON(200, gin.H{
		"msg":  "",
		"code": 0,
		"data": gin.H{
			"errFiles": []string{},
			"succMap":  map[string]string{file.Filename: url},
		},
	})
}
