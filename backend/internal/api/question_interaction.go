// Package api 实现 HTTP handlers。
// 本文件：题目互动（评论 + 笔记 + 考点标签）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// questionInteractionHandler 题目互动 handler。
type questionInteractionHandler struct {
	svc *service.QuestionInteractionService
}

// RegisterQuestionInteractionRoutes 注册 /api/questions 蓝图（评论+笔记+考点）。
func RegisterQuestionInteractionRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.QuestionInteractionService) {
	h := &questionInteractionHandler{svc: svc}

	g := rg.Group("/questions", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	g.GET("/:id/comments", h.ListComments)
	g.POST("/:id/comments", h.CreateComment)
	g.DELETE("/comments/:comment_id", h.DeleteComment)
	g.GET("/:id/note", h.GetNote)
	g.PUT("/:id/note", h.SaveNote)
	g.DELETE("/:id/note", h.DeleteNote)
	g.GET("/:id/knowledge", h.GetKnowledge)
}

// listCommentsReq 评论列表请求。
type listCommentsReq struct {
	QuestionID    int
	CurrentUserID int
}

// ListComments 获取题目评论列表 GET /api/questions/:id/comments
func (h *questionInteractionHandler) ListComments(c *gin.Context) {
	Endpoint[listCommentsReq, []service.QuestionCommentDTO]{
		Parse: func(c *gin.Context) (*listCommentsReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			return &listCommentsReq{QuestionID: questionID, CurrentUserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *listCommentsReq) (*[]service.QuestionCommentDTO, error) {
			result, err := h.svc.ListComments(req.QuestionID, req.CurrentUserID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *listCommentsReq, resp *[]service.QuestionCommentDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// createCommentReq 发表评论请求。
type createCommentReq struct {
	QuestionID int
	UserID     int
	Content    string
}

// CreateComment 发表评论 POST /api/questions/:id/comments
func (h *questionInteractionHandler) CreateComment(c *gin.Context) {
	Endpoint[createCommentReq, service.QuestionCommentDTO]{
		Parse: func(c *gin.Context) (*createCommentReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var req struct {
				Content string `json:"content"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &createCommentReq{
				QuestionID: questionID,
				UserID:     userID,
				Content:    req.Content,
			}, nil
		},
		Invoke: func(ctx context.Context, req *createCommentReq) (*service.QuestionCommentDTO, error) {
			result, err := h.svc.CreateComment(req.QuestionID, req.UserID, req.Content)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
		Render: func(c *gin.Context, _ *createCommentReq, resp *service.QuestionCommentDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "评论成功", resp)
		},
	}.Handle(c)
}

// deleteCommentReq 删除评论请求。
type deleteCommentReq struct {
	CommentID int64
	UserID    int
}

// DeleteComment 删除评论 DELETE /api/questions/comments/:comment_id
func (h *questionInteractionHandler) DeleteComment(c *gin.Context) {
	Endpoint[deleteCommentReq, struct{}]{
		Parse: func(c *gin.Context) (*deleteCommentReq, error) {
			commentID, err := pathInt64(c, "comment_id", "评论ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			return &deleteCommentReq{CommentID: commentID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *deleteCommentReq) (*struct{}, error) {
			if err := h.svc.DeleteComment(int(req.CommentID), req.UserID); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *deleteCommentReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, nil)
		},
	}.Handle(c)
}

// getNoteReq 获取笔记请求。
type getNoteReq struct {
	QuestionID int
	UserID     int
}

// GetNote 获取个人笔记 GET /api/questions/:id/note
func (h *questionInteractionHandler) GetNote(c *gin.Context) {
	Endpoint[getNoteReq, service.QuestionNoteDTO]{
		Parse: func(c *gin.Context) (*getNoteReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			return &getNoteReq{QuestionID: questionID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *getNoteReq) (*service.QuestionNoteDTO, error) {
			result, err := h.svc.GetNote(req.QuestionID, req.UserID)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
		Render: func(c *gin.Context, _ *getNoteReq, resp *service.QuestionNoteDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// saveNoteReq 保存笔记请求。
type saveNoteReq struct {
	QuestionID int
	UserID     int
	Content    string
}

// SaveNote 保存/更新笔记 PUT /api/questions/:id/note
func (h *questionInteractionHandler) SaveNote(c *gin.Context) {
	Endpoint[saveNoteReq, service.QuestionNoteDTO]{
		Parse: func(c *gin.Context) (*saveNoteReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var req struct {
				Content string `json:"content"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &saveNoteReq{
				QuestionID: questionID,
				UserID:     userID,
				Content:    req.Content,
			}, nil
		},
		Invoke: func(ctx context.Context, req *saveNoteReq) (*service.QuestionNoteDTO, error) {
			result, err := h.svc.SaveNote(req.QuestionID, req.UserID, req.Content)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
		Render: func(c *gin.Context, _ *saveNoteReq, resp *service.QuestionNoteDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// deleteNoteReq 删除笔记请求。
type deleteNoteReq struct {
	QuestionID int
	UserID     int
}

// DeleteNote 删除笔记 DELETE /api/questions/:id/note
func (h *questionInteractionHandler) DeleteNote(c *gin.Context) {
	Endpoint[deleteNoteReq, struct{}]{
		Parse: func(c *gin.Context) (*deleteNoteReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			return &deleteNoteReq{QuestionID: questionID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *deleteNoteReq) (*struct{}, error) {
			if err := h.svc.DeleteNote(req.QuestionID, req.UserID); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *deleteNoteReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, nil)
		},
	}.Handle(c)
}

// getKnowledgeReq 获取考点标签请求。
type getKnowledgeReq struct {
	QuestionID int
}

// GetKnowledge 获取题目考点标签 GET /api/questions/:id/knowledge
func (h *questionInteractionHandler) GetKnowledge(c *gin.Context) {
	Endpoint[getKnowledgeReq, []service.KnowledgeTagDTO]{
		Parse: func(c *gin.Context) (*getKnowledgeReq, error) {
			questionID, err := pathInt(c, "id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			return &getKnowledgeReq{QuestionID: questionID}, nil
		},
		Invoke: func(ctx context.Context, req *getKnowledgeReq) (*[]service.KnowledgeTagDTO, error) {
			result, err := h.svc.GetKnowledgeTags(req.QuestionID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *getKnowledgeReq, resp *[]service.KnowledgeTagDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			if resp == nil {
				response.Success(c, []service.KnowledgeTagDTO{})
			} else {
				response.Success(c, *resp)
			}
		},
	}.Handle(c)
}
