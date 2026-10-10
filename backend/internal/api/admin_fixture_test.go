// 管理端测试夹具（#1618 段1）。
package api

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// adminTokenWithAccount 造一个**真实存在**的管理员账号（挂受保护角色）并签发 admin 令牌。
//
// 为什么测试必须真的建账号：管理端能力自 #1618 段1 起由数据层回答（按 admin_id 查所挂角色 →
// 能力集），静态表不再回答 admin。只签一个 role="admin" 的令牌而库里没有该账号，**正确结果就是
// 403** —— 测试若还按「role 即能力」的老口径写，等于在断言一个已被推翻的语义。
func adminTokenWithAccount(t *testing.T, cfg *config.Config, db *gorm.DB, username string) string {
	t.Helper()
	admin := testutil.SeedAdmin(t, db, username, "x")
	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}
	return tok
}
