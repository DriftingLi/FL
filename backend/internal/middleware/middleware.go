// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复。
package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"forklift-training/internal/authz"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// ContextKey 是 context 中存储用户信息的键。
type ContextKey string

const (
	// CtxUserID 用户ID
	CtxUserID ContextKey = "user_id"
	// CtxAccount 登录账号
	CtxAccount ContextKey = "account"
	// CtxUserRole 用户角色
	CtxUserRole ContextKey = "role"
	// CtxRequestID 请求ID
	CtxRequestID ContextKey = "request_id"
	// CtxCapabilityResolver 动态角色能力解析源（#1618 段1）
	CtxCapabilityResolver ContextKey = "capability_resolver"
)

// CapabilityResolver 动态角色（admin）有效能力集的解析源（#1618 段1）。
//
// 为什么是接口而不是直接调用：middleware 是底层包 —— 域包（internal/admin）反过来 import 它，
// 故这里只能声明接口，实现由装配根注入（admin.Service 结构化满足它）。
type CapabilityResolver interface {
	// AdminCapabilities 返回该管理员的有效能力集。
	//
	// 第二个返回值 = 是否已挂角色：false 表示未授权或账号不存在（**这是判定结果**，不是故障）。
	// 第三个返回值 = 查询故障：非 nil 时守卫按 **500** 处理 —— 把「库挂了」渲染成
	// 403「权限不足」会把故障伪装成权限问题（判据见 contract 测试
	// TestListEndpointDBFailureRenders500Envelope：DB 故障必须 500）。
	AdminCapabilities(adminID int) (map[authz.Capability]struct{}, bool, error)
}

// AdminCapabilityResolver 把解析源注入 context（装配根挂在 /api 组上）。
//
// 它**不读 claims**，因此与各域组内 JWTAuth 的先后无关：根组的中间件必然先跑。
// r 为 nil 时不注入 —— 动态角色的能力守卫随之 fail closed（宁可 403，不可放行）。
func AdminCapabilityResolver(r CapabilityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r != nil {
			c.Set(string(CtxCapabilityResolver), r)
		}
		c.Next()
	}
}

// capabilityResolverFrom 读出请求上下文里的解析源。
func capabilityResolverFrom(c *gin.Context) (CapabilityResolver, bool) {
	v, ok := c.Get(string(CtxCapabilityResolver))
	if !ok {
		return nil, false
	}
	r, ok := v.(CapabilityResolver)
	return r, ok
}

// Claims JWT 声明（统一由 security 会话模块持有）。
type Claims = security.Claims

// RequestID 为每个请求注入唯一 ID，**始终由服务端铸造**（ADR-0062 票1）。
// 调用方供给的 X-Request-ID 不被采纳：同一个 ID 既进 AI 计量的幂等键、又进访问与审计日志，
// 采信外部值等于把「这次消费发生过没有」交给被计量方决定（固定一个头值即成免扣费通道）。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := uuid.NewString()
		c.Set(string(CtxRequestID), rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// CORS 跨域中间件。
// 开发环境放开全部来源（本地前端可能运行在任意端口/子域名，避免改端口后被拦截）；
// 生产环境仍按配置白名单校验。
// 在闭包外构造一次 cors.Handler，避免每个请求重复创建（行为保持一致）。
func CORS(origins []string, isProd bool) gin.HandlerFunc {
	config := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Silent", "Accept"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if isProd {
		config.AllowOrigins = origins
	} else {
		config.AllowAllOrigins = true
	}
	handler := cors.New(config)
	return func(c *gin.Context) {
		handler(c)
	}
}

// Recovery panic 恢复中间件。
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		response.ServerError(c, "服务器内部错误")
		c.Abort()
	})
}

// HealthPaths 健康检查探活路径：不出现在访问日志中（避免探活刷屏），
// 也不受限流拦截（容器编排探活不应被限流挡掉）。
var HealthPaths = map[string]struct{}{
	"/api/health":      {},
	"/api/health/live": {},
}

