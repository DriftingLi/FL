// Package api 实现 HTTP handlers。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复）。
package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterForumRoutes 注册 /api/forum 蓝图（需登录，hrwai_user）。
func RegisterForumRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewForumService(db)

	g := rg.Group("/forum", middleware.JWTAuth(cfg), middleware.RoleRequired("hrwai_user"))

	// GET /api/forum/topics?scope=all|general|chapter&chapter_id=&page=&page_size=&keyword=
	g.GET("/topics", func(c *gin.Context) {
		scope := c.Query("scope")
		chapterID := atoiDefault(c.Query("chapter_id"), 0)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		result, err := svc.ListTopics(scope, chapterID, page, pageSize, c.Query("keyword"))
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/forum/topics 发帖（chapter_id 为空/0 表示发到综合讨论区）
	g.POST("/topics", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		var req struct {
			ChapterID *int   `json:"chapter_id"`
			Title     string `json:"title"`
			Content   string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		topic, err := svc.CreateTopic(userID, req.ChapterID, req.Title, req.Content)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "发布成功", topic)
	})

	// GET /api/forum/topics/:id 主题详情（含回复）
	g.GET("/topics/:id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		topicID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || topicID <= 0 {
			response.BadRequest(c, "主题ID无效")
			return
		}
		result, err := svc.GetTopic(topicID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.NotFound(c, "主题不存在")
				return
			}
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/forum/topics/:id/replies 回复
	g.POST("/topics/:id/replies", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		topicID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || topicID <= 0 {
			response.BadRequest(c, "主题ID无效")
			return
		}
		var req struct {
			Content       string `json:"content"`
			ParentReplyID *int64 `json:"parent_reply_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		reply, err := svc.ReplyTopic(userID, topicID, req.Content, req.ParentReplyID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "回复成功", reply)
	})

	// DELETE /api/forum/topics/:id 删除自己的主题
	g.DELETE("/topics/:id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		topicID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || topicID <= 0 {
			response.BadRequest(c, "主题ID无效")
			return
		}
		if err := svc.DeleteTopic(userID, topicID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已删除", nil)
	})

	// DELETE /api/forum/replies/:id 删除自己的回复
	g.DELETE("/replies/:id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		replyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || replyID <= 0 {
			response.BadRequest(c, "回复ID无效")
			return
		}
		if err := svc.DeleteReply(userID, replyID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已删除", nil)
	})
}
