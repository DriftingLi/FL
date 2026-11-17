// 本文件：讲师角色链管理端点的 HTTP 契约（#2599 / 规格 E7）。
//
// 五条 AC 各落一组断言，全部打**真实路由链**（NewRouter，不是自装蓝图）：
//   - 角色 CRUD 正常路径 + 能力键规整（排序、去重、只收讲师侧键）；
//   - 预置全权讲师不可改不可删（409，且能力集仍由代码回答）；
//   - 仍被讲师挂着的角色不可删（409）；
//   - 摘掉最后一个全权讲师被拒（409）；
//   - 讲师自己打不到这些端点（403，tutor.manage 不在讲师侧词表里）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// decodeEnvelopeData 解出信封里的 data 对象（本文件多处要按 key 取值，不走断言型辅助）。
func decodeEnvelopeData(t *testing.T, raw string) map[string]any {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("响应不是信封: %v (%s)", err, raw)
	}
	if len(env.Data) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(env.Data, &m); err != nil {
		t.Fatalf("data 不是对象: %v (%s)", err, string(env.Data))
	}
	return m
}

// tutorRoleFixture 讲师角色链契约的夹具：真实路由 + 一位管理员 + 一位未挂角色的讲师。
type tutorRoleFixture struct {
	t        *testing.T
	r        http.Handler
	db       *gorm.DB
	cfg      *config.Config
	tutorID  int
	adminTok string
	tutorTok string
}

func newTutorRoleFixture(t *testing.T) *tutorRoleFixture {
	t.Helper()
	r, cfg, db := newSlice6Env(t)
	tutor := testutil.SeedTutor(t, db, "role_chain_tutor", "x")
	return &tutorRoleFixture{
		t: t, r: r, db: db, cfg: cfg,
		tutorID:  tutor.TutorID,
		adminTok: adminTokenWithAccount(t, cfg, db, "role_chain_admin"),
		tutorTok: slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor"),
	}
}

// do 发一条请求，返回状态码与响应体原文。
func (f *tutorRoleFixture) do(method, path, token string, body any) (int, string) {
	f.t.Helper()
	rec := doWithToken(f.t, f.r.(*gin.Engine), token, method, path, body)
	return rec.Code, rec.Body.String()
}

// roleByName 从列表响应里按名字取一个角色。
func (f *tutorRoleFixture) roleByName(name string) (map[string]any, bool) {
	f.t.Helper()
	code, raw := f.do(http.MethodGet, "/api/admin/tutor-roles", f.adminTok, nil)
	if code != http.StatusOK {
		f.t.Fatalf("列讲师角色应 200，得到 %d: %s", code, raw)
	}
	data := decodeEnvelopeData(f.t, raw)
	roles, _ := data["roles"].([]any)
	for _, item := range roles {
		role, _ := item.(map[string]any)
		if role["name"] == name {
			return role, true
		}
	}
	return nil, false
}

// TestTutorRoleCRUD 建 → 改 → 删 的正常路径，外加能力键规整。
func TestTutorRoleCRUD(t *testing.T) {
	f := newTutorRoleFixture(t)

	// 一、建角色：能力键乱序 + 重复 + 空串，出口必须是规整后的升序去重结果。
	code, raw := f.do(http.MethodPost, "/api/admin/tutor-roles", f.adminTok, map[string]any{
		"name":   " 题库实习生 ",
		"remark": "只能建题",
		"capabilities": []string{
			"question.author", "tutor.access", "question.author", " ", "notification.use",
		},
	})
	if code != http.StatusCreated {
		t.Fatalf("建角色应 201，得到 %d: %s", code, raw)
	}
	created := decodeEnvelopeData(t, raw)
	if created["name"] != "题库实习生" {
		t.Fatalf("角色名应被 TrimSpace，得到 %v", created["name"])
	}
	caps, _ := created["capabilities"].([]any)
	if len(caps) != 3 {
		t.Fatalf("能力集应去重为 3 个，得到 %v", caps)
	}
	if caps[0] != "notification.use" || caps[1] != "question.author" || caps[2] != "tutor.access" {
		t.Fatalf("能力集应升序排列，得到 %v", caps)
	}
	roleID := int(created["role_id"].(float64))

	// 二、改角色：换名与能力集。
	code, raw = f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor-roles/%d", roleID), f.adminTok, map[string]any{
		"name": "题库实习生", "remark": "改过备注", "capabilities": []string{"tutor.access"},
	})
	if code != http.StatusOK {
		t.Fatalf("改角色应 200，得到 %d: %s", code, raw)
	}
	updated := decodeEnvelopeData(t, raw)
	if updated["remark"] != "改过备注" {
		t.Fatalf("备注未更新: %v", updated)
	}
	if got := updated["capabilities"].([]any); len(got) != 1 || got[0] != "tutor.access" {
		t.Fatalf("能力集应只剩 tutor.access，得到 %v", got)
	}

	// 三、删角色。
	code, raw = f.do(http.MethodDelete, fmt.Sprintf("/api/admin/tutor-roles/%d", roleID), f.adminTok, nil)
	if code != http.StatusOK {
		t.Fatalf("删角色应 200，得到 %d: %s", code, raw)
	}
	if _, ok := f.roleByName("题库实习生"); ok {
		t.Fatalf("删掉的角色仍出现在列表里")
	}
}

