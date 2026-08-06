// Package service 共享的文件清理辅助函数。
package service

import (
	"regexp"
	"strings"
)

// markdownImageRe 匹配 Markdown 图片语法 ![](url)。
var markdownImageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// markdownImageURLs 从 Markdown 文本中提取全部图片 URL（去重、去空白）。
func markdownImageURLs(content string) []string {
	if content == "" {
		return nil
	}
	seen := map[string]bool{}
	var urls []string
	for _, m := range markdownImageRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		u := strings.TrimSpace(m[1])
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	return urls
}

// isFeaturedImageURL 判断 URL 是否指向本站 featured/ 子目录。
// local：/static/uploads/featured/xxx；R2：https://<domain>/featured/xxx。
func isFeaturedImageURL(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	if strings.HasPrefix(u, "/static/uploads/featured/") {
		return true
	}
	idx := strings.Index(u, "/featured/")
	return idx > 0 && (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://"))
}
