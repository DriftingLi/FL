// 证件删除的 Postgres 契约（#1360 / 真实缺陷 #12）——本票判据 1 与判据 3 的承重面。
//
// 为什么只能在 PG 上判：
//   - 判据 1「有练习进度时删证件成功」靠的是迁移 000040 的 ON DELETE CASCADE；
//   - 判据 3「测试库不再因不建外键而假绿」要的正是「外键真的在、动作真的是那两档」。
//
// SQLite 面上这两件事物理上不存在：testutil 的 SQLite 库由模型 AutoMigrate 建表，
// 模型 tag 表达不出 REFERENCES 的删除动作，删证件既不撞外键也不级联 ⇒「成功」是假的，
// 「投稿挡住」也只由服务层预检撑着。internal/api/credential_delete_contract_test.go 判的是
// HTTP 信封（400 / 404 / 成功文案，SQLite 面已足够），本文件判的是库形状。
//
// 本机无 DATABASE_URL 时 testutil.NewPostgresDB 会 t.Skip ⇒ 首跑在 CI（checks.md 的两条纪律）。
// 不依赖 PG 也能红的那半（迁移文本对账）在 internal/migrate/credential_fk_test.go。
package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
	"forklift-training/internal/training"
)

// credPKGFKey 两条 credential 外键的约束名（000013/000040 与 000020 的列级匿名 FK 命名惯例）。
const (
	practiceProgressCredFK = "practice_progress_credential_id_fkey"
	userContribCredFK      = "user_contribution_credential_id_fkey"
)

// deleteCredentialFKRow 一条 FK 的删除动作（confdeltype：c=CASCADE、a=NO ACTION）。
type deleteCredentialFKRow struct {
	Conname     string
	Confdeltype string
}

// actualCredentialFKActions 查当前 schema 里指向 credential 的外键及其删除动作。
//
// ⚠️ 必须按 current_schema() 收窄（checks.md 的硬纪律）：pg_constraint / pg_class 是全库视图，
// 多个契约包共用同一个测试库，没收窄就会读到别的 schema 的约束——那才是真的假绿。
func actualCredentialFKActions(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var rows []deleteCredentialFKRow
	if err := db.Table("pg_constraint AS c").
		Select("c.conname AS conname, c.confdeltype AS confdeltype").
		Joins("JOIN pg_class AS t ON t.oid = c.conrelid").
		Joins("JOIN pg_namespace AS n ON n.oid = t.relnamespace").
		Joins("JOIN pg_class AS rt ON rt.oid = c.confrelid").
		Where("c.contype = 'f' AND n.nspname = current_schema() AND rt.relname = 'credential'").
		Scan(&rows).Error; err != nil {
		t.Fatalf("查询 credential 外键失败: %v", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Conname] = r.Confdeltype
	}
	return out
}

// TestCredentialDeleteFKActionsOnPostgres 判据 3：真实迁移建出来的测试 schema 里，
// 两条证件外键必须**存在**且动作分别是 CASCADE / NO ACTION。
//
// 这一条就是「测试库不再因不建外键而假绿」的字面形态：
//   - 外键不在 ⇒ 本函数查不到行，判红（此前 SQLite 面永远查不到，缺陷因此不可见）；
//   - 投稿侧被改成 CASCADE ⇒ 判红（那会把「投稿阻塞」退化成静默删内容资产）。
func TestCredentialDeleteFKActionsOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置，跳过 Postgres 契约测试")
	}
	actions := actualCredentialFKActions(t, db)
	if got, ok := actions[practiceProgressCredFK]; !ok {
		t.Fatalf("%s 不存在——迁移 000040 没跑到，或测试库根本没建外键（正是真实缺陷 #14 的形状）", practiceProgressCredFK)
	} else if got != "c" {
		t.Fatalf("%s 的删除动作应为 c（ON DELETE CASCADE，练习分区随证件删除），实得 %q", practiceProgressCredFK, got)
	}
	if got, ok := actions[userContribCredFK]; !ok {
		t.Fatalf("%s 不存在——投稿侧外键缺失，预检挡的是一个并不存在的冲突", userContribCredFK)
	} else if got != "a" {
		t.Fatalf("%s 的删除动作应为 a（NO ACTION：投稿是内容资产，不随证件消失），实得 %q", userContribCredFK, got)
	}
	t.Logf("FK 动作：%s=c / %s=a", practiceProgressCredFK, userContribCredFK)
}

