// 本文件：/api/auth 公开面（登录/刷新/登出/me/资料/注销）的 handler 与注册入口。
package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/core"
	"forklift-training/internal/filestore"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 认证相关 handler。
type handler struct {
	authSvc   *Service
	fileSvc   *filestore.FileStore
	storage   storage.Storage
	reviewSvc *ProfileReviewService
	session   *security.Session
}

// newHandler 创建认证 handler。session 由装配根构建一次注入。
func newHandler(sess *security.Session, authSvc *Service, fileSvc *filestore.FileStore, st storage.Storage, reviewSvc *ProfileReviewService, logger *zap.Logger) *handler {
	return &handler{
		authSvc: authSvc, fileSvc: fileSvc, storage: st, reviewSvc: reviewSvc,
		session: sess,
	}
}

// RegisterRoutes 注册 /api/auth 公开面（10 条）——原 api/router.go 的内联 auth 组。
// 形态纪律（ADR-0070）：handler 类型包私有、注册函数内构造、只收自己需要的依赖、不 import internal/api。
//
// session 在此同步给 Service：装配根注入的会话实例必须与服务内的那一份是同一对象，
// 否则「注销先写吊销标记 / 旧 refresh 被拒」这类判据会静默失效（测试里 d.Session 会被换成带黑名单的会话）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service,
	fileSvc *filestore.FileStore, st storage.Storage, reviewSvc *ProfileReviewService,
	logger *zap.Logger) {
	if svc != nil {
		svc.session = session
	}
	h := newHandler(session, svc, fileSvc, st, reviewSvc, logger)

	g := rg.Group("/auth")
	{
		g.POST("/login", h.Login)
		g.POST("/admin-login", h.AdminLogin)
		g.POST("/tutor-login", h.TutorLogin)
		g.POST("/recruiter-login", h.RecruiterLogin)
		// 双令牌会话（ADR-0012）：/refresh 用 refresh token 自身鉴权（不经过 JWTAuth）；
		// /logout 撤销 refresh token（Cookie 优先、请求体兜底，与 /refresh 同口径），不依赖 JWTAuth（access 过期时也能撤销 refresh / 登出）。
		g.POST("/refresh", h.Refresh)
		g.POST("/logout", h.Logout)
		g.GET("/me", middleware.JWTAuth(session), h.Me)
		// 个人资料：昵称 / 头像 / 单位 / 注销
		g.PUT("/profile", middleware.JWTAuth(session), h.UpdateProfile)
		g.POST("/avatar", middleware.JWTAuth(session), h.UploadAvatar)
		g.DELETE("/account", middleware.JWTAuth(session), h.DeleteAccount)
	}
}

// Login 学员登录
// @Summary 学员登录
// @Description 账号密码登录（hrwai_user），成功写入登录 Cookie 并返回双令牌
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "登录" example({"username":"13800000001","password":"123456"})
// @Success 200 {object} response.R{data=LoginResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/login [post]
func (h *handler) Login(c *gin.Context) {
	httpx.Endpoint[loginReq, LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := httpx.BindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, httpx.BadRequest("账号和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*LoginResult, error) {
			return h.authSvc.HrwaiLogin(req.Username, req.Password)
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *loginReq, resp *LoginResult) {
			h.session.SetLoginCookies(c.Writer, resp.Token, resp.RefreshToken)
			response.SuccessWithMsg(c, "登录成功", resp)
		},
	}.Handle(c)
}

// @Summary 管理员登录
// @Description 管理员账号密码登录，成功写入登录 Cookie 并返回双令牌
// @Tags 管理端-认证
// @Accept json
// @Produce json
// @Param body body object true "登录" example({"username":"admin","password":"123456"})
// @Success 200 {object} response.R{data=LoginResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/admin-login [post]
// AdminLogin 管理员登录 POST /api/auth/admin-login
func (h *handler) AdminLogin(c *gin.Context) {
	httpx.Endpoint[loginReq, LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := httpx.BindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, httpx.BadRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*LoginResult, error) {
			return h.authSvc.AdminLogin(req.Username, req.Password)
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *loginReq, resp *LoginResult) {
			h.session.SetLoginCookies(c.Writer, resp.Token, resp.RefreshToken)
			response.SuccessWithMsg(c, "管理员登录成功", resp)
		},
	}.Handle(c)
}

// @Summary 导师登录
// @Description 导师账号密码登录，成功写入登录 Cookie 并返回双令牌
// @Tags 讲师端-认证
// @Accept json
// @Produce json
// @Param body body object true "登录" example({"username":"tutor","password":"123456"})
// @Success 200 {object} response.R{data=LoginResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/tutor-login [post]
// TutorLogin 导师登录 POST /api/auth/tutor-login
func (h *handler) TutorLogin(c *gin.Context) {
	httpx.Endpoint[loginReq, LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := httpx.BindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, httpx.BadRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*LoginResult, error) {
			return h.authSvc.TutorLogin(req.Username, req.Password)
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *loginReq, resp *LoginResult) {
			h.session.SetLoginCookies(c.Writer, resp.Token, resp.RefreshToken)
			response.SuccessWithMsg(c, "讲师登录成功", resp)
		},
	}.Handle(c)
}

