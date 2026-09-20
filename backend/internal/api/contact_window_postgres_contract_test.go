// 裁决窗口的库层契约（ADR-0061 §2 / #1197，Postgres adapter）。
//
// 为什么必须用 Postgres：`contact_requests` 的 pending 唯一**有两个源**——应用层计数
// （ContactService.Create）与迁移 000010:16 的偏索引 `WHERE status = 'pending'`。
// SQLite 测试库由 AutoMigrate 建表、不执行 migrations/，那条索引与 000039 的 CHECK
// 在 SQLite 里物理上不存在 ⇒ 应用层放宽后「库层还挡不挡」这件事只有真 PG 能回答
// （同 ADR-0044 的 forum CHECK 口径，测试基建 testutil.NewPostgresDB 跑真实迁移）。
package api

import (
	"strings"
	"sync"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestContactDecisionWindowOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	svc := newContractDeps(t, db, nil).ContactSvc

	recruiter := model.RecruiterUser{
		Username: "win_rec", Password: "x", CompanyName: "窗口测试企业",
		CreditCode: "win_credit", BusinessScope: "维修", ContactName: "张三",
		ContactPhone: "13800000000", ContactEmail: "win@example.com", Status: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&recruiter).Error; err != nil {
		t.Fatalf("seed recruiter: %v", err)
	}
	student := testutil.SeedStudent(t, db, "win_stu", "x")
	other := testutil.SeedStudent(t, db, "win_stu_other", "x")

	// 1. 库层形状：偏索引仍在（唯一性仍由库兜），000039 的 CHECK 已生效。
	var indexDef string
	if err := db.Raw(`SELECT indexdef FROM pg_indexes
		WHERE tablename = 'contact_requests' AND indexname = 'idx_contact_requests_pending_unique'`).
		Scan(&indexDef).Error; err != nil {
		t.Fatalf("查偏索引: %v", err)
	}
	if !strings.Contains(indexDef, "WHERE") {
		t.Fatalf("pending 偏索引不再是偏索引（应用层放宽后唯一性就没了兜底）：%s", indexDef)
	}
	var chkCount int64
	if err := db.Raw(`SELECT count(*) FROM pg_constraint WHERE conname = 'chk_contact_requests_pending_window'`).
		Scan(&chkCount).Error; err != nil {
		t.Fatalf("查 CHECK: %v", err)
	}
	if chkCount != 1 {
		t.Fatalf("迁移 000039 的 CHECK 不存在（%d 行）", chkCount)
	}

	// 2. CHECK 的两面：pending 无窗口必须被拒；approved 无窗口必须可写。
	if err := db.Exec(`INSERT INTO contact_requests
		(recruiter_id, student_user_id, message, status, created_at, updated_at, expires_at)
		VALUES (?, ?, '无窗口的 pending', 'pending', NOW(), NOW(), NULL)`,
		recruiter.ID, student.ID).Error; err == nil {
		t.Fatalf("库层 CHECK 未拦住无窗口的 pending——「pending 必有窗口」又退回应用层单点")
	}
	if err := db.Exec(`INSERT INTO contact_requests
		(recruiter_id, student_user_id, message, status, created_at, updated_at, expires_at)
		VALUES (?, ?, '无窗口的 approved', 'approved', NOW(), NOW(), NULL)`,
		recruiter.ID, other.ID).Error; err != nil {
		t.Fatalf("approved 应允许无窗口（永久授权，ADR-0061 §2）: %v", err)
	}

	// 3. 并发双发：恰好一个成功，另一个拿到的是业务文案而不是驱动原文。
	// 结果与调度无关：对手先提交则本方撞计数分支，同时通过计数则撞偏索引后回读确认——
	// 两条出口同一句文案，且「那条 pending 确实存在」为真。
	var (
		gate    sync.WaitGroup
		done    sync.WaitGroup
		errs    = make([]error, 2)
		windows = make([]string, 2)
	)
	gate.Add(2)
	done.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer done.Done()
			gate.Done()
			gate.Wait()
			dto, err := svc.Create(recruiter.ID, student.ID, "并发重发")
			errs[i] = err
			if err == nil && dto != nil {
				windows[i] = derefString(dto.ExpiresAt)
			}
		}(i)
	}
	done.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		if !strings.Contains(err.Error(), "已存在待处理的申请") {
			t.Fatalf("并发落败方应是业务文案，实际 %q", err.Error())
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "idx_contact_requests") {
			t.Fatalf("驱动原文泄到了对外文案里：%q", err.Error())
		}
	}
	if ok != 1 {
		t.Fatalf("并发双发应恰好一条成功，实际 %d（errs=%v）", ok, errs)
	}
	// pending 必带窗口：成功那条的 DTO 必须输出 expires_at（ADR-0061 §2 的正向半边）。
	if windows[0] == "" && windows[1] == "" {
		t.Fatalf("成功的那条 pending 必须带裁决窗口，DTO 却两处都缺失")
	}

	// 4. expired 不占偏索引：窗口关闭后同一对可立即再挂一条 pending（§2 的「不挡重发」在库层成立）。
	if err := db.Model(&model.ContactRequest{}).
		Where("recruiter_id = ? AND student_user_id = ?", recruiter.ID, student.ID).
		Update("status", string(service.ContactGrantExpired)).Error; err != nil {
		t.Fatalf("置 expired: %v", err)
	}
	if _, err := svc.Create(recruiter.ID, student.ID, "闭窗后重发"); err != nil {
		t.Fatalf("expired 之后应可立即重发（偏索引只认 pending）: %v", err)
	}

	// 5. 投递即授权的两条分支对窗口列的处置（ADR-0061 §2）：
	//    分支 2（无 pending → 新建 approved）**不写**窗口；
	//    分支 1（有 pending → 覆盖为 approved）保留该行原值作签发时留痕。
	//    `other` 此刻只有第 2 步手插的那条 approved、没有 pending ⇒ 走分支 2；
	//    `student` 在第 4 步末尾留有一条 pending ⇒ 走分支 1。
	if err := svc.EnsureApproved(db, recruiter.ID, other.ID, "投递即授权", time.Now()); err != nil {
		t.Fatalf("EnsureApproved(other): %v", err)
	}
	// 分支 2 的断言必须带 `source='application'`：第 2 步手插的那条 approved 也是无窗口的
	// （它的 source 走库默认值 recruiter），不加这个过滤就会白捡一次恒绿。
	var newApproved int64
	if err := db.Raw(`SELECT count(*) FROM contact_requests
		WHERE recruiter_id = ? AND student_user_id = ? AND status = 'approved'
		  AND source = 'application' AND expires_at IS NULL`,
		recruiter.ID, other.ID).Scan(&newApproved).Error; err != nil {
		t.Fatalf("查新建 approved: %v", err)
	}
	if newApproved == 0 {
		t.Fatalf("投递新建的 approved 不应带窗口（旧代码在此写 now+14d，正是「给永久授权编造期限」的洞）")
	}
	if err := svc.EnsureApproved(db, recruiter.ID, student.ID, "投递即授权", time.Now()); err != nil {
		t.Fatalf("EnsureApproved(student): %v", err)
	}
	var kept int64
	if err := db.Raw(`SELECT count(*) FROM contact_requests
		WHERE recruiter_id = ? AND student_user_id = ? AND status = 'approved' AND expires_at IS NOT NULL`,
		recruiter.ID, student.ID).Scan(&kept).Error; err != nil {
		t.Fatalf("查覆盖分支: %v", err)
	}
	if kept == 0 {
		t.Fatalf("pending→approved 覆盖分支应保留该行原窗口值（签发时留痕），实际全被清空")
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
