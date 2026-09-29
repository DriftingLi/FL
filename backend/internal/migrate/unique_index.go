// 关键唯一索引登记与对账（#1362 / spec #1345 决策总表 #14，「真实缺陷 #14」）。
//
// 为什么需要一张登记表 + 一把逐字锁（与 columns.go 的单向列对账并列的第二半）：
//   - 生产 schema 里这些唯一约束的事实源是 migrations/*.up.sql 的 CREATE UNIQUE INDEX，
//     其中多数是**偏索引**（带 WHERE）——GORM tag 表达不了 WHERE，模型侧根本映射不出来；
//   - SQLite 测试库走 AutoMigrate 建表，物理上**没有**这些索引 ⇒ 「并发建号应被唯一索引兜底」
//     这类用例靠「测试库没有约束」通过，是假绿（真实缺陷 #14 的原文）。
//     所以本文件同时是两侧的唯一宿主：testutil 建库后逐条补跑登记表里的 DDL（与迁移同句），
//     cmd/migrate check-columns 则拿登记表去对真实库的 pg_indexes。
//
// 三处对账的分工（判据「删掉任一索引时能判红」两头都要有牙齿）：
//  1. 登记表 ⇔ migrations/*.up.sql 逐字相等：不连库、本机就能红（删/改任一句的一侧即红），
//     这条锁掉「迁移改了、测试库没跟着改」与反向漂移；
//  2. 登记表 ⇒ 真实库唯一索引存在性（check-columns 的新增判据）：迁移里那句没真建出来
//     （被删掉、被改名、挂错表）即在 CI 的 migration-check 里红；
//  3. testutil 建库后补跑 DDL：约束在 SQLite 测试面真实存在，兜底分支走得到。
//
// 口径纪律（沿用 columns.go，不做反转）：
//   - **单向**：库里多出来的索引（非唯一索引、模型刻意不映射的历史索引）不算错，
//     本文件只把「登记表要求存在」的那批唯一索引钉住，旧列对账语义一行不动；
//   - **目录查询必须按 schema 收窄**：pg_indexes 是全库视图，`go test ./...` 并发跑多个包
//     共用同一个测试库、各自建随机 schema（见 checks.md 与 #1197 的首跑教训）；
//   - **逐字相等按「空白归一」判定**：换行/缩进差不判，但索引名、列、表、WHERE 谓词、
//     IF NOT EXISTS 任一改一个 token 就红——那是会改变约束语义的全部自由度。
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// UniqueIndexSpec 一条关键唯一索引登记。
type UniqueIndexSpec struct {
	Name      string // 索引名（与迁移逐字相同）
	Table     string // 宿主表
	Migration string // 出处迁移文件名（报错定位用）
	DDL       string // 空白归一后的完整语句（不含结尾分号），可直接 Exec
}