// TestCredentialDeleteCascadesPracticeProgressOnPostgres 判据 1 的 PG 面：
// 有练习进度（无投稿）时删证件成功，且**该证件的分区真的随证件消失**。
//
// 服务层不需要记得删进度行——分区消失是库层事实。同时判两张对照面：
//   - 另一证件的分区必须原样留着（级联范围越过本证件 = 数据事故）；
//   - 无证件（credential_id IS NULL）的兜底桶行必须留着（CASCADE 不该把它卷走，
//     uq_practice_progress_nocred 那格的存在意义就是「未分区」这一桶）。
func TestCredentialDeleteCascadesPracticeProgressOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置，跳过 Postgres 契约测试")
	}
	svc := training.NewService(db, zap.NewNop())

	doomed := seedCredForPGDelete(t, db, "N1_pg_cascade")
	keeper := seedCredForPGDelete(t, db, "N1_pg_keeper")
	student := seedStudent(t, db, "cred_del_pg", "x")

	// 被删证件下：同一学员两种模式各一行（分区里不止一行时才看得出级联是整段消失）
	seedPGProgress(t, db, student.ID, doomed.ID, "sequential")
	seedPGProgress(t, db, student.ID, doomed.ID, "random")
	// 另一证件的分区：必须活下来
	seedPGProgress(t, db, student.ID, keeper.ID, "sequential")
	// 「未分区」兜底桶（credential_id IS NULL）：uq_practice_progress_nocred 管的那一格，不该被卷走
	nullBucket := model.PracticeProgress{
		StudentID: student.ID, PracticeMode: "sequential", CredentialID: nil,
		QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: time.Now(),
	}
	if err := db.Create(&nullBucket).Error; err != nil {
		t.Fatalf("建未分区兜底行失败: %v", err)
	}

	if err := svc.DeleteCredential(doomed.ID); err != nil {
		t.Fatalf("只有练习进度时删证件应成功（原缺陷：FK 无删除动作 ⇒ 撞外键回 500），实得 %v", err)
	}
	if n := countPGProgress(t, db, doomed.ID); n != 0 {
		t.Fatalf("该证件的练习进度分区应随证件消失，实得 %d 行", n)
	}
	if n := countPGProgress(t, db, keeper.ID); n != 1 {
		t.Fatalf("另一证件的分区不得被级联卷走，实得 %d 行", n)
	}
	var nullLeft int64
	if err := db.Model(&model.PracticeProgress{}).
		Where("student_id = ? AND credential_id IS NULL", student.ID).Count(&nullLeft).Error; err != nil {
		t.Fatalf("数未分区兜底行失败: %v", err)
	}
	if nullLeft != 1 {
		t.Fatalf("credential_id IS NULL 的兜底桶行不该被级联卷走，实得 %d 行", nullLeft)
	}
	// 证件本身已删除（级联删的是子行，父行由 catalogDelete 自己删）
	var gone int64
	if err := db.Model(&model.Credential{}).Where("id = ?", doomed.ID).Count(&gone).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if gone != 0 {
		t.Fatalf("证件应已删除，实得 %d 行", gone)
	}
}

