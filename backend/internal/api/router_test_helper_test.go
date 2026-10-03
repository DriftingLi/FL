package api

import (
	"sync"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// seedMu 串行化本测试包对 testutil.SeedStudent 的调用（#1366）。
//
// testutil.SeedStudent 的 uid 取自一个进程级、非原子递增的计数器（testutil/db.go
// 的 seedUIDCounter++）。#1362 刚把 testutil 定形，本票不改它；而契约测试改 t.Parallel
// 后，并行用例并发播种会读-改-写同一个全局 ⇒ -race 报数据竞态。此处只在 api 测试包的
// 调用边界加锁把并发串行化，不复制 testutil 的任何构造逻辑（不构成第二份事实源，只是
// 把「并发触碰同一全局」收成「互斥访问」）。uid 因此仍逐次唯一，且本包无任何用例断言
// 某个绝对 uid 值（已核：无 1000000000000000000 量级的字面对账），加锁只影响并发时序、
// 不影响期望值。
var seedMu sync.Mutex

// seedStudent 与 testutil.SeedStudent 完全同一条链，只多把并发播种互斥。
func seedStudent(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.HrwaiUser {
	t.Helper()
	seedMu.Lock()
	defer seedMu.Unlock()
	return testutil.SeedStudent(t, db, username, hashedPassword)
}

// newContractDeps 构建契约测试用的完整装配根（storage 与导出 store 传 nil，被测蓝图不使用）。
//
// 「被测蓝图不使用 storage」这句从 #1361 起对投稿蓝图不再成立：Create 的第四校验要问存储侧
// 「那个暂存文件在不在」，nil storage 会让它在装配链里落 500。需要 storage 的用例走下面那条
// 同一条装配链的变体，不要另起一份构造（装配链分叉 = 测试装配与生产装配各测各的）。
func newContractDeps(t *testing.T, db *gorm.DB, cfg *config.Config) *Deps {
	return newContractDepsWithStorage(t, db, cfg, nil)
}

// newContractDepsWithStorage 与 newContractDeps 同一条链，只多把 storage 注进 NewDeps。
func newContractDepsWithStorage(t *testing.T, db *gorm.DB, cfg *config.Config, st storage.Storage) *Deps {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	d := NewDeps(cfg, db, st, zap.NewNop(), nil)
	// 测试链路没有 Redis：会话模块的黑名单存储换成内存实现，否则「注销先写吊销标记」
	// 这类要真实写凭证状态的端点只能判红（ADR-0060 票2）。Cookie 与有效期口径不变。
	d.Session = security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	// P2 波 3a：handler 随域包收进 internal/auth（包私有），会话改由域包的注册入口注给服务
	// （auth.RegisterRoutes 内把 session 同步进 Service），此处不再改指任何 handler 字段。
	return d
}
