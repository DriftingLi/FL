// 文件类型表的两条锁（ADR-0066 决策 6）。
//
// 锁①：可上传集合里的每个扩展名必须显式表态为 safe —— 新增可上传类型时，
//
//	「它会不会被浏览器当可执行文档渲染」这个问题必须被回答一次。
//
// 锁②：可执行/脚本类型必须显式登记为 unsafe 且不可上传 —— 不许靠「没登记」蒙混
//
//	（未登记走 unknown，同样强制下载，但「考虑过它」这件事必须留在表里）。
package service

import (
	"testing"

	"go.uber.org/zap"
)

// executableExtensions 认「浏览器会把它当可执行文档渲染」这一族（含常见变体）。
var executableExtensions = []string{"svg", "svgz", "html", "htm", "xhtml", "xml", "js", "mjs", "cjs"}

func TestFileTypeTable_UploadableMustBeSafe(t *testing.T) {
	for ext, e := range fileTypeTable {
		if e.uploadCategory == "" {
			continue
		}
		if e.class != FileTypeSafe {
			t.Errorf("可上传扩展名 %q 的分档 = %v，必须为 FileTypeSafe（ADR-0066 决策 1/6）", ext, e.class)
		}
	}
}

func TestFileTypeTable_ExecutableExtensionsAreUnsafeAndNotUploadable(t *testing.T) {
	for _, ext := range executableExtensions {
		e, ok := fileTypeTable[ext]
		if !ok {
			t.Errorf("可执行类型 %q 未在类型表里显式表态（unsafe 也必须登记，ADR-0066 决策 6）", ext)
			continue
		}
		if e.class != FileTypeUnsafe {
			t.Errorf("%q 的分档 = %v，期望 FileTypeUnsafe", ext, e.class)
		}
		if e.uploadCategory != "" {
			t.Errorf("%q 竟可上传（category = %q）", ext, e.uploadCategory)
		}
	}
}

// TestFileTypeTable_SvgNotUploadableAndUnsafe 是本次缺陷的直接判据：
// 匿名上传曾接受 svg，而 svg 与门户同源 ⇒ 存储型 XSS。
func TestFileTypeTable_SvgNotUploadableAndUnsafe(t *testing.T) {
	if got := UploadCategoryOf("svg"); got != "" {
		t.Fatalf("svg 必须不在上传白名单里，实得 category = %q", got)
	}
	if got := FileTypeClassOf("svg"); got != FileTypeUnsafe {
		t.Fatalf("svg 的分档 = %v，期望 FileTypeUnsafe", got)
	}
}

func TestFileTypeTable_UnknownClassAndImageCategory(t *testing.T) {
	if got := FileTypeClassOf("mystery"); got != FileTypeUnknown {
		t.Errorf("未登记扩展名应为 FileTypeUnknown，实得 %v", got)
	}
	if got := FileTypeClassOf(""); got != FileTypeUnknown {
		t.Errorf("无扩展名应为 FileTypeUnknown，实得 %v", got)
	}
	if got := UploadCategoryOf("png"); got != "image" {
		t.Errorf("png 的上传类别 = %q，期望 image", got)
	}
	if got := UploadCategoryOf("pdf"); got != "document" {
		t.Errorf("pdf 的上传类别 = %q，期望 document", got)
	}
}

// TestValidateImageRejectsSvg 走的是上传侧唯一闸门的公开行为：
// 五个上传端点共用 ValidateImage，它必须拒掉 svg。
func TestValidateImageRejectsSvg(t *testing.T) {
	fs := NewFileStore("", nil, zap.NewNop())
	if ok, msg := fs.ValidateImage("evil.svg", 1024); ok {
		t.Fatalf("ValidateImage 接受了 svg（msg = %q），ADR-0066 决策 1 要求拒绝", msg)
	}
	if ok, _ := fs.ValidateImage("ok.png", 1024); !ok {
		t.Fatal("ValidateImage 拒绝了 png，白名单被误伤")
	}
}
