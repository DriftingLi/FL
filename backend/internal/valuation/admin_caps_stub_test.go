// 管理端能力解析的测试替身（#1618 段1）。
package valuation

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
)

// stubAdminCapabilities 直接回答「受保护角色的能力全集」。
//
// 为什么估值域用替身而不是真实解析器：本域的测试用**内存 adapter** 装配（没有 gorm DB），
// 而解析器需要查库。解析实现本身（查库 / 40s 缓存 / 「未授权」与「查询故障」的分档）由
// internal/admincap 与 internal/middleware 的测试覆盖，这里只负责让管理端路由在测试里可达。
type stubAdminCapabilities struct{}

func (stubAdminCapabilities) AdminCapabilities(int) (map[authz.Capability]struct{}, bool, error) {
	caps := make(map[authz.Capability]struct{}, len(authz.ProtectedAdminCapabilities()))
	for _, c := range authz.ProtectedAdminCapabilities() {
		caps[c] = struct{}{}
	}
	return caps, true, nil
}

// attachAdminCapabilities 给自建引擎挂上解析替身（与装配根 NewRouter 的挂法同形）。
func attachAdminCapabilities(r *gin.Engine) {
	r.Use(middleware.AdminCapabilityResolver(stubAdminCapabilities{}))
}
