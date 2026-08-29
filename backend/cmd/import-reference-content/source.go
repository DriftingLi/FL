// 资料文件发现与分类：题库文件 / 知识文章 / 其他（跳过）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	kindQuestion = "question"
	kindArticle  = "article"
	kindOther    = "other"
)

// 资料顶层目录 → 系统证件 code（CONTEXT.md：课程与题库按 credential 单归属分区）。
var categoryCredential = map[string]string{
	"N1":    "forklift_n1",
	"场厂内":   "forklift_n1",
	"一级":    "maintenance_L1",
	"二级":    "maintenance_L2",
	"三级":    "maintenance_L3",
	"四级":    "maintenance_L4",
	"五级":    "maintenance_L5",
	"低压电工证": "low_voltage_electrician",
	"焊工证":   "welder",
	"基础知识":  "forklift_n1", // 混入基础知识目录的题目文件均为 N1 相关
}

// SourceFile 单个资料文件的分类结果。
type SourceFile struct {
	Path     string
	Category string            // 顶层目录名
	Name     string            // 文件名（含扩展名）
	FM       map[string]string // frontmatter 键值（无 frontmatter 则为空）
	Kind     string
	RealExam bool // 真题源：material_type=真题 或文件名含「真题/历年」
	Reason   string
}

// answerMarkRe 答案标记行（**答案：** 系列 / 正确答案：），用于题库文件判定。
var answerMarkRe = regexp.MustCompile(`(?m)^\s*(\*\*答案|正确答案)`)

// discoverFiles 扫描 Markdown 根目录（跳过「电子书」，其下无 md，内容为 PDF 书籍），
// 逐文件分类。返回按路径排序的文件清单与按「分类|类型」的计数。
func discoverFiles(root string) ([]*SourceFile, map[string]int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, fmt.Errorf("读取资料目录失败: %w", err)
	}
	var files []*SourceFile
	counts := map[string]int{}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "电子书" {
			continue
		}
		dir := filepath.Join(root, e.Name())
		mdFiles, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("读取分类目录 %s 失败: %w", e.Name(), err)
		}
		for _, f := range mdFiles {
			if f.IsDir() || !strings.EqualFold(filepath.Ext(f.Name()), ".md") {
				continue
			}
			sf := &SourceFile{
				Path:     filepath.Join(dir, f.Name()),
				Category: e.Name(),
				Name:     f.Name(),
			}
			classifyFile(sf)
			files = append(files, sf)
			counts[sf.Category+"|"+sf.Kind]++
		}
	}
	// 排序保证多次运行的处理顺序稳定（去重保留策略依赖确定顺序）。
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, counts, nil
}

// credentialFor 按资料分类返回目标证件 code。
func credentialFor(sf *SourceFile) string {
	return categoryCredential[sf.Category]
}

// classifyFile 就地填充 Kind / RealExam / Reason。
func classifyFile(sf *SourceFile) {
	data, err := os.ReadFile(sf.Path)
	if err != nil {
		sf.Kind = kindOther
		sf.Reason = "读取失败: " + err.Error()
		return
	}
	body, fm := splitFrontmatter(string(data))
	sf.FM = fm
	sf.RealExam = isRealExamSource(sf.Name, fm)
	markCount := len(answerMarkRe.FindAllString(body, -1))
	switch {
	case markCount >= 2: // 阈值 2：可捞回付费墙预览仅抓到几题的 PARTIAL 题库文件
		sf.Kind = kindQuestion
	case fm["knowledge_point"] != "":
		sf.Kind = kindArticle
	default:
		sf.Kind = kindOther
		sf.Reason = skipReasonFor(sf)
	}
}

// isRealExamSource 真题源判定：frontmatter material_type=真题，或文件名含「真题/历年」。
func isRealExamSource(name string, fm map[string]string) bool {
	if strings.TrimSpace(fm["material_type"]) == "真题" {
		return true
	}
	return strings.Contains(name, "真题") || strings.Contains(name, "历年")
}

// splitFrontmatter 拆分文件头 YAML frontmatter（平铺 key: "value"），返回正文与键值。
func splitFrontmatter(data string) (string, map[string]string) {
	fm := map[string]string{}
	rest := strings.TrimPrefix(data, "\ufeff")
	lines := strings.Split(rest, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return rest, fm
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return rest, fm
	}
	for _, line := range lines[1:end] {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		if key != "" {
			fm[key] = val
		}
	}
	return strings.Join(lines[end+1:], "\n"), fm
}

// skipReasonFor 非题库非文章文件的归类原因（报告用）。
func skipReasonFor(sf *SourceFile) string {
	mt := sf.FM["material_type"]
	switch {
	case strings.Contains(mt, "攻略"):
		return "备考/实操攻略"
	case strings.Contains(mt, "视频"):
		return "视频资料"
	case strings.Contains(mt, "词条"), strings.Contains(mt, "书籍"):
		return "书籍百科词条"
	case strings.Contains(sf.Name, "大纲"):
		return "考试大纲文件"
	case strings.Contains(sf.Name, "安全工程师"):
		return "非叉车域（注册安全工程师）"
	default:
		return "未识别类型（无答案标记、无知识点）"
	}
}

// readFileBody 读取正文（frontmatter 已拆分）。
func readFileBody(sf *SourceFile) (string, error) {
	data, err := os.ReadFile(sf.Path)
	if err != nil {
		return "", err
	}
	body, _ := splitFrontmatter(string(data))
	return body, nil
}