// TestCredentialDeleteBlockedByContributionsOnPostgres 判据 2 的 PG 面：
// 有投稿时预检回带条数的错误，且**库层外键确实会挡住**（证明预检挡的是真冲突，不是想象中的）。
//
// 后一半是本票「不做静默级联删投稿」的库层凭据：把预检摘掉，删除不会温柔地成功，
// 而是被 PG 以外键冲突判红 ⇒ 那条 500 正是本票要消掉的东西。
func TestCredentialDeleteBlockedByContributionsOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置，跳过 Postgres 契约测试")
	}
	svc := training.NewService(db, zap.NewNop())

	cred := seedCredForPGDelete(t, db, "N1_pg_contrib")
	student := seedStudent(t, db, "cred_del_pg_block", "x")
	seedPGContribution(t, db, student.ID, cred.ID, "pending")
	seedPGContribution(t, db, student.ID, cred.ID, "withdrawn")

	err := svc.DeleteCredential(cred.ID)
	if err == nil {
		t.Fatal("证件下仍有投稿时删除必须被拒")
	}
	if !errors.Is(err, training.ErrCredentialHasContributions) {
		t.Fatalf("应可被哨兵 ErrCredentialHasContributions 命中（api 侧据此落 400），实得 %v", err)
	}
	if !strings.Contains(err.Error(), "2 篇投稿") {
		t.Fatalf("文案必须带该证件下的投稿条数 2，实得 %q", err.Error())
	}

	// 预检确实没动到任何行
	var stillCred, stillContrib int64
	if err := db.Model(&model.Credential{}).Where("id = ?", cred.ID).Count(&stillCred).Error; err != nil {
		t.Fatalf("数证件失败: %v", err)
	}
	if err := db.Model(&model.UserContribution{}).Where("credential_id = ?", cred.ID).Count(&stillContrib).Error; err != nil {
		t.Fatalf("数投稿失败: %v", err)
	}
	if stillCred != 1 || stillContrib != 2 {
		t.Fatalf("被拒的删除不得动到任何行，实得 证件 %d / 投稿 %d", stillCred, stillContrib)
	}

	// 库层兜底：绕过预检直接删父行 ⇒ 外键必须判红（NO ACTION 真的在生效）
	raw := db.Exec("DELETE FROM credential WHERE id = ?", cred.ID)
	if raw.Error == nil {
		t.Fatal("投稿侧外键保持 NO ACTION：库层必须挡住这条删除。挡不住 = 预检在保护一个并不存在的冲突")
	}
	if !strings.Contains(raw.Error.Error(), "foreign key constraint") {
		t.Fatalf("应以外键冲突被拒，实得 %v", raw.Error)
	}
}

// seedCredForPGDelete 建一枚证件（编码唯一）。
func seedCredForPGDelete(t *testing.T, db *gorm.DB, code string) *model.Credential {
	t.Helper()
	c := &model.Credential{Code: code, Name: "证件-" + code, Category: "special_operation", Status: 1}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	return c
}

// seedPGProgress 给某学员在某证件下建一行练习进度（credential_id 是指针列，取值才能表达分区）。
func seedPGProgress(t *testing.T, db *gorm.DB, studentID, credID int, mode string) {
	t.Helper()
	cid := credID
	p := model.PracticeProgress{
		StudentID: studentID, PracticeMode: mode, CredentialID: &cid,
		QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: time.Now(),
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("建练习进度失败: %v", err)
	}
}

// seedPGContribution 在某证件下建一篇指定状态的投稿。
func seedPGContribution(t *testing.T, db *gorm.DB, userID, credID int, status string) {
	t.Helper()
	row := model.UserContribution{
		UserID: userID, CredentialID: credID,
		Title: fmt.Sprintf("PG 投稿-%d-%s", credID, status), Intro: "夹具", Status: status,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("建投稿失败: %v", err)
	}
}

// countPGProgress 数某证件分区下的练习进度行数。
//
// 计数走模型（GORM 带上 search_path 里那张 practice_progress），不裸查 pg 目录——
// 这里要的是「我的分区里还剩几行」，不是全库视图。
func countPGProgress(t *testing.T, db *gorm.DB, credID int) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.PracticeProgress{}).Where("credential_id = ?", credID).Count(&n).Error; err != nil {
		t.Fatalf("数练习进度失败: %v", err)
	}
	return n
}
