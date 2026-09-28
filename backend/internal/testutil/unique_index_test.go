// 测试库的关键唯一索引（#1362，spec #1345 决策 #14）。
//
// 本文件锁的是「SQLite 测试库里那些偏唯一索引真的存在、真的会咬人」——
// 真实缺陷 #14 的内容正是：AutoMigrate 建不出偏索引（GORM tag 没有 WHERE 这格），
// 于是「并发建号应被唯一索引兜底」这类用例靠「测试库没有约束」通过。
// 现在建库后补跑与 migrations 逐字同句的 DDL（宿主在 internal/migrate），
// 这里逐条验证：约束在 sqlite_master 里数得到、违反约束的插入真的报错、
// 偏索引的「不参与判重」那一半也照旧成立（空手机号 / NULL 证件可以并存）。
package testutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/migrate"
	"forklift-training/internal/model"
)

// TestMemoryDBCarriesCriticalUniqueIndexes 建库后 sqlite_master 里必须数得到登记表的每一条索引。
// 这条断言不依赖 PG，是 #1362 在本机的第一道锁（登记表少一条 / Exec 静默失败都会红）。
func TestMemoryDBCarriesCriticalUniqueIndexes(t *testing.T) {
	db := NewMemoryDB(t)
	assertIndexesInSQLiteMaster(t, db)
}

// TestFileDBCarriesCriticalUniqueIndexes 同上，文件库（并发用例的载体）也须补齐。
func TestFileDBCarriesCriticalUniqueIndexes(t *testing.T) {
	db := NewFileDB(t)
	assertIndexesInSQLiteMaster(t, db)
}

