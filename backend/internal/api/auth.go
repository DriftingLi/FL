// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/pkg/response"
)

// AuthHandler 认证相关 handler。
type AuthHandler struct {
	authSvc   *service.AuthService
	fileSvc   *service.FileStore
	storage   storage.Storage
	reviewSvc *service.ProfileReviewService
	session   *security.Session
}

// NewAuthHandler 创建认证 handler。session 由装配根构建一次注入。
func NewAuthHandler(sess *security.Session, authSvc *service.AuthService, fileSvc *service.FileStore, st storage.Storage, reviewSvc *service.ProfileReviewService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc, fileSvc: fileSvc, storage: st, reviewSvc: reviewSvc,
		session: sess,
	}
}

// Login 学员登录
// @Summary 学员登录
// @Description 账号密码登录（hrwai_user），成功写入登录 Cookie 并返回双令牌
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "登录" example({"username":"13800000001","password":"123456"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	Endpoint[loginReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := bindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, badRequest("账号和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*service.LoginResult, error) {
			return h.authSvc.HrwaiLogin(req.Username, req.Password)
		},
		Render: func(c *gin.Context, _ *loginReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setAuthCookie(c, h.session, resp.Token)
			response.SuccessWithMsg(c, "登录成功", resp)
		},
	}.Handle(c)
}

// AdminLogin 管理员登录 POST /api/auth/admin-login
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	Endpoint[loginReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := bindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, badRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*service.LoginResult, error) {
			return h.authSvc.AdminLogin(req.Username, req.Password)
		},
		Render: func(c *gin.Context, _ *loginReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setAuthCookie(c, h.session, resp.Token)
			response.SuccessWithMsg(c, "管理员登录成功", resp)
		},
	}.Handle(c)
}

// TutorLogin 导师登录 POST /api/auth/tutor-login
func (h *AuthHandler) TutorLogin(c *gin.Context) {
	Endpoint[loginReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := bindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, badRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*service.LoginResult, error) {
			return h.authSvc.TutorLogin(req.Username, req.Password)
		},
		Render: func(c *gin.Context, _ *loginReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setAuthCookie(c, h.session, resp.Token)
			response.SuccessWithMsg(c, "导师登录成功", resp)
		},
	}.Handle(c)
}

// RecruiterLogin 企业招聘者登录 POST /api/auth/recruiter-login（第四角色，host-only cookie 隔离）
func (h *AuthHandler) RecruiterLogin(c *gin.Context) {
	Endpoint[loginReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := bindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, badRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*service.LoginResult, error) {
			return h.authSvc.RecruiterLogin(req.Username, req.Password)
		},
		Render: func(c *gin.Context, _ *loginReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setRecruiterCookie(c, h.session, resp.Token)
			response.SuccessWithMsg(c, "招聘者登录成功", resp)
		},
	}.Handle(c)
}

// loginReq 登录请求体（三种角色共用字段）。
type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Logout 登出
// @Summary 登出
// @Description 撤销 refresh_token 并清除登录 Cookie；不依赖 JWTAuth，access 过期亦可登出
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object false "refresh_token" example({"refresh_token":"eyJhbGciOi..."})
// @Success 200 {object} response.R "success"
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req) // refresh_token 缺失或解析失败时只清本地/Cookie，静默放行
	if req.RefreshToken != "" {
		_ = h.session.RevokeRefresh(c.Request.Context(), req.RefreshToken)
	}
	h.session.ClearCookie(c.Writer)
	response.SuccessWithMsg(c, "已登出", nil)
}

// Refresh 刷新双令牌
// @Summary 刷新双令牌
// @Description 轮换签发新 access/refresh，旧 refresh 入黑名单；失败统一 401
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "refresh_token" example({"refresh_token":"eyJhbGciOi..."})
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		response.Unauthorized(c, "登录已过期，请重新登录")
		return
	}
	// 原子轮换（ADR-0016）：校验/抢占/吊销/签发收敛在会话模块单点，并发双刷恰一成功
	access, refresh, err := h.session.RotateRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, security.ErrInvalidRefresh) {
			response.Unauthorized(c, "登录已过期，请重新登录")
			return
		}
		response.ServerError(c, "服务器内部错误")
		return
	}
	response.Success(c, map[string]string{"token": access, "refresh_token": refresh})
}

