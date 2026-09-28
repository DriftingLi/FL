// 文件类型表（ADR-0066）：上传白名单、静态投递分档与 MIME 推断的**唯一**事实源。
//
// 每项三列：
//
//	class  —— 静态投递分档（safe 内联 / unsafe 强制下载 / 未登记 = unknown 同样强制下载）
//	upload —— 允许的上传类别（"" = 不可上传；取值与 maxFileSizes 的键对齐）
//	mime   —— 该扩展名的 MIME 类型（Save/Read 落库与回传用）
//
// 为什么 unsafe 也要登记：ADR-0066 决策 6 要求「考虑过它」这件事留在表里 ——
// 未登记走 unknown、行为相同，但下一个读者会问「svg 到底想过没有」。
//
// 关于 upload="" 但 class=safe 的行：它们由**投稿域自己的白名单**（allowedContributionExt）放行，
// 不走上传校验的 image/document 闸门；登记在此只为静态投递分档有明确答案（否则落 unknown ⇒ 强制下载）。
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
	class  FileTypeClass
	upload string
	mime   string
}

// fileTypeTable 见文件头。键是小写、不带点的扩展名。
var fileTypeTable = map[string]fileTypeEntry{
	// ===== 可上传：文档 / 幻灯片 / 视频 / 图片（全部 safe）=====
	"pdf":  {class: FileTypeSafe, upload: "document", mime: "application/pdf"},
	"ppt":  {class: FileTypeSafe, upload: "ppt", mime: "application/vnd.ms-powerpoint"},
	"pptx": {class: FileTypeSafe, upload: "ppt", mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	"mp4":  {class: FileTypeSafe, upload: "video", mime: "video/mp4"},
	"webm": {class: FileTypeSafe, upload: "video", mime: "video/webm"},
	"png":  {class: FileTypeSafe, upload: "image", mime: "image/png"},
	"jpg":  {class: FileTypeSafe, upload: "image", mime: "image/jpeg"},
	"jpeg": {class: FileTypeSafe, upload: "image", mime: "image/jpeg"},
	"gif":  {class: FileTypeSafe, upload: "image", mime: "image/gif"},
	"webp": {class: FileTypeSafe, upload: "image", mime: "image/webp"},
	"bmp":  {class: FileTypeSafe, upload: "image", mime: "image/bmp"},

	// ===== 投稿白名单的其余扩展名：投稿域自行校验，这里只为静态投递分档表态 =====
	"doc":  {class: FileTypeSafe, mime: "application/msword"},
	"docx": {class: FileTypeSafe, mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
	"xls":  {class: FileTypeSafe, mime: "application/vnd.ms-excel"},
	"xlsx": {class: FileTypeSafe, mime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	"zip":  {class: FileTypeSafe, mime: "application/zip"},

	// ===== 可执行/脚本类型：显式登记为 unsafe 且不可上传 =====
	"svg":   {class: FileTypeUnsafe, mime: "image/svg+xml"},
	"svgz":  {class: FileTypeUnsafe, mime: "image/svg+xml"},
	"html":  {class: FileTypeUnsafe, mime: "text/html"},
	"htm":   {class: FileTypeUnsafe, mime: "text/html"},
	"xhtml": {class: FileTypeUnsafe, mime: "application/xhtml+xml"},
	"xml":   {class: FileTypeUnsafe, mime: "application/xml"},
	"js":    {class: FileTypeUnsafe, mime: "text/javascript"},
	"mjs":   {class: FileTypeUnsafe, mime: "text/javascript"},
	"cjs":   {class: FileTypeUnsafe, mime: "text/javascript"},
}

// normExt 归一扩展名入参：去点、转小写。
func normExt(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// FileTypeClassOf 返回扩展名的静态投递分档；未登记 = FileTypeUnknown。
func FileTypeClassOf(ext string) FileTypeClass {
	return fileTypeTable[normExt(ext)].class
}

// FileTypeClassOfPath 同 FileTypeClassOf，入参是文件名或 URL 路径。
func FileTypeClassOfPath(path string) FileTypeClass {
	return FileTypeClassOf(fileExtension(path))
}

// UploadCategoryOf 返回扩展名允许的上传类别（"" = 不在上传白名单）。
func UploadCategoryOf(ext string) string {
	return fileTypeTable[normExt(ext)].upload
}

// MimeTypeOf 返回扩展名的 MIME 类型；未登记回落 application/octet-stream。
func MimeTypeOf(ext string) string {
	if m := fileTypeTable[normExt(ext)].mime; m != "" {
		return m
	}
	return "application/octet-stream"
}

// AllowedExtensionsFor 返回某上传类别的允许扩展名（字母序，供错误文案与断言用）。
func AllowedExtensionsFor(category string) []string {
	out := make([]string, 0, 8)
	for ext, e := range fileTypeTable {
		if e.upload == category {
			out = append(out, ext)
		}
	}
	sort.Strings(out)
	return out
}
