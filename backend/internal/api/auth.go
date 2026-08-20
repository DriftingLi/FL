// Package api 实现 HTTP handlers。
package api

import (
	"context"
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

// Login 学员登录 POST /api/auth/login
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

// loginReq 登录请求体（三种角色共用字段）。
type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Logout 登出 POST /api/auth/logout
// 双令牌会话（ADR-0012）：撤销请求体携带的 refresh token（写黑名单至其自然过期）并清除登录
// Cookie；access 短生命周期自然过期，不入黑名单。本 handler 不依赖 JWTAuth——access 过期时
// 亦能撤销 refresh，保证登出语义可靠。
// 无 service 调用、纯会话操作，保留为薄适配（ADR-0009 一次性适配例外）。
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

// Refresh 刷新双令牌 POST /api/auth/refresh
// 请求体带 refresh token：校验类型/黑名单/有效期 → 签发新 access + 新 refresh（轮换），
// 旧 refresh 立即入黑名单防重放；失败统一返回 401（不区分原因，防枚举）。
// 本端点不经过 JWTAuth（用 refresh token 自身鉴权）。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		response.Unauthorized(c, "登录已过期，请重新登录")
		return
	}
	rt := req.RefreshToken
	claims, err := h.session.ValidateRefresh(rt)
	if err != nil {
		response.Unauthorized(c, "登录已过期，请重新登录")
		return
	}
	if revoked, _ := h.session.IsRevoked(c.Request.Context(), rt); revoked {
		response.Unauthorized(c, "登录已过期，请重新登录")
		return
	}
	access, refresh, err := h.session.IssuePair(claims.UserID, claims.Account, claims.Role)
	if err != nil {
		response.ServerError(c, "服务器内部错误")
		return
	}
	// 轮换：旧 refresh 立即入黑名单（防重放）
	_ = h.session.RevokeRefresh(c.Request.Context(), rt)
	response.Success(c, map[string]string{"token": access, "refresh_token": refresh})
}

// meReq /auth/me 请求（身份来自 JWT 中间件上下文）。
type meReq struct {
	UserID  int
	Role    string
	Account string
}

// Me 获取当前用户 GET /api/auth/me
// 资料组装收编在 AuthService.GetProfile（响应形状由契约测试锁定）。
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

// UpdateProfile 提交个人资料（昵称）修改审核 POST /api/auth/profile
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
			return &updateProfileReq{UID: uid, Nickname: req.Nickname}, nil
		},
		Invoke: func(ctx context.Context, req *updateProfileReq) (*service.ProfileChangeRequestDTO, error) {
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
			response.SuccessWithMsg(c, "昵称修改已提交，审核通过后生效", resp)
		},
	}.Handle(c)
}

// updateProfileReq 更新个人资料请求。
type updateProfileReq struct {
	UID      int
	Nickname string
}

// UploadAvatar 上传头像并提交审核 POST /api/auth/avatar（multipart，图片自动压缩为 WebP 后存入 local/R2）
// multipart + 多步文件处理，保留为薄适配（ADR-0009 一次性适配例外）。
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