// criticalUniqueIndexes 登记表：migrations 里全部 CREATE UNIQUE INDEX（偏索引 + 复合唯一索引）。
// DDL 逐字抄自出处迁移文件；新增偏唯一索引时必须在这里加一行，
// 否则 TestCriticalUniqueIndexesMatchMigrations 会以「迁移有、登记表没有」判红。
var criticalUniqueIndexes = []UniqueIndexSpec{
	// ===== 000001_baseline.up.sql =====
	{Name: "idx_hrwai_users_email_unique", Table: "hrwai_users", Migration: "000001_baseline.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_hrwai_users_email_unique ON hrwai_users (email) WHERE email <> ''"},
	{Name: "idx_hrwai_users_wechat_openid_unique", Table: "hrwai_users", Migration: "000001_baseline.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_hrwai_users_wechat_openid_unique ON hrwai_users (wechat_openid) WHERE wechat_openid <> ''"},
	{Name: "idx_hrwai_users_phone_unique", Table: "hrwai_users", Migration: "000001_baseline.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_hrwai_users_phone_unique ON hrwai_users (phone) WHERE phone <> ''"},
	{Name: "idx_wrong_question_student_question", Table: "wrong_question", Migration: "000001_baseline.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_wrong_question_student_question ON wrong_question (student_id, question_id)"},
	// ===== 000002_points_system.up.sql（积分领取幂等占坑的两臂）=====
	{Name: "uq_points_task_claim_daily", Table: "points_task_claim", Migration: "000002_points_system.up.sql",
		DDL: "CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_daily ON points_task_claim(user_id, task_code, claim_date) WHERE claim_date IS NOT NULL"},
	{Name: "uq_points_task_claim_ref", Table: "points_task_claim", Migration: "000002_points_system.up.sql",
		DDL: "CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_ref ON points_task_claim(user_id, task_code, ref_id) WHERE ref_id IS NOT NULL"},
	// ===== 000010_contact_requests.up.sql =====
	{Name: "idx_contact_requests_pending_unique", Table: "contact_requests", Migration: "000010_contact_requests.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_contact_requests_pending_unique ON contact_requests (recruiter_id, student_user_id) WHERE status = 'pending'"},
	// ===== 000013_practice_progress_credential.up.sql =====
	{Name: "uq_practice_progress_cred", Table: "practice_progress", Migration: "000013_practice_progress_credential.up.sql",
		DDL: "CREATE UNIQUE INDEX uq_practice_progress_cred ON practice_progress (student_id, practice_mode, credential_id) WHERE credential_id IS NOT NULL"},
	{Name: "uq_practice_progress_nocred", Table: "practice_progress", Migration: "000013_practice_progress_credential.up.sql",
		DDL: "CREATE UNIQUE INDEX uq_practice_progress_nocred ON practice_progress (student_id, practice_mode) WHERE credential_id IS NULL"},
	// ===== 000014_recruiter_credit_code_unique.up.sql =====
	{Name: "idx_recruiter_users_credit_code", Table: "recruiter_users", Migration: "000014_recruiter_credit_code_unique.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_recruiter_users_credit_code ON recruiter_users (credit_code)"},
	// ===== 000015_job_postings.up.sql =====
	{Name: "idx_job_applications_applied_unique", Table: "job_applications", Migration: "000015_job_postings.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_job_applications_applied_unique ON job_applications (job_posting_id, student_user_id) WHERE status = 'applied'"},
	{Name: "idx_job_reports_student_job_unique", Table: "job_reports", Migration: "000015_job_postings.up.sql",
		DDL: "CREATE UNIQUE INDEX idx_job_reports_student_job_unique ON job_reports (job_posting_id, student_user_id)"},
}

// CriticalUniqueIndexes 返回登记表副本（testutil 建库后补跑、check-columns 对账共用同一宿主）。
func CriticalUniqueIndexes() []UniqueIndexSpec {
	out := make([]UniqueIndexSpec, len(criticalUniqueIndexes))
	copy(out, criticalUniqueIndexes)
	return out
}

// ===== migrations 侧解析 =====

// createUniqueIndexRe 抓一条 CREATE UNIQUE INDEX 语句：索引名 + 到第一个分号为止的正文。
// (?s) 让正文跨行（000013 的两句就是三行写法）。局限（写在注释里而不是藏着）：
// 正文里的分号若出现在字符串字面量中会被截断——当前 migrations 无此形态。
var createUniqueIndexRe = regexp.MustCompile(`(?is)CREATE\s+UNIQUE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z0-9_."]+)(.*?);`)

// indexOnRe 从语句正文里取 ON <table>。
var indexOnRe = regexp.MustCompile(`(?is)\bON\s+([A-Za-z0-9_."]+)`)

// NormalizeIndexDDL 空白归一并去掉结尾分号：两侧（登记表与迁移解析结果）都过这里再比。
func NormalizeIndexDDL(ddl string) string {
	s := strings.TrimSpace(ddl)
	s = strings.TrimSuffix(s, ";")
	return strings.Join(strings.Fields(s), " ")
}