// meReq /auth/me 请求（身份来自 JWT 中间件上下文）。
type meReq struct {
	UserID  int
	Role    string
	Account string
}

// Me 获取当前用户
// @Summary 当前用户信息
// @Description 基于 JWT 获取当前用户档案（响应形状由契约测试锁定）
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	Endpoint[meReq, service.ProfileDTO]{
		Parse: func(c *gin.Context) (*meReq, error) {
			return &meReq{
				UserID:  middleware.CurrentUserID(c),
				Role:    middleware.CurrentRole(c),
				Account: middleware.CurrentAccount(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *meReq) (*service.ProfileDTO, error) {
			return h.authSvc.GetProfile(req.UserID, req.Role, req.Account), nil
		},
		Render: func(c *gin.Context, _ *meReq, resp *service.ProfileDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UpdateProfile 提交个人资料修改
// @Summary 提交昵称修改审核或直接更新单位
// @Description nickname 走资料审核；company 立即生效
// @Tags 学员端-个人资料
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "资料" example({"nickname":"新昵称","company":"新单位"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	Endpoint[updateProfileReq, service.ProfileChangeRequestDTO]{
		Parse: func(c *gin.Context) (*updateProfileReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid <= 0 {
				return nil, &ParseError{Status: 401, Message: "请先登录"}
			}
			req, err := bindJSON[updateProfileReq](c)
			if err != nil {
				return nil, err
			}
			if req.Nickname == "" && req.Company == nil {
				return nil, badRequest("参数错误")
			}
			return &updateProfileReq{UID: uid, Nickname: req.Nickname, Company: req.Company}, nil
		},
		Invoke: func(ctx context.Context, req *updateProfileReq) (*service.ProfileChangeRequestDTO, error) {
			// 单位立即生效
			if req.Company != nil {
				if err := h.authSvc.UpdateCompany(req.UID, *req.Company); err != nil {
					return nil, err
				}
				// 若同时带昵称，继续走审核流
				if req.Nickname == "" {
					return &service.ProfileChangeRequestDTO{}, nil
				}
			}
			return h.reviewSvc.CreateRequest(req.UID, service.ProfileFieldNickname, req.Nickname)
		},
		Render: func(c *gin.Context, _ *updateProfileReq, resp *service.ProfileChangeRequestDTO, err error) {
			if err != nil {
				var pe *ParseError
				if asParseError(err, &pe) {
					renderStatus(c, pe.Status, pe.Message)
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			if resp != nil && resp.ID == 0 {
				response.SuccessWithMsg(c, "单位更新成功", resp)
				return
			}
			response.SuccessWithMsg(c, "昵称修改已提交，审核通过后生效", resp)
		},
	}.Handle(c)
}

// updateProfileReq 更新个人资料请求。
type updateProfileReq struct {
	UID      int     `json:"uid"`
	Nickname string  `json:"nickname"`
	Company  *string `json:"company"`
}

// DeleteAccount 注销当前学员账号（硬删除）
// @Summary 注销帐号
// @Description 硬删除当前学员及关联数据，论坛内容匿名化
// @Tags 学员端-个人资料
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/account [delete]
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.authSvc.DeleteAccount(uid); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.session.ClearCookie(c.Writer)
	response.SuccessWithMsg(c, "帐号已注销", nil)
}

// UploadAvatar 上传头像
// @Summary 上传头像
// @Description multipart 上传，自动压缩为 WebP 后存入 storage 并提交审核
// @Tags 学员端-个人资料
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "头像图片"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/avatar [post]
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	userID, _ := c.Get(string(middleware.CtxUserID))
	uid, _ := userID.(int)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	if ok, msg := h.fileSvc.ValidateImage(file.Filename, file.Size); !ok {
		response.BadRequest(c, msg)
		return
	}
	src, err := file.Open()
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}

	url, err := h.fileSvc.Save(content, file.Filename, "avatars")
	if err != nil {
		response.ServerError(c, "头像保存失败: "+err.Error())
		return
	}
	reqDTO, err := h.reviewSvc.CreateRequest(uid, service.ProfileFieldAvatar, url)
	if err != nil {
		// 提交审核失败时清理已上传的文件（尽力而为）
		_ = h.storage.Delete(c.Request.Context(), url)
		response.BadRequest(c, "头像提交失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "头像修改已提交，审核通过后生效", reqDTO)
}