// JWTAuth 强制 JWT 认证中间件。
// sess 由装配根构建一次注入，避免每处路由注册重复构造会话模块。
func JWTAuth(sess *security.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveClaims(c, sess) {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuth 可选 JWT 认证：有 token 则解析填充，无则放行。
func OptionalAuth(sess *security.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		resolveClaims(c, sess)
		c.Next()
	}
}

// resolveClaims 提取 token → 校验（仅 access 类型）→ 写 context，是
// JWTAuth 与 OptionalAuth 共享的解析核心。
// 返回是否通过认证：JWTAuth 据此 401 中止，OptionalAuth 忽略结果静默放行。
// 双令牌会话（ADR-0012）：access 短生命周期不入黑名单，refresh 传入鉴权端点被 VerifyAccess 拒绝；
// 登出撤销的是 refresh（由 /logout 处理器处理），access 自然过期。
func resolveClaims(c *gin.Context, sess *security.Session) bool {
	tokenStr := sess.ExtractToken(c.GetHeader("Authorization"), authCookieValue(c, sess))
	if tokenStr == "" {
		return false
	}
	if claims, err := sess.VerifyAccess(tokenStr); err == nil {
		c.Set(string(CtxUserID), claims.UserID)
		c.Set(string(CtxAccount), claims.Account)
		c.Set(string(CtxUserRole), claims.Role)
		return true
	}
	return false
}

// authCookieValue 读取登录 Cookie（依次尝试 hrwai_token 与 recruiter_token，不存在时返回空串）。
// 遍历顺序的实现已上收到 Session.AccessCookieValue —— refresh 的族判定读的是同一个函数：
// 「鉴权面认的活跃身份」与「续期面选择的令牌族」必须是同一个答案（#1376 跨端评审的串族缺陷）。
func authCookieValue(c *gin.Context, sess *security.Session) string {
	return sess.AccessCookieValue(c.Request)
}

// CapabilityRequired 能力守卫（ADR-0047 §1）：判据是 authz 能力，不是角色字面量。
// 这是逐域迁移的目标形态——端点声明「需要什么能力」，角色可达面由 authz 能力表回答。
// 必须在 JWTAuth 之后使用。
//
// **多参 = 任一命中即放行**（#1639）：一片只读面若被两个侧栏叶子共用（生成页要选课程、
// 巡检视图要读积分流水与举报队列），端点无法只声明其中一个——声明成一个就等于给另一个
// 叶子的持有者关上门。单参调用的语义与收敛前逐字一致。
func CapabilityRequired(caps ...authz.Capability) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(string(CtxUserRole)); !exists {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		ok, err := requireCapability(c, caps...)
		if err != nil {
			// 判定所需的查询失败：渲染 500 而非 403 —— 故障不得伪装成权限问题。
			response.ServerError(c, "服务器内部错误")
			c.Abort()
			return
		}
		if !ok {
			response.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}

// HasCapability 读当前请求的能力判定（JWTAuth 未应用或未登录时一律 false）。
// 与 CapabilityRequired 同源，供 handler 在**同一端点内分流**读路径时使用
// （如题目 by-id：作者/审核者走编辑面，学员走题库池口径）。
//
// 两条路径（#1618 段1）：
//   - 静态角色 → authz 能力表；
//   - **动态角色（admin）** → 装配根注入的解析源（admin.Service.AdminCapabilities）：
//     每请求查库 + 短缓存，故超管的授权变更最多延迟一个 TTL 生效。
//
// 解析源未装配或管理员未挂角色时一律 false —— 能力守卫宁可 403，不可放行。
func HasCapability(c *gin.Context, capability authz.Capability) bool {
	ok, err := requireCapability(c, capability)
	// handler 的分流判定没有「故障」这一档：查询失败按无能力处理（fail closed，退到保守分支）。
	return err == nil && ok
}

// requireCapability 与 HasCapability 同源，但把**查询故障**与**判定结果**分开带出：
// 守卫（CapabilityRequired）据此把故障渲染成 500，而把「无能力」渲染成 403。
//
// 多参即**任一命中**（#1639）；动态角色无论几个候选只解析一次能力集 —— 逐个候选各查一次库
// 会把「一次判定」变成 N 次查询，而候选越多只会发生在共享只读面上。
func requireCapability(c *gin.Context, candidates ...authz.Capability) (bool, error) {
	role := authz.Role(CurrentRole(c))
	if !authz.IsDynamicRole(role) {
		for _, capability := range candidates {
			if authz.Has(role, capability) {
				return true, nil
			}
		}
		return false, nil
	}
	resolver, ok := capabilityResolverFrom(c)
	if !ok {
		return false, nil
	}
	caps, granted, err := resolver.AdminCapabilities(CurrentUserID(c))
	if err != nil {
		return false, err
	}
	if !granted {
		return false, nil
	}
	for _, capability := range candidates {
		if _, has := caps[capability]; has {
			return true, nil
		}
	}
	return false, nil
}

// CurrentUserID 从 gin.Context 读取当前登录用户 ID(未登录返回 0)。
// 统一供主体系与估值模块使用,替代原 vhandler.CurrentValuationUserID。
func CurrentUserID(c *gin.Context) int {
	v, ok := c.Get(string(CtxUserID))
	if !ok {
		return 0
	}
	uid, _ := v.(int)
	return uid
}

// CurrentAccount 从 gin.Context 读取当前登录账号(未登录返回空串)。
func CurrentAccount(c *gin.Context) string {
	v, ok := c.Get(string(CtxAccount))
	if !ok {
		return ""
	}
	uid, _ := v.(string)
	return uid
}

// CurrentRole 从 gin.Context 读取当前登录用户角色(未登录返回空串)。
func CurrentRole(c *gin.Context) string {
	v, ok := c.Get(string(CtxUserRole))
	if !ok {
		return ""
	}
	uid, _ := v.(string)
	return uid
}
