// #1639 管理端能力按侧栏叶子全解耦：授权界面上的一个勾选框 = 侧栏里的一个页面。
//
// 判据是**可达面**而不是键名：给一个自定义角色只勾一片叶子，它就该读得到这一片、读不到别人那一片。
// 收敛前这些端点共用 admin.access / catalog.manage / content.manage 三个粗键，本文件的每一对
// 「200 + 403」在那种键表下都做不到（勾了用户管理就必然拿到讲师管理与 AI 配置）。
//
// 装配与取数走契约测试的同一套（newContractDeps + adminData / adminAccountOf），不另起一条链。
// 每个场景用**独立的管理员**：能力解析带 40s 进程内缓存（internal/admincap），换角色不换人会在
// TTL 内读到旧集合，测试就成了时序的奴隶。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// createdRoleOf 解析「新建角色」的响应（形状同 admin_role 的 DTO）。
func createdRoleOf(t *testing.T, raw json.RawMessage) adminRoleRow {
	t.Helper()
	var row adminRoleRow
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatalf("解析新建角色失败: %v raw=%s", err, raw)
	}
	return row
}

// grantCapabilityRole 建一个只持有 capabilities 的角色，返回它的 role_id。
func grantCapabilityRole(t *testing.T, r *gin.Engine, superTok, name string, capabilities ...string) int {
	t.Helper()
	raw := adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/roles",
		map[string]any{"name": name, "capabilities": capabilities}), http.StatusCreated)
	return createdRoleOf(t, raw).RoleID
}

