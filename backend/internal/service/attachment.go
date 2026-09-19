// Package service 实现业务服务层。
// 本文件：附件归属单 module（第十二波票 4，#1168）——前缀登记 + 本站判定 + URL→key + multipart 读取四件事全收。
// 此前同一判据有五份互不一致的写法（isForumImageURL / isAIImageURL / isFeaturedImageURL 三份本站判定，
// forumImageKey / contributionURLKey 两份 URL→key），由本 module 单点取代；判定语义与三份 is* 原文一致
// （R2 形态接受任意域名 + scheme 守卫），漂移从此只在一处修。
package service

import (
	"io"
	"mime/multipart"
	"strings"
)

// 附件目录前缀登记（单一事实源）：各域声明自己的前缀，新增上传子目录在此登记。
const (
	ForumImageDirPrefix         = "images/forum"        // 论坛图片：上传 + 写面门禁 + 悬空回收
	AIAssistantImageDirPrefix   = "images/ai-assistant" // AI 助手对话图片
	FeaturedImageDirPrefix      = "featured"            // 内容精选封面与正文内嵌图
	ContributionFileDirPrefix   = "contributions"       // 投稿附件文件
	AvatarImageDirPrefix        = "avatars"             // 头像（走审核队列）
	ResumeFileDirPrefix         = "resumes"             // 简历 PDF
	ResumeImageDirPrefix        = "resumes/images"      // 简历工作照与证书原图
	QuestionImageDirPrefix      = "images/questions"    // 题库题目图片
	ChapterImageDirPrefix       = "images/chapters"     // 章节图文图片（可按章节分目录 images/chapters/<id>）
	ChapterFileDirPrefix        = "chapters"            // 章节课件附件
	localStaticUploadsURLPrefix = "/static/uploads/"    // local 存储 URL 前缀（storage.LocalStorage.Save 的返回形态）
)

// IsSiteAttachmentURL 判断 u 是否为本站在指定前缀下产生的附件 URL。两种形态：
//   - local：/static/uploads/<prefix>/...
//   - R2：  http(s)://<任意域名>/.../<prefix>/...
//
// 大小写敏感；根相对但非 /static/uploads/ 开头（如 /images/forum/x）判非本站。
func IsSiteAttachmentURL(u, prefix string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	if strings.HasPrefix(u, localStaticUploadsURLPrefix+prefix+"/") {
		return true
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	return strings.Index(u, "/"+prefix+"/") > 0
}

// AttachmentKey 从 URL 中提取以 prefix 起始的对象 key（兼容 local 与 R2 两种形态，
// 与 storage 各实现的 urlToKey 口径一致）；未命中前缀返回空串。
func AttachmentKey(u, prefix string) string {
	idx := strings.Index(u, prefix+"/")
	if idx < 0 {
		return ""
	}
	return u[idx:]
}

// ReadMultipartFile 读取 multipart 文件头的全部内容（Open + ReadAll 单点，供各上传 handler 复用）。
func ReadMultipartFile(fh *multipart.FileHeader) ([]byte, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return io.ReadAll(src)
}