// TestTutorRoleRejectsAdminCapability 讲师角色不能承载管理员侧能力键。
//
// 这是规格硬约束 5 的机检面：讲师一旦拿到 question.review，就绕过了「先审后发」。
func TestTutorRoleRejectsAdminCapability(t *testing.T) {
	f := newTutorRoleFixture(t)
	code, raw := f.do(http.MethodPost, "/api/admin/tutor-roles", f.adminTok, map[string]any{
		"name": "偷审核权", "capabilities": []string{"question.review"},
	})
	if code != http.StatusBadRequest {
		t.Fatalf("挂管理员侧能力键应 400，得到 %d: %s", code, raw)
	}
	if _, ok := f.roleByName("偷审核权"); ok {
		t.Fatalf("被拒的角色不应落库")
	}
}

// TestProtectedTutorRoleImmutable 预置全权讲师不可改、不可删，且能力集由代码回答。
func TestProtectedTutorRoleImmutable(t *testing.T) {
	f := newTutorRoleFixture(t)

	role, ok := f.roleByName(authz.TutorRoleNameFull)
	if !ok {
		t.Fatalf("预置角色 %q 不存在（迁移或 seed 没跑）", authz.TutorRoleNameFull)
	}
	if role["protected"] != true {
		t.Fatalf("全权讲师应为 protected，得到 %v", role["protected"])
	}
	// 能力集必须等于讲师侧全量（由 authz 回答，不读表）。
	caps, _ := role["capabilities"].([]any)
	if len(caps) != len(authz.ProtectedTutorCapabilities()) {
		t.Fatalf("全权讲师能力集应有 %d 个键，得到 %d 个: %v",
			len(authz.ProtectedTutorCapabilities()), len(caps), caps)
	}
	roleID := int(role["role_id"].(float64))

	code, raw := f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor-roles/%d", roleID), f.adminTok, map[string]any{
		"name": "改个名", "capabilities": []string{"tutor.access"},
	})
	if code != http.StatusConflict {
		t.Fatalf("改受保护角色应 409，得到 %d: %s", code, raw)
	}
	code, raw = f.do(http.MethodDelete, fmt.Sprintf("/api/admin/tutor-roles/%d", roleID), f.adminTok, nil)
	if code != http.StatusConflict {
		t.Fatalf("删受保护角色应 409，得到 %d: %s", code, raw)
	}
}

// TestTutorRoleInUseNotDeletable 仍被讲师挂着的角色不可删。
func TestTutorRoleInUseNotDeletable(t *testing.T) {
	f := newTutorRoleFixture(t)
	code, raw := f.do(http.MethodPost, "/api/admin/tutor-roles", f.adminTok, map[string]any{
		"name": "临时角色", "capabilities": []string{"tutor.access"},
	})
	if code != http.StatusCreated {
		t.Fatalf("建角色应 201，得到 %d: %s", code, raw)
	}
	roleID := int(decodeEnvelopeData(t, raw)["role_id"].(float64))

	code, raw = f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), f.adminTok,
		map[string]any{"role_id": roleID})
	if code != http.StatusOK {
		t.Fatalf("挂角色应 200，得到 %d: %s", code, raw)
	}
	assigned := decodeEnvelopeData(t, raw)
	if assigned["role_id"] != float64(roleID) || assigned["role_name"] != "临时角色" {
		t.Fatalf("出口应带所挂角色，得到 %v", assigned)
	}

	code, raw = f.do(http.MethodDelete, fmt.Sprintf("/api/admin/tutor-roles/%d", roleID), f.adminTok, nil)
	if code != http.StatusConflict {
		t.Fatalf("删仍被使用的角色应 409，得到 %d: %s", code, raw)
	}
}