// adminWithRole 新建管理员账号，再经 **PUT /api/admin/accounts/{id}/role** 挂上角色（授权界面走的
// 就是这条端点），返回它的 token。
func adminWithRole(t *testing.T, r *gin.Engine, sess *security.Session, superTok, username string, roleID int) string {
	t.Helper()
	created := adminAccountOf(t, adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": username, "name": username, "password": "granpass123"}), http.StatusCreated))
	if rec := doWithToken(t, r, superTok, http.MethodPut, "/api/admin/accounts/"+strconv.Itoa(created.AdminID)+"/role",
		map[string]any{"role_id": roleID}); rec.Code != http.StatusOK {
		t.Fatalf("给 %s 挂角色 %d 失败: %d %s", username, roleID, rec.Code, rec.Body.String())
	}
	tok, err := sess.Issue(created.AdminID, username, "admin")
	if err != nil {
		t.Fatalf("签发 %s 的 token 失败: %v", username, err)
	}
	return tok
}

// TestAdminCapabilityGranularity 逐片叶子打一对「该读得到的 / 该被挡住的」。
func TestAdminCapabilityGranularity(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		JWTSecretKey: "admin-granularity-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})

	super := testutil.SeedAdmin(t, db, "gran_super", "x")
	superTok, err := sess.Issue(super.AdminID, super.Username, "admin")
	if err != nil {
		t.Fatalf("签发超管 token 失败: %v", err)
	}

	cases := []struct {
		role  string
		caps  []string
		allow []string
		deny  []string
	}{
		{
			// 审计员：勾不到用户管理（收敛前它与审计同在 admin.access 那一个键的覆盖下）
			role: "审计员", caps: []string{"audit.read"},
			allow: []string{"/api/admin/audit-logs"},
			deny:  []string{"/api/admin/hrwai-users", "/api/admin/tutors", "/api/admin/statistics"},
		},
		{
			role: "用户管理员", caps: []string{"hrwai_user.manage"},
			allow: []string{"/api/admin/hrwai-users"},
			deny:  []string{"/api/admin/audit-logs", "/api/admin/tutors", "/api/admin/course/generate-content/probe"},
		},
		{
			// 讲师管理员：与用户管理彻底分家（#1639 要的那个场景）
			role: "讲师管理员", caps: []string{"tutor.manage"},
			allow: []string{"/api/admin/tutors"},
			deny:  []string{"/api/admin/hrwai-users", "/api/admin/audit-logs"},
		},
		{
			role: "统计员", caps: []string{"statistics.read"},
			allow: []string{"/api/admin/statistics"},
			deny:  []string{"/api/admin/hrwai-users", "/api/admin/audit-logs"},
		},
		{
			// 仪表盘入口键：旧 admin.access 只剩仪表盘这一片可达面，但统计端点它也读（同一份概览）
			role: "仪表盘", caps: []string{"admin.access"},
			allow: []string{"/api/admin/statistics"},
			deny:  []string{"/api/admin/hrwai-users", "/api/admin/tutors", "/api/admin/ai-configs"},
		},
		{
			// 课程管理：读得到共享字典（证件列表、目录树）
			role: "课程管理员", caps: []string{"course.manage"},
			allow: []string{"/api/admin/courses", "/api/admin/credentials", "/api/admin/catalog/tree"},
			deny:  []string{"/api/admin/featured-contents"},
		},
		{
			// 岗位字典：目录树是共享只读面，课程列表不是
			role: "岗位管理员", caps: []string{"position.manage"},
			allow: []string{"/api/admin/positions", "/api/admin/catalog/tree"},
			deny:  []string{"/api/admin/courses", "/api/admin/credentials"},
		},
		{
			role: "证件管理员", caps: []string{"credential.manage"},
			allow: []string{"/api/admin/credentials", "/api/admin/catalog/tree"},
			deny:  []string{"/api/admin/courses", "/api/admin/positions"},
		},
		{
			role: "精选编辑", caps: []string{"featured.manage"},
			allow: []string{"/api/admin/featured-contents"},
			deny:  []string{"/api/admin/courses", "/api/admin/credentials"},
		},
		{
			// 内容生成：生成页要选课程，故课程读面放行；精选与目录树不归它
			role: "内容生成员", caps: []string{"content.generate"},
			allow: []string{"/api/admin/courses"},
			deny:  []string{"/api/admin/featured-contents", "/api/admin/catalog/tree"},
		},
		{
			role: "AI 配置员", caps: []string{"ai_config.manage"},
			allow: []string{"/api/admin/ai-configs"},
			deny:  []string{"/api/admin/hrwai-users", "/api/admin/courses", "/api/admin/statistics"},
		},
		{
			// 共享只读面：巡检视图要读积分流水与举报队列（两条「任一命中」的读面）
			role: "巡检员", caps: []string{"inspection.read"},
			allow: []string{"/api/admin/points/ledger", "/api/admin/job-reports"},
			deny:  []string{"/api/admin/hrwai-users"},
		},
		{
			// 反向的「任一命中」：积分管理也读得到那条流水、举报处置也读得到那个队列
			role: "积分管理员", caps: []string{"points.admin"},
			allow: []string{"/api/admin/points/ledger"},
			deny:  []string{"/api/admin/job-reports", "/api/admin/hrwai-users"},
		},
		{
			role: "举报处置员", caps: []string{"job_report.handle"},
			allow: []string{"/api/admin/job-reports"},
			deny:  []string{"/api/admin/points/ledger", "/api/admin/hrwai-users"},
		},
	}

	for i, tc := range cases {
		// 角色名唯一（admin_role.name 有唯一索引），逐条加次序锚点
		roleID := grantCapabilityRole(t, r, superTok, tc.role+strconv.Itoa(i), tc.caps...)
		tok := adminWithRole(t, r, sess, superTok, "gran_"+strconv.Itoa(i), roleID)

		for _, path := range tc.allow {
			if rec := doWithToken(t, r, tok, http.MethodGet, path, nil); rec.Code != http.StatusOK {
				t.Errorf("角色 %v 应能读 %s, got %d %s", tc.caps, path, rec.Code, rec.Body.String())
			}
		}
		for _, path := range tc.deny {
			if rec := doWithToken(t, r, tok, http.MethodGet, path, nil); rec.Code != http.StatusForbidden {
				t.Errorf("角色 %v 不应能读 %s, got %d %s", tc.caps, path, rec.Code, rec.Body.String())
			}
		}
	}
}

// TestAdminCapabilityGranularity_WriteFacesStayNarrow 共享只读面放行不等于写面放行：
// 课程管理员读得到证件列表与目录树，但证件写面仍只认 credential.manage。
func TestAdminCapabilityGranularity_WriteFacesStayNarrow(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		JWTSecretKey: "admin-granularity-write-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})

	super := testutil.SeedAdmin(t, db, "gran_super_w", "x")
	superTok, err := sess.Issue(super.AdminID, super.Username, "admin")
	if err != nil {
		t.Fatalf("签发超管 token 失败: %v", err)
	}

	courseRole := grantCapabilityRole(t, r, superTok, "课程管理员写面", "course.manage")
	courseTok := adminWithRole(t, r, sess, superTok, "gran_write_course", courseRole)
	genRole := grantCapabilityRole(t, r, superTok, "内容生成员写面", "content.generate")
	genTok := adminWithRole(t, r, sess, superTok, "gran_write_gen", genRole)

	// 写面不接受共享读面的候选
	if rec := doWithToken(t, r, courseTok, http.MethodPost, "/api/admin/credential",
		map[string]any{"name": "不该建出来的证件"}); rec.Code != http.StatusForbidden {
		t.Fatalf("course.manage 不该能建证件, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, r, genTok, http.MethodPost, "/api/admin/course",
		map[string]any{"name": "不该建出来的课程"}); rec.Code != http.StatusForbidden {
		t.Fatalf("content.generate 不该能建课程, got %d %s", rec.Code, rec.Body.String())
	}
}
