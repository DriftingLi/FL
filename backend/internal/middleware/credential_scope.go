package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
)

// CtxCredentialID 本次请求生效的「当前证件」（ADR-0047 §4）。
// 只有经 CredentialScoped 的端点在上下文里能看到它。
const CtxCredentialID ContextKey = "credential_id"

// CredentialResolver 解析某个用户的当前证件（服务端事实源：hrwai_users.current_credential_id）。
// 实现由 service 提供（TrainingCatalogService），middleware 只依赖这个窄 interface。
type CredentialResolver interface {
	// CurrentCredentialID 返回 (证件 ID, 是否存在)。用户不存在或未选证件时 ok=false。
	CurrentCredentialID(userID int) (int, bool)
}

// CredentialScoped 证件作用域守卫（ADR-0047 §4）：把「本次请求按哪个证件过滤」收敛成一个事实源。
//
// 解析序（显式优先，服务端兜底）：
//
//  1. 显式 query credential_id > 0 → 用它（保留「显式浏览其他证件」的既有用法）；
//  2. 否则用请求上下文里的登录用户当前证件（JWTAuth 已放行时才有）；
//  3. 都没有 → 不设置上下文值，端点按「不分区」处理（匿名访问公开面时的既有行为）。
//
// 客户端因此不再需要维护「哪些端点要注入证件」的豁免表——漏传不再静默返回全量。
func CredentialScoped(r CredentialResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := explicitCredentialID(c); ok {
			c.Set(string(CtxCredentialID), id)
			c.Next()
			return
		}
		// 只在学员角色上解析：current_credential_id 是 hrwai_users 的列，而 admin / tutor /
		// recruiter 的 JWT sub 来自各自的表——按 sub 直查 hrwai_users 会命中**同号的陌生素员行**，
		// 让控制台请求被静默按别人的证件过滤。改造前客户端也从不给这三端注入证件（豁免清单），
		// 故这里保持「非学员 = 不分区」的原语义。
		if r != nil && roleOf(c) == string(authz.RoleStudent) {
			if userID := CurrentUserID(c); userID > 0 {
				if id, ok := r.CurrentCredentialID(userID); ok {
					c.Set(string(CtxCredentialID), id)
				}
			}
		}
		c.Next()
	}
}

// explicitCredentialID 读显式 query 参数（>0 才算声明）。
func explicitCredentialID(c *gin.Context) (int, bool) {
	raw := c.Query("credential_id")
	if raw == "" {
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// CredentialIDPtr 读取本次请求生效的证件指针（未设置 = nil）。**指针形态是默认选择**：
// 服务层的 credentialID *int 形参用 nil 表达「未选证件」，指针能原样透传；
// 该 nil 读作「看全部」还是「只取未选定那一桶」由**调用的具名谓词**决定（ADR-0056 §2：
// RecordPartitionOf / PartitionBucket / EntityOwnedBy 三族，后者的 nil 语义相反）。
// 语义与既有的 queryIDPtr 一致：
// 「未声明」与「声明为 0」都表示不分区）。
func CredentialIDPtr(c *gin.Context) *int {
	v, ok := c.Get(string(CtxCredentialID))
	if !ok {
		return nil
	}
	id, ok := v.(int)
	if !ok || id <= 0 {
		return nil
	}
	return &id
}

// CredentialIDValue 读取本次请求生效的证件（未设置 = 0）。**仅在调用方以 0 表达「不分区」时使用**
// （真题卷列表的 CredentialID int 字段），其余一律用 CredentialIDPtr。
func CredentialIDValue(c *gin.Context) int {
	if p := CredentialIDPtr(c); p != nil {
		return *p
	}
	return 0
}

// roleOf 读请求上下文里的角色（JWTAuth 未应用时为空串）。
func roleOf(c *gin.Context) string {
	v, ok := c.Get(string(CtxUserRole))
	if !ok {
		return ""
	}
	role, _ := v.(string)
	return role
}