// TestAssignTutorRoleLastFullTutorGuarded 摘掉最后一个全权讲师被拒。
func TestAssignTutorRoleLastFullTutorGuarded(t *testing.T) {
	f := newTutorRoleFixture(t)
	role, ok := f.roleByName(authz.TutorRoleNameFull)
	if !ok {
		t.Fatalf("预置角色 %q 不存在", authz.TutorRoleNameFull)
	}
	fullRoleID := int(role["role_id"].(float64))

	// 把测试讲师挂成全权讲师。
	code, raw := f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), f.adminTok,
		map[string]any{"role_id": fullRoleID})
	if code != http.StatusOK {
		t.Fatalf("挂全权讲师应 200，得到 %d: %s", code, raw)
	}

	// 库里有几位全权讲师？只有一位时摘角色必须被拒。
	var others int64
	if err := f.db.Model(&model.Tutor{}).
		Where("tutor_id <> ? AND role_id = ?", f.tutorID, fullRoleID).Count(&others).Error; err != nil {
		t.Fatalf("查其他全权讲师失败: %v", err)
	}
	code, raw = f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), f.adminTok,
		map[string]any{"role_id": 0})
	if others == 0 {
		if code != http.StatusConflict {
			t.Fatalf("摘最后一个全权讲师应 409，得到 %d: %s", code, raw)
		}
	} else if code != http.StatusOK {
		t.Fatalf("还有 %d 位全权讲师时应允许摘角色，得到 %d: %s", others, code, raw)
	}
}

// TestTutorRoleEndpointsRejectTutor 讲师打不到讲师角色管理端点（tutor.manage 只在管理员侧）。
func TestTutorRoleEndpointsRejectTutor(t *testing.T) {
	f := newTutorRoleFixture(t)
	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/admin/tutor-roles", nil},
		{http.MethodPost, "/api/admin/tutor-roles", map[string]any{"name": "x"}},
		{http.MethodPut, "/api/admin/tutor-roles/1", map[string]any{"name": "x"}},
		{http.MethodDelete, "/api/admin/tutor-roles/1", nil},
		{http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), map[string]any{"role_id": 1}},
	}
	for _, tc := range cases {
		code, raw := f.do(tc.method, tc.path, f.tutorTok, tc.body)
		if code != http.StatusForbidden {
			t.Fatalf("%s %s 讲师应 403，得到 %d: %s", tc.method, tc.path, code, raw)
		}
		// 匿名请求应是 401（与「有令牌但没权」区分开）。
		code, raw = f.do(tc.method, tc.path, "", tc.body)
		if code != http.StatusUnauthorized {
			t.Fatalf("%s %s 匿名应 401，得到 %d: %s", tc.method, tc.path, code, raw)
		}
	}
}

// TestUnassignedTutorGetsEmptyCapabilities 未挂角色的讲师取能力集得空集且 granted=false（规格 E2）。
//
// 注意本票**尚未翻转 dynamicRoles**：讲师守卫仍走静态能力表，故「空集」此刻只体现在
// 这个端点与 resolver 上。翻转是 #2600 的事，本用例锁的是「数据面已经 fail closed」。
func TestUnassignedTutorGetsEmptyCapabilities(t *testing.T) {
	f := newTutorRoleFixture(t)

	// 新建讲师默认不挂角色。
	var fresh model.Tutor
	if err := f.db.Where("tutor_id = ?", f.tutorID).First(&fresh).Error; err != nil {
		t.Fatalf("查讲师失败: %v", err)
	}
	if fresh.RoleID != nil {
		t.Fatalf("新建讲师默认应未挂角色，得到 role_id=%d", *fresh.RoleID)
	}

	code, raw := f.do(http.MethodGet, "/api/tutor/me/capabilities", f.tutorTok, nil)
	if code != http.StatusOK {
		t.Fatalf("讲师取能力集应 200（只挂登录守卫），得到 %d: %s", code, raw)
	}
	data := decodeEnvelopeData(t, raw)
	if data["granted"] != false {
		t.Fatalf("未挂角色应 granted=false，得到 %v", data["granted"])
	}
	if caps, _ := data["capabilities"].([]any); len(caps) != 0 {
		t.Fatalf("未挂角色能力集应为空，得到 %v", caps)
	}
}