func assertIndexesInSQLiteMaster(t *testing.T, db *gorm.DB) {
	t.Helper()
	registry := migrate.CriticalUniqueIndexes()
	if len(registry) == 0 {
		t.Fatal("登记表为空：测试库不会补任何约束，本文件会集体假绿")
	}
	var names []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'index'`).Scan(&names).Error; err != nil {
		t.Fatalf("查 sqlite_master 失败: %v", err)
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	var missing []string
	for _, idx := range registry {
		if !set[idx.Name] {
			missing = append(missing, idx.Table+"."+idx.Name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("测试库缺关键唯一索引 %d/%d：%s", len(missing), len(registry), strings.Join(missing, ", "))
	}
	t.Logf("测试库已补齐关键唯一索引 %d 条", len(registry))
}

// TestCriticalUniqueIndexesBite 逐条约束各一正一负：违反唯一性被拒、偏索引放行的一侧可并存。
// 逐条点名而不是抽一条：真实缺陷是「某几条没建出来」，抽样会漏。
func TestCriticalUniqueIndexesBite(t *testing.T) {
	db := NewMemoryDB(t)
	cred := 7
	otherCred := 8
	now := Now()

	cases := []struct {
		name  string
		table string
		// base 给 hrwai_users 那几族错开 id/账号/昵称（模型上的 account/username 也是唯一索引，
		// 三族共用一个基数的话夹具会先撞在它们身上，判不到本文件要判的那条偏索引）。
		base int
		// dup 造违反唯一性的两次插入；second 是第二条（应与第一条冲突）。
		first  func(i int) any
		second func(i int) any
		// partial 是与「不参与判重」那一侧同形的插入（空串 / NULL），必须放行。
		partial func(i int) any
	}{
		{
			name: "hrwai_users.phone 偏唯一", table: "hrwai_users", base: 1,
			first:   func(i int) any { return phoneUser(i, "13800000001") },
			second:  func(i int) any { return phoneUser(i, "13800000001") },
			partial: func(i int) any { return phoneUser(i, "") },
		},
		{
			name: "hrwai_users.email 偏唯一", table: "hrwai_users", base: 11,
			first:   func(i int) any { return emailUser(i, "dup@example.com") },
			second:  func(i int) any { return emailUser(i, "dup@example.com") },
			partial: func(i int) any { return emailUser(i, "") },
		},
		{
			name: "hrwai_users.wechat_openid 偏唯一", table: "hrwai_users", base: 21,
			first:   func(i int) any { return openidUser(i, "openid-dup") },
			second:  func(i int) any { return openidUser(i, "openid-dup") },
			partial: func(i int) any { return openidUser(i, "") },
		},
		{
			name: "wrong_question (student, question) 唯一", table: "wrong_question",
			first:  func(i int) any { return &model.WrongQuestion{StudentID: 31, QuestionID: 41, CreatedAt: now} },
			second: func(i int) any { return &model.WrongQuestion{ID: 500, StudentID: 31, QuestionID: 41, CreatedAt: now} },
		},
		{
			name: "points_task_claim 每日臂偏唯一", table: "points_task_claim",
			first: func(i int) any {
				d := "2026-01-02"
				return &model.PointsTaskClaim{UserID: 61, TaskCode: "daily_x", ClaimDate: &d, RefID: strPtr(fmt.Sprintf("ref-a-%d", i))}
			},
			second: func(i int) any {
				d := "2026-01-02"
				return &model.PointsTaskClaim{UserID: 61, TaskCode: "daily_x", ClaimDate: &d, RefID: strPtr("ref-b")}
			},
			// ref_id 为 NULL 的行落不进每日臂的另一侧（终身臂谓词）——同 (user, code) 无 claim_date 才可并存。
		},
		{
			name: "points_task_claim 终身臂（ref_id）偏唯一", table: "points_task_claim",
			first: func(i int) any {
				r := "lifetime-1"
				return &model.PointsTaskClaim{UserID: 62, TaskCode: "life_x", RefID: &r}
			},
			second: func(i int) any {
				r := "lifetime-1"
				return &model.PointsTaskClaim{UserID: 62, TaskCode: "life_x", RefID: &r}
			},
			// claim_date 为 NULL ⇒ 每日臂不判重；两行都只有 ref_id 相同才冲突，这里正是冲突侧。
			partial: func(i int) any {
				r := fmt.Sprintf("lifetime-null-%d", i)
				return &model.PointsTaskClaim{UserID: 62, TaskCode: "life_x", RefID: &r}
			},
		},
		{
			name: "contact_requests pending 偏唯一", table: "contact_requests",
			first:  func(i int) any { return contactRequest(91, 21, "pending") },
			second: func(i int) any { return contactRequest(91, 21, "pending") },
			// 已裁决的一对可以并存（status 不再是 pending ⇒ 偏索引不管）。
			partial: func(i int) any { return contactRequest(93, 23, "approved") },
		},
		{
			name: "job_applications applied 偏唯一", table: "job_applications",
			first:   func(i int) any { return jobApplication(11, 12, "applied") },
			second:  func(i int) any { return jobApplication(11, 12, "applied") },
			partial: func(i int) any { return jobApplication(14, 15, "withdrawn") },
		},
		{
			name: "job_reports (job, student) 唯一", table: "job_reports",
			first:  func(i int) any { return jobReport(16, 17) },
			second: func(i int) any { return jobReport(16, 17) },
		},
		{
			name: "practice_progress 证件分区偏唯一", table: "practice_progress",
			first: func(i int) any {
				c := cred
				return &model.PracticeProgress{StudentID: 51, PracticeMode: "sequential", CredentialID: &c, UpdatedAt: now}
			},
			second: func(i int) any {
				c := cred
				return &model.PracticeProgress{ID: 777, StudentID: 51, PracticeMode: "sequential", CredentialID: &c, UpdatedAt: now}
			},
			// 换证件 = 另一分区（uq_practice_progress_cred 的列集含 credential_id），必须放行。
			partial: func(i int) any {
				c := otherCred
				return &model.PracticeProgress{StudentID: 51, PracticeMode: "sequential", CredentialID: &c, UpdatedAt: now}
			},
		},
		{
			name: "practice_progress 未分区兜底偏唯一", table: "practice_progress",
			first: func(i int) any {
				return &model.PracticeProgress{StudentID: 52, PracticeMode: "tag", CredentialID: nil, UpdatedAt: now}
			},
			second: func(i int) any {
				return &model.PracticeProgress{ID: 888, StudentID: 52, PracticeMode: "tag", CredentialID: nil, UpdatedAt: now}
			},
			// 不同模式不冲突（NULL 侧谓词只管 credential_id IS NULL）。
			partial: func(i int) any {
				return &model.PracticeProgress{StudentID: 52, PracticeMode: "paper", CredentialID: nil, UpdatedAt: now}
			},
		},
		{
			name: "recruiter_users.credit_code 唯一", table: "recruiter_users",
			first:  func(i int) any { return recruiterUser(i, "91310000DUP") },
			second: func(i int) any { return recruiterUser(100+i, "91310000DUP") },
		},
	}

	for _, tc := range cases {
		if err := db.Create(tc.first(tc.base)).Error; err != nil {
			t.Fatalf("%s：第一条插入就失败（夹具问题）：%v", tc.name, err)
		}
		err := db.Create(tc.second(tc.base + 1)).Error
		if err == nil {
			t.Fatalf("%s：重复插入未被唯一索引拒绝——测试库没建出这条约束（#1362 要防的假绿）", tc.name)
		}
		if !isUniqueViolation(err) {
			t.Fatalf("%s：报错了但不是唯一约束冲突，判据没锁到点上：%v", tc.name, err)
		}
		if tc.partial != nil {
			if err := db.Create(tc.partial(tc.base + 2)).Error; err != nil {
				t.Fatalf("%s：偏索引「不参与判重」的一侧被误拦：%v", tc.name, err)
			}
		}
		t.Logf("%s（表 %s）✔ 违反侧被拒 / 放行侧可并存", tc.name, tc.table)
	}
}

// isUniqueViolation 判定错误来自唯一约束（SQLite 与 PG 的措辞都收进来，两侧同一判据）。
func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}

// TestConcurrentAccountCreationHitsUniqueIndex 判据 1 的 DB 层承载：并发建号（同手机号）
// 恰有一笔成功。改造前测试库没有 phone 偏唯一索引 ⇒ N 笔全部成功 ⇒ 服务层的
// 「数据库唯一索引兜底」分支在测试面根本走不到（真实缺陷 #14 的原文）。
func TestConcurrentAccountCreationHitsUniqueIndex(t *testing.T) {
	db := NewFileDB(t) // :memory: 每连接独立库，并发场景须文件库
	const attempts = 6
	const phone = "13900000001"

	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- db.Create(phoneUser(1000+i*7, phone)).Error
		}(i)
	}
	wg.Wait()
	close(errs)

	ok, violated := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case isUniqueViolation(err):
			violated++
		default:
			t.Fatalf("并发建号出现了唯一冲突以外的错误: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("并发建号应恰有一笔成功（唯一索引兜底），实际成功 %d 笔、撞约束 %d 笔", ok, violated)
	}
	if violated != attempts-1 {
		t.Fatalf("其余 %d 笔都应撞到唯一索引，实际 %d 笔", attempts-1, violated)
	}
	var cnt int64
	if err := db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&cnt).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("同手机号应只有一行，实际 %d 行", cnt)
	}
}

// ===== 夹具 =====

func strPtr(s string) *string { return &s }

func phoneUser(id int, phone string) *model.HrwaiUser {
	return &model.HrwaiUser{ID: id, UID: int64(9_000_000_000 + id), Account: fmt.Sprintf("acct_%d", id),
		Username: fmt.Sprintf("u_%d", id), Password: "x", Phone: phone, Status: 1, CreatedAt: Now()}
}

func emailUser(id int, email string) *model.HrwaiUser {
	u := phoneUser(id, "")
	u.Email = email
	return u
}

func openidUser(id int, openid string) *model.HrwaiUser {
	u := phoneUser(id, "")
	u.WechatOpenID = openid
	return u
}

func contactRequest(recruiterID, studentID int, status string) *model.ContactRequest {
	return &model.ContactRequest{
		RecruiterID: recruiterID, StudentUserID: studentID, Message: "夹具", Status: status,
		CreatedAt: Now(), UpdatedAt: Now(), ExpiresAt: timePtr(Now().Add(14 * 24 * time.Hour)),
	}
}

func jobApplication(jobPostingID, studentID int, status string) *model.JobApplication {
	return &model.JobApplication{
		JobPostingID: jobPostingID, RecruiterID: 1, StudentUserID: studentID, Status: status,
		ResumeUpdatedAt: Now(), CreatedAt: Now(), UpdatedAt: Now(),
	}
}

func jobReport(jobPostingID, studentID int) *model.JobReport {
	return &model.JobReport{JobPostingID: jobPostingID, StudentUserID: studentID, Reason: "夹具",
		CreatedAt: Now(), UpdatedAt: Now()}
}

func recruiterUser(id int, creditCode string) *model.RecruiterUser {
	return &model.RecruiterUser{
		Username: fmt.Sprintf("rec_%d", id), Password: "x", CompanyName: "夹具企业",
		CreditCode: creditCode, BusinessScope: "维修", ContactName: "张三",
		ContactPhone: "13800000000", ContactEmail: "rec@example.com", Status: 1,
		CreatedAt: Now(), UpdatedAt: Now(),
	}
}

func timePtr(t time.Time) *time.Time { return &t }
