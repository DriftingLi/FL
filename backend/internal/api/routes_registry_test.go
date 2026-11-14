package api

import (
	"testing"

	"forklift-training/internal/config"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// 域注册表覆盖锁（ADR-0047 §6 / spec #933）：
//  1. 域名非空且唯一；
//  2. 用域注册表构出的路由总数等于登记常量——**新增蓝图却忘记登记会让数量下降**，测试即红；
//     反过来，重复登记会在 gin 注册期直接 panic（同一路径重复注册），由本测试触发暴露。
func TestRouteRegistryCoverage(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	seen := map[string]bool{}
	for _, reg := range routeRegistrars {
		if reg.Domain == "" || reg.Register == nil {
			t.Fatalf("域注册项不完整: %+v", reg)
		}
		if seen[reg.Domain] {
			t.Fatalf("域名重复: %s", reg.Domain)
		}
		seen[reg.Domain] = true
	}
	if len(routeRegistrars) == 0 {
		t.Fatal("域注册表为空")
	}

	db := testutil.NewMemoryDB(t)
	r := NewRouter(NewDeps(&config.Config{}, db, nil, zap.NewNop(), nil))
	if got := len(r.Routes()); got != expectedRouteCount {
		t.Fatalf("路由总数 = %d, want %d（新增蓝图请同步登记到 routeRegistrars；若本次是有意增删端点，请更新本常量）", got, expectedRouteCount)
	}
}

// expectedRouteCount 域注册表构出的路由总数基线（2026-11-04 #2242 v2 引导只读面落地：
// 加 5 条（GET /api/ai-assistant/diagnosis/{intents,systems,faults,code-hint,health}）→ 347）。
// 历史：2026-11-03 #2237 外部诊断 v1 通道退役：删 4 条只读代理
// （GET /api/ai-assistant/diagnosis/{brands,models,fault-codes,manual/*}）、
// 加 1 条回答配图代理（GET /api/ai-assistant/diagnosis/ossimg/*）→ 342。
// 2026-10-28 #2070 新增 4 条反馈端点
// （POST/GET /api/feedback、GET /api/admin/feedback、POST /api/admin/feedback/{id}/handle）→ 345；
// 2026-10-28 #2065 新增 1 条浏览课程上报端点（POST /api/course/{course_id}/browse）→ 341；
// 2026-10-28 #2064 新增 1 条商城在售列表端点（GET /api/points/shop/items）→ 340；
// 2026-10-10，#1640 新增 1 条代重置口令端点（PUT /api/admin/accounts/{admin_id}/password）→ 339；
// 2026-10-10 #1632 新增 2 条管理员账号端点（新建/删除）→ 338；
// 2026-10-10 #1621 段4 新增 6 条授权管理端点（角色 CRUD + 管理员挂角色）→ 336；
// 2026-10-10 #1618 段1 新增 GET /api/admin/me/capabilities → 330；
// 2026-09-19 第十二波票 6 新增 POST /api/question-bank/questions/{id}/submit → 329；
// 2026-09-14 全局搜索升级 #982 新增 GET /api/admin/search-facts/zero-results → 315；
// 2026-09-17 学员笔记域 #1078 新增 4 条 → 319。
// 2026-11-13 #2494 新增 6 条导师文件库端点（POST /api/tutor/files/upload、
// PUT /api/tutor/files/upload/{session_id}/chunk/{part_number}、POST …/complete、
// POST …/abort、GET /api/tutor/files、DELETE /api/tutor/files/{file_id}）→ 353。
// 2026-11-13 #2495 断点续传清单端点（GET /api/tutor/files/upload/{session_id}）→ 354。
// 2026-11-13 #2497 挂接/换挂/改名 4 条（POST /api/tutor/files/{file_id}/attach|detach|rename、
// POST /api/tutor/chapter/{chapter_id}/files/swap）→ 358。
// 2026-11-13 #2498 审核流 3 条（POST /api/tutor/chapter/{chapter_id}/submit-review、
// POST /api/admin/chapter/{chapter_id}/approve-review|reject-review）→ 361。
// 2026-11-14 #2499 审核队列 4 条（GET /api/admin/chapter-reviews/pending、
// GET /api/admin/chapter-reviews/{chapter_id}/snapshot、
// POST /api/admin/chapter-reviews/batch-approve|batch-reject）→ 365。
const expectedRouteCount = 365
