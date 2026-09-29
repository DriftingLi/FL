// 证件外键动作的迁移文本锁（#1360，不依赖 PG 也能红的那半）。
//
// 为什么要有本文件：#1360 的判据 1「有练习进度时删证件成功」承重在那条 ON DELETE CASCADE 上，
// 而 CASCADE 只能在真 Postgres 上跑到（SQLite 测试库由 AutoMigrate 建表、不执行 migrations，
// 也建不出 REFERENCES 的删除动作）。PG 契约用例本机无 DATABASE_URL 会干净 skip ⇒ 首跑在 CI。
// 这里补的是「CI 之前也能红」的一侧：迁移文本里那条动作在不在、方向对不对、
// 投稿侧有没有被顺手改成 CASCADE（那会把「投稿阻塞」退化成静默删内容资产）。
//
// 判据都写成纯函数 + 合成破坏样本：删掉/改坏任一句，本文件必须红（不是「读一遍文件没报错」）。
package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// migrationFile 一份迁移正文（name 为文件名，用于报错定位与棘轮判定）。
type migrationFile struct {
	name    string
	content string
}

// practiceFKCascadeRe 抓「给 practice_progress 的 credential_id 建外键且带 ON DELETE CASCADE」。
var practiceFKCascadeRe = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+practice_progress[\s\S]*?ADD\s+CONSTRAINT\s+practice_progress_credential_id_fkey\s+FOREIGN\s+KEY\s*\(\s*credential_id\s*\)\s*REFERENCES\s+credential\s*\(\s*id\s*\)\s*ON\s+DELETE\s+CASCADE`)

// practiceFKPlainRe 抓「同一枚外键、没有 CASCADE」的形态（000040 的 down 就是它，up 侧不许出现）。
var practiceFKPlainRe = regexp.MustCompile(`(?is)ADD\s+CONSTRAINT\s+practice_progress_credential_id_fkey\s+FOREIGN\s+KEY\s*\(\s*credential_id\s*\)\s*REFERENCES\s+credential\s*\(\s*id\s*\)\s*;`)

// contributionCascadeRe 抓「把投稿表对 credential 的外键改成级联删除」——那是要防的反面。
// 两种写法各一条：
//   - 建表内联列（000020 的形状：credential_id INTEGER NOT NULL REFERENCES credential(id)，
//     全仓只有它用 INTEGER，其余 credential_id 都是 INT，故这条不会误伤 real_exam_paper 那条合法级联）；
//   - 后续 ALTER 收回同一列。
//
// 为什么不能用「语句里同时出现 credential_id 与 ON DELETE CASCADE」这种宽松判据：
// 000020 的 user_id 行本来就带 hrwai_users 的合法级联，宽松判据会把正样本读成破坏样本。
var contributionInlineCascadeRe = regexp.MustCompile(`(?is)credential_id\s+INTEGER\s+NOT\s+NULL\s+REFERENCES\s+credential\s*\(\s*id\s*\)\s+ON\s+DELETE\s+CASCADE`)
var contributionAlterCascadeRe = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?user_contribution\b[\s\S]*?FOREIGN\s+KEY\s*\(\s*credential_id\s*\)\s*REFERENCES\s+credential\s*\(\s*id\s*\)\s+ON\s+DELETE\s+CASCADE`)

// credentialFKShape 汇总三个判据的结论。
type credentialFKShape struct {
	cascadeAddedInUp   []string // 建出 CASCADE 的 up 文件（必须恰有一个）
	plainReAddInUp     []string // up 里又建回无动作外键的文件（必须为空）
	contributionCascad []string // 把投稿侧改成级联的文件（必须为空）
}

// checkCredentialFKShape 扫一批迁移正文（纯函数：真实目录与合成破坏样本共用同一判据）。
func checkCredentialFKShape(files []migrationFile) credentialFKShape {
	var s credentialFKShape
	for _, f := range files {
		text := strings.ReplaceAll(f.content, "\r\n", "\n")
		if !strings.HasSuffix(f.name, ".up.sql") {
			continue
		}
		if practiceFKCascadeRe.MatchString(text) {
			s.cascadeAddedInUp = append(s.cascadeAddedInUp, f.name)
		}
		// 「先 DROP 再 ADD ... ;」的 down 形态若出现在 up 里，等于把级联又收回去了。
		if practiceFKPlainRe.MatchString(text) && !strings.Contains(text, "ON DELETE CASCADE") {
			s.plainReAddInUp = append(s.plainReAddInUp, f.name)
		}
		if contributionInlineCascadeRe.MatchString(text) || contributionAlterCascadeRe.MatchString(text) {
			s.contributionCascad = append(s.contributionCascad, f.name)
		}
	}
	return s
}

// listMigrationSQLFiles 列出目录里的迁移正文文件名。
func listMigrationSQLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

func readMigrationsForShape(t *testing.T) []migrationFile {
	t.Helper()
	dir := migrationsDir(t)
	names, err := listMigrationSQLFiles(dir)
	if err != nil {
		t.Fatalf("列 migrations 目录失败: %v", err)
	}
	out := make([]migrationFile, 0, len(names))
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatalf("读迁移 %s 失败: %v", n, err)
		}
		out = append(out, migrationFile{name: n, content: string(data)})
	}
	return out
}