// stripSQLLineComments 去掉行注释（`--` 到行尾），单引号字符串内部不算注释起点。
// 解析器不能连注释一起吞：迁移文件里「-- 使用部分唯一索引避免 NULL 冲突」这类说明
// 恰好含「部分唯一索引」的同义词，但真正的危险是注释里写着的示例 DDL 会被数成一条约束。
func stripSQLLineComments(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	inString := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inString {
			// PG 的转义是 ''，逐字符照抄即可（引号状态只在真正的 ' 上翻转）。
			if c == '\'' {
				inString = false
				if i+1 < len(text) && text[i+1] == '\'' {
					b.WriteByte(c)
					i++
				}
			}
			b.WriteByte(c)
			continue
		}
		switch {
		case c == '\'':
			inString = true
			b.WriteByte(c)
		case c == '-' && i+1 < len(text) && text[i+1] == '-':
			for i < len(text) && text[i] != '\n' {
				i++
			}
			if i < len(text) {
				b.WriteByte('\n')
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// UniqueIndexesFromSQL 从一段 SQL 文本解析全部 CREATE UNIQUE INDEX（已归一化）。
func UniqueIndexesFromSQL(text string) []UniqueIndexSpec {
	body := stripSQLLineComments(strings.ReplaceAll(text, "\r\n", "\n"))
	var out []UniqueIndexSpec
	for _, m := range createUniqueIndexRe.FindAllStringSubmatch(body, -1) {
		// 用整段命中（m[0]）而不是「CREATE UNIQUE INDEX + 名字 + 正文」重建：
		// 重建会把 IF NOT EXISTS 抹掉（它落在非捕获组里），而那一截正是逐字对账要盯的形状。
		ddl := NormalizeIndexDDL(m[0])
		name := strings.Trim(m[1], `"`)
		table := ""
		if on := indexOnRe.FindStringSubmatch(m[2]); on != nil {
			table = strings.Trim(on[1], `"`)
		}
		out = append(out, UniqueIndexSpec{Name: name, Table: table, DDL: ddl})
	}
	return out
}

// UniqueIndexesFromMigrationsDir 扫描 migrations 目录下全部 *.up.sql 并解析唯一索引。
// 一条都没解析出来即报错（fail-closed：解析器失灵比缺一条索引更早发生，不能让它判绿）。
func UniqueIndexesFromMigrationsDir(dir string) ([]UniqueIndexSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读 migrations 目录失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out []UniqueIndexSpec
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("读迁移 %s 失败: %w", name, err)
		}
		for _, spec := range UniqueIndexesFromSQL(string(data)) {
			spec.Migration = name
			out = append(out, spec)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("migrations 目录 %s 未解析到任何 CREATE UNIQUE INDEX——解析器失守，拒绝在空集合上判「对账通过」", dir)
	}
	return out, nil
}

// ===== 对账一：登记表 ⇔ migrations 逐字相等 =====

// UniqueIndexRegistryDiff 登记表与迁移的双向差集。三个桶都判红，分开报才定位得动。
type UniqueIndexRegistryDiff struct {
	MissingInMigrations []string // 登记表要求、迁移里已找不到该索引名（被删/被改名）
	MissingInRegistry   []string // 迁移里有、登记表没登记（新索引没同步给测试库与对账）
	Mismatched          []string // 名字两侧都在，但语句或宿主表逐字不等
}

// OK 报告两侧是否逐字一致。
func (d UniqueIndexRegistryDiff) OK() bool {
	return len(d.MissingInMigrations) == 0 && len(d.MissingInRegistry) == 0 && len(d.Mismatched) == 0
}

// Error 汇总成一条错误（CheckColumns 直接用）。
func (d UniqueIndexRegistryDiff) Error() error {
	if d.OK() {
		return nil
	}
	lines := append(append(append([]string{}, d.MissingInMigrations...), d.MissingInRegistry...), d.Mismatched...)
	return fmt.Errorf("唯一索引登记表与 migrations 逐字对账失败（%d 项）：%s", len(lines), strings.Join(lines, "；"))
}

// DiffUniqueIndexRegistry 双向逐字对账登记表与迁移解析结果。
func DiffUniqueIndexRegistry(registry, migrations []UniqueIndexSpec) UniqueIndexRegistryDiff {
	var diff UniqueIndexRegistryDiff
	mig := make(map[string]UniqueIndexSpec, len(migrations))
	for _, m := range migrations {
		mig[m.Name] = m
	}
	seen := make(map[string]bool, len(registry))
	for _, r := range registry {
		seen[r.Name] = true
		m, ok := mig[r.Name]
		if !ok {
			diff.MissingInMigrations = append(diff.MissingInMigrations,
				fmt.Sprintf("%s：登记表要求（出处 %s），migrations 里没有这条 CREATE UNIQUE INDEX", r.Name, r.Migration))
			continue
		}
		if r.DDL != m.DDL {
			diff.Mismatched = append(diff.Mismatched,
				fmt.Sprintf("%s：登记表「%s」与迁移 %s 的「%s」逐字不等", r.Name, r.DDL, m.Migration, m.DDL))
		}
		if r.Table != m.Table {
			diff.Mismatched = append(diff.Mismatched, fmt.Sprintf("%s：宿主表登记为 %s，迁移写的是 %s", r.Name, r.Table, m.Table))
		}
	}
	for _, m := range migrations {
		if !seen[m.Name] {
			diff.MissingInRegistry = append(diff.MissingInRegistry,
				fmt.Sprintf("%s：迁移 %s 建了它，登记表没有 ⇒ 测试库不会补、对账也不会管", m.Name, m.Migration))
		}
	}
	sort.Strings(diff.MissingInMigrations)
	sort.Strings(diff.MissingInRegistry)
	sort.Strings(diff.Mismatched)
	return diff
}

// CheckUniqueIndexRegistry 连不上库也能跑的那半：登记表 ⇔ migrations 目录逐字相等。
func CheckUniqueIndexRegistry(logger *zap.Logger) error {
	dir, err := resolveMigrationsDir(logger)
	if err != nil {
		return fmt.Errorf("定位 migrations 目录失败: %w", err)
	}
	migrations, err := UniqueIndexesFromMigrationsDir(dir)
	if err != nil {
		return err
	}
	diff := DiffUniqueIndexRegistry(CriticalUniqueIndexes(), migrations)
	if !diff.OK() {
		for _, line := range diff.MissingInMigrations {
			logger.Error("唯一索引对账失败：登记表要求、迁移没有", zap.String("项", line))
		}
		for _, line := range diff.MissingInRegistry {
			logger.Error("唯一索引对账失败：迁移有、登记表没登记", zap.String("项", line))
		}
		for _, line := range diff.Mismatched {
			logger.Error("唯一索引对账失败：两侧逐字不等", zap.String("项", line))
		}
		return diff.Error()
	}
	logger.Info("唯一索引登记表与迁移逐字对账通过", zap.Int("条数", len(migrations)))
	return nil
}

// ===== 对账二：登记表 ⇒ 真实库唯一索引存在性（check-columns 新增判据） =====

// pgIndexesSQL 生产口径的实际唯一索引查询：只读元数据。
// schemaname = current_schema() 与列对账同一条纪律（pg_indexes 是全库视图，不收窄会数到
// 别的包那份同名对象）；索引名在 schema 内唯一，故不需要再去 join pg_class。
const pgIndexesSQL = `SELECT tablename, indexname
FROM pg_indexes
WHERE schemaname = current_schema()`

// ActualUniqueIndexes 读取实际库的索引集合，键为「表.索引名」。query 可替换，便于单测用
// 内存库跑真实自省（对账的「实际」侧仍来自数据库，而不是测试伪造的 map）。
func ActualUniqueIndexes(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	return actualIndexes(ctx, db, pgIndexesSQL)
}

func actualIndexes(ctx context.Context, db *sql.DB, query string) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询实际索引失败: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]struct{}{}
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			return nil, fmt.Errorf("读取实际索引失败: %w", err)
		}
		out[table+"."+name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历实际索引失败: %w", err)
	}
	return out, nil
}

