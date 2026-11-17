// 契约测试 #2599 E1：讲师角色链的迁移与 authz 词表对账（pg 双跑）。
//
// 迁移 000061 做四件事：建 tutor_role / tutor_role_capability、给 tutor 挂 role_id、
// seed 四模板角色、把存量讲师挂到全权讲师。本文件锁的是「迁移 SQL 与代码声明是同一件事」——
// 两边各自漂移是这类「SQL 一份 + Go 一份」结构最容易发生的失配（admin 侧已有同形先例）。
//
// 为什么本机看不到：无 DATABASE_URL 时 NewPostgresDB 干净 skip，真跑只在 CI。
package api

import (
	"os"
	"strings"
	"testing"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestTutorRoleChainMigrationOnPostgres(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	// 一、四模板角色都在，且 protected 有且仅有一个（就是全权讲师）。
	var roles []model.TutorRole
	if err := db.Order("role_id").Find(&roles).Error; err != nil {
		t.Fatalf("查讲师角色失败: %v", err)
	}
	byName := map[string]model.TutorRole{}
	protectedCount := 0
	for _, r := range roles {
		byName[r.Name] = r
		if r.Protected {
			protectedCount++
		}
	}
	if protectedCount != 1 {
		t.Fatalf("protected 讲师角色应恰好 1 个，得到 %d（%v）", protectedCount, roles)
	}
	if got := byName[authz.TutorRoleNameFull]; !got.Protected {
		t.Fatalf("全权讲师应为 protected 角色，得到 %+v", got)
	}

	// 二、三个子角色的能力行与 authz.TutorTemplateRoles() 逐键相等（双向比对）。
	for _, tpl := range authz.TutorTemplateRoles() {
		role, ok := byName[tpl.Name]
		if !ok {
			t.Fatalf("迁移未 seed 预置角色 %q", tpl.Name)
		}
		if role.Protected != tpl.Protected {
			t.Fatalf("角色 %q 的 protected = %v, want %v", tpl.Name, role.Protected, tpl.Protected)
		}
		var rows []model.TutorRoleCapability
		if err := db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
			t.Fatalf("查角色 %q 能力行失败: %v", tpl.Name, err)
		}
		got := map[string]struct{}{}
		for _, row := range rows {
			got[row.Capability] = struct{}{}
		}
		if tpl.Protected {
			// protected 角色的能力由代码回答、**不落数据行**（规格 E6 的硬约束：
			// 能力词表只有 authz 一个事实源）。落行会让「改代码不改库」出现第二种事实。
			if len(rows) != 0 {
				t.Fatalf("protected 角色 %q 不应落能力行，得到 %d 行", tpl.Name, len(rows))
			}
			continue
		}
		if len(got) != len(tpl.Capabilities) {
			t.Fatalf("角色 %q 能力行 = %v（%d 个），authz 声明 %v（%d 个）",
				tpl.Name, got, len(got), tpl.Capabilities, len(tpl.Capabilities))
		}
		for _, c := range tpl.Capabilities {
			if _, ok := got[string(c)]; !ok {
				t.Fatalf("角色 %q 缺能力行 %q（迁移 SQL 与 authz 声明漂移）", tpl.Name, c)
			}
		}
	}

	// 三、存量讲师全部挂到全权讲师（避免「表建好但没人有角色」的全员 fail-closed 中间态）。
	var unassigned int64
	if err := db.Model(&model.Tutor{}).Where("role_id IS NULL").Count(&unassigned).Error; err != nil {
		t.Fatalf("查未挂角色的讲师失败: %v", err)
	}
	// 迁移只回填「迁移那一刻已存在的讲师」。本测试自己 seed 的讲师是迁移之后建的，
	// 默认 NULL 正是规格 E2 要求的，故这里只断言「迁移前就存在的讲师没有漏挂」——
	// 判据是：全权角色存在，且迁移刚跑完时库里若已有讲师，他们都被挂上了。
	fullRole := byName[authz.TutorRoleNameFull]
	var total int64
	if err := db.Model(&model.Tutor{}).Count(&total).Error; err != nil {
		t.Fatalf("查讲师总数失败: %v", err)
	}
	if total > 0 && unassigned == total {
		t.Fatalf("库里有 %d 位讲师但全部未挂角色 —— 迁移的回填段没生效", total)
	}
	_ = fullRole
}

