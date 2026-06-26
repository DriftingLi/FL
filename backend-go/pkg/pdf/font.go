// Package pdf 实现 PDF 报告生成
// 本文件：字体加载器，负责从 assets/fonts 目录加载 simhei.ttf 并注册到 gofpdf
package pdf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jung-kurt/gofpdf"
)

// 字体常量
const (
	// FontSimHei 黑体（用于正文，支持中文）
	FontSimHei = "simhei"
	// FontSimHeiBold 黑体（用于标题，加粗样式）
	FontSimHeiBold = "simhei_b"

	// fontFile 字体文件相对路径（相对于可执行文件或工作目录）
	fontFile = "assets/fonts/simhei.ttf"
)

var (
	// bytesOnce 读取字体文件字节只执行一次（跨 Fpdf 实例共享）
	bytesOnce sync.Once
	// fontBytes 缓存字体文件字节
	fontBytes []byte
	// bytesErr 读取过程中遇到的错误
	bytesErr error
)

// ensureFontLoaded 确保字体已注册到给定的 Fpdf 实例
//
// 注意：gofpdf 的字体注册是 per-instance 的（每个 Fpdf 都有自己的字体表），
// 因此每次新建 PDF 文档时都必须重新注册。这里用 sync.Once 仅缓存磁盘读取结果，
// 避免每个 Fpdf 实例都重新读盘 9.7MB 的 TTF 文件。
func ensureFontLoaded(pdf *gofpdf.Fpdf) error {
	// 1. 读盘：只发生一次
	bytesOnce.Do(func() {
		path, err := findFontFile()
		if err != nil {
			bytesErr = err
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			bytesErr = fmt.Errorf("读取字体文件失败 (%s): %w", path, err)
			return
		}
		if len(data) == 0 {
			bytesErr = fmt.Errorf("字体文件为空: %s", path)
			return
		}
		fontBytes = data
	})
	if bytesErr != nil {
		return fmt.Errorf("字体加载失败: %w", bytesErr)
	}
	if len(fontBytes) == 0 {
		return errors.New("字体字节为空")
	}

	// 2. 注册到当前 Fpdf 实例（gofpdf 内部对同 family+style 的字体去重）
	pdf.AddUTF8FontFromBytes(FontSimHei, "", fontBytes)
	pdf.AddUTF8FontFromBytes(FontSimHeiBold, "B", fontBytes)
	return nil
}

// findFontFile 查找字体文件
// 按以下顺序查找：
//  1. 与可执行文件同目录的 assets/fonts/simhei.ttf
//  2. 当前工作目录的 assets/fonts/simhei.ttf
//  3. 常见相对路径（backend 目录、上一级目录等）
func findFontFile() (string, error) {
	candidates := []string{}

	// 可执行文件目录
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), fontFile))
	}
	// 工作目录
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, fontFile))
		// go run 场景下，cwd 是 backend 目录
		candidates = append(candidates, filepath.Join(cwd, "assets", "fonts", "simhei.ttf"))
		// 若从项目根目录运行，尝试进入 backend 子目录
		candidates = append(candidates, filepath.Join(cwd, "backend", "assets", "fonts", "simhei.ttf"))
	}
	// 兜底绝对路径（与开发机一致）
	candidates = append(candidates,
		`d:\叉车残值评估系统\backend\assets\fonts\simhei.ttf`,
		`d:/叉车残值评估系统/backend/assets/fonts/simhei.ttf`,
		"backend/assets/fonts/simhei.ttf",
		"../backend/assets/fonts/simhei.ttf",
	)

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", errors.New("未找到字体文件 assets/fonts/simhei.ttf，请确认 backend/assets/fonts/simhei.ttf 存在")
}
