// Package service 共享的文件清理辅助函数。
// 本站图片归属判定已收编附件归属 module（attachment.go：IsSiteAttachmentURL）。
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