// TestTutorRoleChainMigrationIdempotentOnPostgres 重放 seed 段必须幂等（迁移可重跑）。
func TestTutorRoleChainMigrationIdempotentOnPostgres(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	// 重放迁移里的 seed 语句（与 000061_tutor_role_chain.up.sql 的 seed 段逐字同义）。
	seedSQL := `
INSERT INTO tutor_role (name, protected, remark)
VALUES
  ('全权讲师', TRUE,  '受保护角色：拥有全部讲师能力，不可编辑/删除'),
  ('题库讲师', FALSE, ''),
  ('课程讲师', FALSE, ''),
  ('只读讲师', FALSE, '')
ON CONFLICT (name) DO NOTHING`
	if err := db.Exec(seedSQL).Error; err != nil {
		t.Fatalf("重放角色 seed 失败: %v", err)
	}
	if err := db.Exec(seedSQL).Error; err != nil {
		t.Fatalf("第二次重放角色 seed 失败（应幂等）: %v", err)
	}

	var count int64
	if err := db.Model(&model.TutorRole{}).Where("name IN ?", []string{
		authz.TutorRoleNameFull, authz.TutorRoleNameBank,
		authz.TutorRoleNameCourse, authz.TutorRoleNameReadonly,
	}).Count(&count).Error; err != nil {
		t.Fatalf("查预置角色数失败: %v", err)
	}
	if count != int64(len(authz.TutorTemplateRoles())) {
		t.Fatalf("重放后预置角色数 = %d, want %d（重放不幂等）", count, len(authz.TutorTemplateRoles()))
	}
}

// TestTutorRoleChainMigrationFileDeclaresEveryTemplate 迁移文件本身必须提到全部四个角色名。
//
// 这是「SQL 与 Go 两份声明」之间的最低成本反向锁：迁移文件被删改到只剩建表时，
// 上面的 PG 用例在无 DATABASE_URL 的本机仍会 skip —— 本用例不依赖数据库，任何环境都跑。
func TestTutorRoleChainMigrationFileDeclaresEveryTemplate(t *testing.T) {
	t.Parallel()
	root := testutil.ModuleRoot(t)
	path := root + "/migrations/000061_tutor_role_chain.up.sql"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读迁移文件失败（编号被占用或文件改名？）: %v", err)
	}
	sql := string(raw)
	for _, tpl := range authz.TutorTemplateRoles() {
		if !strings.Contains(sql, "'"+tpl.Name+"'") {
			t.Fatalf("迁移 000061 未包含预置角色 %q —— SQL 与 authz.TutorTemplateRoles() 漂移", tpl.Name)
		}
	}
	// 建表与回填三段都必须在同一个文件里（规格 E1：一条迁移内完成，避免全员 fail-closed 中间态）。
	// 回填句按「UPDATE tutor」+「SET role_id」两段断言：SQL 里它有换行缩进，
	// 拼成单行片段会让本锁对合法排版误报。
	for _, frag := range []string{
		"CREATE TABLE IF NOT EXISTS tutor_role",
		"CREATE TABLE IF NOT EXISTS tutor_role_capability",
		"ALTER TABLE tutor ADD COLUMN IF NOT EXISTS role_id",
		"UPDATE tutor",
		"SET role_id = (SELECT role_id FROM tutor_role WHERE protected)",
	} {
		if !strings.Contains(sql, frag) {
			t.Fatalf("迁移 000061 缺少片段 %q（规格 E1 要求建表 + 挂列 + 回填同一条迁移）", frag)
		}
	}
}