// @Summary 企业招聘者登录
// @Description 招聘者账号密码登录（第四角色，host-only cookie 隔离），成功返回双令牌
// @Tags 招聘域-认证
// @Accept json
// @Produce json
// @Param body body object true "登录" example({"username":"hr001","password":"123456"})
// @Success 200 {object} response.R{data=LoginResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/recruiter-login [post]
// RecruiterLogin 企业招聘者登录 POST /api/auth/recruiter-login（第四角色，host-only cookie 隔离）
func (h *handler) RecruiterLogin(c *gin.Context) {
	httpx.Endpoint[loginReq, LoginResult]{
		Parse: func(c *gin.Context) (*loginReq, error) {
			req, err := httpx.BindJSON[loginReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" {
				return nil, httpx.BadRequest("用户名和密码不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *loginReq) (*LoginResult, error) {
			return h.authSvc.RecruiterLogin(req.Username, req.Password)
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *loginReq, resp *LoginResult) {
			h.session.SetRecruiterLoginCookies(c.Writer, resp.Token, resp.RefreshToken)
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
// @Description 撤销 refresh_token（Cookie 优先，回退请求体）并清除登录 Cookie；不依赖 JWTAuth，access 过期亦可登出
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object false "refresh_token" example({"refresh_token":"eyJhbGciOi..."})
// @Success 200 {object} response.R "success"
// @Router /auth/logout [post]
func (h *handler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req) // refresh_token 缺失或解析失败时只清本地/Cookie，静默放行
	// ADR-0067 的读取顺序（Cookie 优先）在登出入口同样成立：手上那支凭证来自哪里是入口差异，
	// 不是第三种语义（Session.SignOut 的既有口径）。
	// ADR-0067 决策 2 的落点（2026-09-29 修订）：refresh cookie 的 Path = /api/auth，覆盖本端点，
	// 所以浏览器登出**能**取到凭证、单会话终止（CONTEXT.md「会话」）照旧成立。
	// 取的是**哪一族**由 access 定（RefreshCookieForRequest）：本端点与 /refresh 面对的是同一个
	// 并存问题，任选一族就是把别人的会话终止掉。
	// body 那一路继续服务显式回传凭证的非浏览器客户端（移动端拿不到 Cookie）。
	refresh := h.session.RefreshCookieForRequest(c.Request)
	if refresh == "" {
		refresh = req.RefreshToken
	}
	_ = h.session.SignOut(c.Request.Context(), c.Writer, refresh)
	response.SuccessWithMsg(c, "已登出", nil)
}

// Refresh 刷新双令牌
// @Summary 刷新双令牌
// @Description 轮换签发新 access/refresh，旧 refresh 入黑名单；凭证读取顺序为 httpOnly Cookie（ADR-0067）→ 请求体（移动端兼容），失败统一 401
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object false "refresh_token（Cookie 通道存在时被忽略；无 Cookie 的客户端才用）" example({"refresh_token":"eyJhbGciOi..."})
// @Success 200 {object} response.R{data=RefreshResultDTO} "success（响应体恒含新 access + 新 refresh，请求体通道客户端需要）"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/refresh [post]
func (h *handler) Refresh(c *gin.Context) {
	// ADR-0067 决策 1（#1376 跨端评审后收窄）：Cookie 通道存在即以 Cookie 为准，请求体不再参与
	// 判定（连读都不读，否则「两个通道各带一支」时谁赢就成了实现细节）。
	// 而「Cookie 通道存在」的前提是**读得出族**：族由 access 定（Session.RefreshCookieForRequest），
	// 没有族线索时一枚 Cookie 都不看。原因——通道归属由客户端容器行为决定，不由代码决定，
	// 而 #1389 的真机读数把方向判成了：**App 容器不维持 cookie jar**（那句「App/H5 自动带 cookie」
	// 出自 uni-app vue 版参数表，uni-app x 的表里没有），所以并存问题实际落在**浏览器**这一面
	// （父域学员那枚 + host-only 招聘者那枚在 recruit. 上同时被投递；H5 面未测）。
	// ⇒ 正确性靠这里的定族，不靠「body 分支一定被走到」，也不靠「App 一定不带 cookie」。
	// 请求体通道保留给移动端与非浏览器客户端——它们拿不到 Cookie，砍掉就是跨端断供。
	refreshToken := h.session.RefreshCookieForRequest(c.Request)
	if refreshToken == "" {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
			response.Unauthorized(c, "登录已过期，请重新登录")
			return
		}
		refreshToken = req.RefreshToken
	}
	// 原子轮换（ADR-0016）：校验/抢占/吊销/签发收敛在会话模块单点，并发双刷恰一成功。
	// 轮换语义不变（票 #1363 判据 2），只是浏览器侧多回写一枚新 Cookie（ADR-0067）。
	access, refresh, err := h.session.RotateAndSetCookie(c.Request.Context(), c.Writer, refreshToken)
	if err != nil {
		if errors.Is(err, security.ErrInvalidRefresh) {
			response.Unauthorized(c, "登录已过期，请重新登录")
			return
		}
		response.ServerError(c, "服务器内部错误")
		return
	}
	response.Success(c, RefreshResultDTO{RefreshToken: refresh, Token: access})
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
// @Success 200 {object} response.R{data=ProfileDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/me [get]
func (h *handler) Me(c *gin.Context) {
	httpx.Endpoint[meReq, ProfileDTO]{
		Parse: func(c *gin.Context) (*meReq, error) {
			return &meReq{
				UserID:  middleware.CurrentUserID(c),
				Role:    middleware.CurrentRole(c),
				Account: middleware.CurrentAccount(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *meReq) (*ProfileDTO, error) {
			return h.authSvc.GetProfile(req.UserID, req.Role, req.Account), nil
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
// @Success 200 {object} response.R{data=ProfileChangeRequestDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/profile [put]
func (h *handler) UpdateProfile(c *gin.Context) {
	httpx.Endpoint[updateProfileReq, ProfileChangeRequestDTO]{
		Parse: func(c *gin.Context) (*updateProfileReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid <= 0 {
				return nil, &httpx.ParseError{Status: 401, Message: "请先登录"}
			}
			req, err := httpx.BindJSON[updateProfileReq](c)
			if err != nil {
				return nil, err
			}
			if req.Nickname == "" && req.Company == nil {
				return nil, httpx.BadRequest("参数错误")
			}
			return &updateProfileReq{UID: uid, Nickname: req.Nickname, Company: req.Company}, nil
		},
		Invoke: func(ctx context.Context, req *updateProfileReq) (*ProfileChangeRequestDTO, error) {
			// 单位立即生效
			if req.Company != nil {
				if err := h.authSvc.UpdateCompany(req.UID, *req.Company); err != nil {
					return nil, err
				}
				// 若同时带昵称，继续走审核流
				if req.Nickname == "" {
					return &ProfileChangeRequestDTO{}, nil
				}
			}
			return h.reviewSvc.CreateRequest(req.UID, model.ProfileFieldNickname, req.Nickname)
		},
		ErrStatus: &httpx.ErrStatusTable{Fallback: http.StatusBadRequest},
		Render: func(c *gin.Context, _ *updateProfileReq, resp *ProfileChangeRequestDTO) {
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
func (h *handler) DeleteAccount(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	// 全会话吊销在先（ADR-0060 票2）：标记写失败即整体不生效。与改密/禁用的尽力而为
	// 策略有意不同——那两处有已生效且不可回退的动作，注销没有；先删后吊销会留下
	// 「资料已删、凭证仍活」（RotateRefresh 不查用户存在，旧 refresh 最长 7 天仍可签发 access）。
	if err := h.session.RevokeIdentity(c.Request.Context(), core.HrwaiRole, uid); err != nil {
		response.BadRequest(c, "注销失败：会话吊销未生效，请稍后重试")
		return
	}
	if err := h.authSvc.DeleteAccount(uid); err != nil {
		// 「用户不存在」是业务态，可以照原话回；清理或自证失败是内部故障——它们的包装里带
		// 表名与驱动原文，不外发（#1356 要的是「明确失败」，不是把库的结构贴给调用方）。
		if errors.Is(err, model.ErrHrwaiUserNotFound) {
			response.BadRequest(c, err.Error())
			return
		}
		response.BadRequest(c, "注销失败：数据清理未生效，请稍后重试")
		return
	}
	h.session.ClearLoginCookies(c.Writer)
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
// @Success 200 {object} response.R{data=ProfileChangeRequestDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /auth/avatar [post]
func (h *handler) UploadAvatar(c *gin.Context) {
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
	content, err := filestore.ReadMultipartFile(file)
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}

	url, err := h.fileSvc.Save(content, file.Filename, filestore.AvatarImageDirPrefix)
	if err != nil {
		response.ServerErrorCause(c, "头像保存失败: ", err)
		return
	}
	reqDTO, err := h.reviewSvc.CreateRequest(uid, model.ProfileFieldAvatar, url)
	if err != nil {
		// 提交审核失败时清理已上传的文件（尽力而为）
		_ = h.storage.Delete(c.Request.Context(), url)
		response.BadRequest(c, "头像提交失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "头像修改已提交，审核通过后生效", reqDTO)
}