// TestCredentialFKShapeInMigrations 真迁移必须同时满足三件事：
// CASCADE 恰好由一条 up 建出来、没有任何 up 把它收回无动作、投稿侧从未被改成级联。
func TestCredentialFKShapeInMigrations(t *testing.T) {
	files := readMigrationsForShape(t)
	if len(files) == 0 {
		t.Fatal("没读到任何迁移文件")
	}
	shape := checkCredentialFKShape(files)
	if len(shape.cascadeAddedInUp) != 1 {
		t.Fatalf("practice_progress.credential_id 的 ON DELETE CASCADE 应由且仅由一条 up 迁移建出来，实得 %d 条：%s",
			len(shape.cascadeAddedInUp), strings.Join(shape.cascadeAddedInUp, ", "))
	}
	if !strings.HasPrefix(shape.cascadeAddedInUp[0], "000040_") {
		t.Fatalf("级联动作应落在 000040（000013 已上过生产，就地改只影响空库重建），实得 %s", shape.cascadeAddedInUp[0])
	}
	if len(shape.plainReAddInUp) > 0 {
		t.Fatalf("有 up 迁移把该外键收回无删除动作：%s（判据 1「有练习进度时删证件成功」会退回 500）",
			strings.Join(shape.plainReAddInUp, ", "))
	}
	if len(shape.contributionCascad) > 0 {
		t.Fatalf("投稿侧外键被改成级联删除：%s（投稿是内容资产，必须 NO ACTION + 预检给条数）",
			strings.Join(shape.contributionCascad, ", "))
	}
	t.Logf("CASCADE 宿主：%s；投稿侧保持 NO ACTION", shape.cascadeAddedInUp[0])
}

// TestCredentialFKShapeDownRevertsAction 000040 的 down 必须把动作收回无删除（回滚 = 退回旧代码形状），
// 且收回的是同一枚具名约束，不留两个外键。
func TestCredentialFKShapeDownRevertsAction(t *testing.T) {
	files := readMigrationsForShape(t)
	var down string
	for _, f := range files {
		if strings.HasPrefix(f.name, "000040_") && strings.HasSuffix(f.name, ".down.sql") {
			down = f.content
		}
	}
	if down == "" {
		t.Fatal("找不到 000040 的 down 迁移（成对性由 TestMigrationFilesArePairedAndNamed 管，这里要的是正文）")
	}
	if !strings.Contains(flattenSQL(down), "DROP CONSTRAINT IF EXISTS practice_progress_credential_id_fkey") {
		t.Fatalf("down 必须先 DROP 那枚具名外键：%s", down)
	}
	if !practiceFKPlainRe.MatchString(down) {
		t.Fatalf("down 应把外键收回无 ON DELETE 动作（退回 000013 的形状）：%s", down)
	}
	if strings.Contains(strings.ToUpper(down), "ON DELETE CASCADE") {
		t.Fatalf("down 里不许留 CASCADE，否则回滚等于没回滚：%s", down)
	}
}

// TestCredentialFKShapeDiscriminates 防恒绿：三条判据各喂一份合成破坏正文，必须逐个判红。
// 只跑通过的那一次不算验收——本函数就是「我故意弄坏被测物，它会不会红」的答案。
func TestCredentialFKShapeDiscriminates(t *testing.T) {
	up := migrationFile{name: "000040_x.up.sql", content: `
ALTER TABLE practice_progress DROP CONSTRAINT IF EXISTS practice_progress_credential_id_fkey;
ALTER TABLE practice_progress ADD CONSTRAINT practice_progress_credential_id_fkey
    FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;`}
	good := []migrationFile{up}
	if s := checkCredentialFKShape(good); len(s.cascadeAddedInUp) != 1 || len(s.plainReAddInUp) != 0 || len(s.contributionCascad) != 0 {
		t.Fatalf("正样本被判红: %+v", s)
	}

	// 破坏一：整条迁移被删掉 ⇒ 没有任何 up 建级联。
	if s := checkCredentialFKShape(nil); len(s.cascadeAddedInUp) != 0 {
		t.Fatalf("删光迁移后仍报有 CASCADE: %+v", s)
	}
	// 破坏二：把 CASCADE 改回无动作。
	noAction := migrationFile{name: "000040_x.up.sql", content: `
ALTER TABLE practice_progress DROP CONSTRAINT IF EXISTS practice_progress_credential_id_fkey;
ALTER TABLE practice_progress ADD CONSTRAINT practice_progress_credential_id_fkey
    FOREIGN KEY (credential_id) REFERENCES credential(id);`}
	s := checkCredentialFKShape([]migrationFile{noAction})
	if len(s.cascadeAddedInUp) != 0 || len(s.plainReAddInUp) != 1 {
		t.Fatalf("改掉 CASCADE 没被判红: %+v", s)
	}
	// 破坏三：投稿侧被改成级联（要消掉预检、静默删内容资产的那种「修法」）。
	contribCascade := migrationFile{name: "000041_y.up.sql", content: `
ALTER TABLE user_contribution DROP CONSTRAINT user_contribution_credential_id_fkey;
ALTER TABLE user_contribution ADD CONSTRAINT user_contribution_credential_id_fkey
    FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;`}
	if s := checkCredentialFKShape([]migrationFile{up, contribCascade}); len(s.contributionCascad) != 1 {
		t.Fatalf("投稿侧被改成级联没被判红: %+v", s)
	}
	t.Log("三条破坏样本各判红一次（cascade=0 / plain=1 / contributionCascade=1）")
}

// flattenSQL 折叠空白，便于按单行子串比多行写法。
func flattenSQL(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\r\n", "\n")), " ")
}