// DiffUniqueIndexes 单向存在性对账：登记表要求的「表.索引名」必须在实际库里都在。
// 实际库多出来的索引不算错（与列对账同口径，不反转旧判据）。
func DiffUniqueIndexes(expected []UniqueIndexSpec, actual map[string]struct{}) []string {
	var missing []string
	for _, e := range expected {
		if _, ok := actual[e.Table+"."+e.Name]; !ok {
			missing = append(missing, e.Table+"."+e.Name)
		}
	}
	sort.Strings(missing)
	return missing
}

// checkUniqueIndexPresence 连库那半：真实库必须把登记表的索引都建出来。
func checkUniqueIndexPresence(ctx context.Context, db *sql.DB, logger *zap.Logger) error {
	expected := CriticalUniqueIndexes()
	if len(expected) == 0 {
		return fmt.Errorf("唯一索引登记表为空：拒绝在空集合上判「对账通过」")
	}
	actual, err := ActualUniqueIndexes(ctx, db)
	if err != nil {
		return err
	}
	missing := DiffUniqueIndexes(expected, actual)
	if len(missing) > 0 {
		for _, item := range missing {
			logger.Error("唯一索引对账失败：登记表要求、实际库没有", zap.String("表.索引", item))
		}
		return fmt.Errorf("唯一索引存在性对账失败：实际库缺 %d/%d 条（migrations 未把它们建出来，或索引被改名/挂错表）",
			len(missing), len(expected))
	}
	logger.Info("唯一索引存在性对账通过", zap.Int("条数", len(expected)))
	return nil
}
