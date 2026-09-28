// 文件类型表（ADR-0066）：上传白名单与静态投递分档的**唯一**事实源。
//
// 每项两列：
//
//	class          —— 静态投递分档（safe 内联 / unsafe 强制下载 / 未登记 = unknown 同样强制下载）
//	uploadCategory —— 允许的上传类别（"" = 不可上传；取值与 maxFileSizes 的键对齐）
//
// 为什么 unsafe 也要登记：ADR-0066 决策 6 要求「考虑过它」这件事留在表里 ——
// 未登记走 unknown、行为相同，但下一个读者会问「svg 到底想过没有」。
package service

import (
	"sort"
	"strings"
)

// FileTypeClass 静态投递分档。
type FileTypeClass int

const (
	// FileTypeUnknown 未登记扩展名（含无扩展名）：一律强制下载 + octet-stream。
	FileTypeUnknown FileTypeClass = iota
	// FileTypeSafe 浏览器不会把它当可执行文档渲染：保持内联（PDF 内联预览依赖它）。
	FileTypeSafe
	// FileTypeUnsafe 已知可执行/脚本类型：强制下载 + octet-stream。
	FileTypeUnsafe
)

// fileTypeEntry 类型表一行。
type fileTypeEntry struct {
	class          FileTypeClass
	uploadCategory string
}

// fileTypeTable 见文件头。键是小写、不带点的扩展名。
var fileTypeTable = map[string]fileTypeEntry{
	// ===== 可上传：文档 / 幻灯片 / 视频 / 图片（全部 safe）=====
	"pdf":  {FileTypeSafe, "document"},
	"ppt":  {FileTypeSafe, "ppt"},
	"pptx": {FileTypeSafe, "ppt"},
	"mp4":  {FileTypeSafe, "video"},
	"webm": {FileTypeSafe, "video"},
	"png":  {FileTypeSafe, "image"},
	"jpg":  {FileTypeSafe, "image"},
	"jpeg": {FileTypeSafe, "image"},
	"gif":  {FileTypeSafe, "image"},
	"webp": {FileTypeSafe, "image"},
	"bmp":  {FileTypeSafe, "image"},

	// ===== 投稿白名单的其余扩展名：走投稿自己的校验（allowedContributionExt），
	//       静态面按 safe 保持既有 Content-Type（浏览器不会执行它们）=====
	"doc":  {FileTypeSafe, ""},
	"docx": {FileTypeSafe, ""},
	"xls":  {FileTypeSafe, ""},
	"xlsx": {FileTypeSafe, ""},
	"zip":  {FileTypeSafe, ""},

	// ===== 可执行/脚本类型：显式登记为 unsafe 且不可上传 =====
	"svg":   {FileTypeUnsafe, ""},
	"svgz":  {FileTypeUnsafe, ""},
	"html":  {FileTypeUnsafe, ""},
	"htm":   {FileTypeUnsafe, ""},
	"xhtml": {FileTypeUnsafe, ""},
	"xml":   {FileTypeUnsafe, ""},
	"js":    {FileTypeUnsafe, ""},
	"mjs":   {FileTypeUnsafe, ""},
	"cjs":   {FileTypeUnsafe, ""},
}

// normExt 归一扩展名入参：去点、转小写。
func normExt(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// FileTypeClassOf 返回扩展名的静态投递分档；未登记 = FileTypeUnknown。
func FileTypeClassOf(ext string) FileTypeClass {
	if e, ok := fileTypeTable[normExt(ext)]; ok {
		return e.class
	}
	return FileTypeUnknown
}

// FileTypeClassOfPath 同 FileTypeClassOf，入参是文件名或 URL 路径。
func FileTypeClassOfPath(path string) FileTypeClass {
	return FileTypeClassOf(fileExtension(path))
}

// UploadCategoryOf 返回扩展名允许的上传类别（"" = 不在上传白名单）。
func UploadCategoryOf(ext string) string {
	return fileTypeTable[normExt(ext)].uploadCategory
}

// AllowedExtensionsFor 返回某上传类别的允许扩展名（字母序，供错误文案与断言用）。
func AllowedExtensionsFor(category string) []string {
	out := make([]string, 0, 8)
	for ext, e := range fileTypeTable {
		if e.uploadCategory == category {
			out = append(out, ext)
		}
	}
	sort.Strings(out)
	return out
}