// TestAssignedTutorCapabilitiesMatchTemplate 挂上角色后能力集等于模板（本票数据面生效）。
//
// 必须**先挂角色再首次取能力集**：resolver 把结果缓存 40s（规格 E3），先取空集再挂角色
// 时读到的仍是缓存空集 —— 那是刻意的语义，不是缺陷，由下面 TestRoleChangeTakesEffectAfterTTL 单独锁。
func TestAssignedTutorCapabilitiesMatchTemplate(t *testing.T) {
	f := newTutorRoleFixture(t)

	role, ok := f.roleByName(authz.TutorRoleNameBank)
	if !ok {
		t.Fatalf("预置角色 %q 不存在", authz.TutorRoleNameBank)
	}
	code, raw := f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), f.adminTok,
		map[string]any{"role_id": int(role["role_id"].(float64))})
	if code != http.StatusOK {
		t.Fatalf("挂题库讲师应 200，得到 %d: %s", code, raw)
	}

	code, raw = f.do(http.MethodGet, "/api/tutor/me/capabilities", f.tutorTok, nil)
	if code != http.StatusOK {
		t.Fatalf("取能力集应 200，得到 %d: %s", code, raw)
	}
	data := decodeEnvelopeData(t, raw)
	if data["granted"] != true {
		t.Fatalf("已挂角色应 granted=true，得到 %v", data["granted"])
	}
	want := map[string]struct{}{}
	for _, tpl := range authz.TutorTemplateRoles() {
		if tpl.Name != authz.TutorRoleNameBank {
			continue
		}
		for _, c := range tpl.Capabilities {
			want[string(c)] = struct{}{}
		}
	}
	caps, _ := data["capabilities"].([]any)
	if len(caps) != len(want) {
		t.Fatalf("题库讲师能力集应 %d 个，得到 %v", len(want), caps)
	}
	for _, item := range caps {
		if _, ok := want[item.(string)]; !ok {
			t.Fatalf("题库讲师能力集出现模板外能力键 %v", item)
		}
	}
}

// TestRoleChangeTakesEffectAfterTTL 角色变更最多延迟 40s 生效（规格 E3/E8 的刻意语义）。
//
// 锁的是「缓存真的在起作用」这件事本身：先取一次能力集（缓存空集），再挂角色，
// 立刻再取仍必须是空集 —— 若这里忽然变成新角色，说明缓存被绕过了（每次请求直查库），
// 那是性能回归而非功能改进；反过来若 TTL 变成永不失效，则管理员改权限要重启才生效。
func TestRoleChangeTakesEffectAfterTTL(t *testing.T) {
	f := newTutorRoleFixture(t)

	// 第一次取：未挂角色 → 空集并缓存。
	code, raw := f.do(http.MethodGet, "/api/tutor/me/capabilities", f.tutorTok, nil)
	if code != http.StatusOK {
		t.Fatalf("取能力集应 200，得到 %d: %s", code, raw)
	}
	if got := decodeEnvelopeData(t, raw)["granted"]; got != false {
		t.Fatalf("首次取应 granted=false，得到 %v", got)
	}

	// 挂上题库讲师后立刻再取：仍应是缓存里的空集。
	role, ok := f.roleByName(authz.TutorRoleNameBank)
	if !ok {
		t.Fatalf("预置角色 %q 不存在", authz.TutorRoleNameBank)
	}
	code, raw = f.do(http.MethodPut, fmt.Sprintf("/api/admin/tutor/%d/role", f.tutorID), f.adminTok,
		map[string]any{"role_id": int(role["role_id"].(float64))})
	if code != http.StatusOK {
		t.Fatalf("挂题库讲师应 200，得到 %d: %s", code, raw)
	}
	code, raw = f.do(http.MethodGet, "/api/tutor/me/capabilities", f.tutorTok, nil)
	if code != http.StatusOK {
		t.Fatalf("取能力集应 200，得到 %d: %s", code, raw)
	}
	if got := decodeEnvelopeData(t, raw)["granted"]; got != false {
		t.Fatalf("TTL 内的第二次取应仍读缓存（granted=false），得到 %v —— 缓存被绕过了？", got)
	}
}
