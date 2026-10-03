// 职位域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名 / 搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 四格随职位域从 internal/service 的 nonnilOutletsPeople 搬来（P2 波 4e），键前缀随包名改成 job。
// 后两格的 DTO **组装点在 internal/api**（服务层只回 `[]DTO + total`），本文件按 handler 那一行
// 逐字复现包装（切片仍取自同一条服务方法）—— 判词随格搬来，不改口径。
package job

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nonnilOutletsJob 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsJob = map[string]func(t *testing.T) any{
	"job.RecruiterApplicationListResult.items": outletRecruiterApplicationListEmpty,
	"job.JobListResult.items":                  outletJobListEmpty,
	"job.ApplicationListResult.items":          outletStudentApplicationListEmpty,
	"job.ReportListResult.items":               outletJobReportQueueEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsJob})
}

// seedJobPosting 播一个职位（投递/举报这类「先看职位在不在」的读面共用）。
func seedJobPosting(t *testing.T, db *gorm.DB, recruiterID int) *model.JobPosting {
	t.Helper()
	job := model.JobPosting{
		RecruiterID: recruiterID, Title: "叉车司机", Status: "open",
		PublishedAt: testutil.Now(), CreatedAt: testutil.Now(),
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("播职位失败: %v", err)
	}
	return &job
}

// outletRecruiterApplicationListEmpty 企业侧投递列表：职位存在、零投递时 items 是空集。
func outletRecruiterApplicationListEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	job := seedJobPosting(t, db, 7)
	svc := NewApplicationService(db, zap.NewNop(), nil, nil)
	res, err := svc.ListForRecruiter(7, job.ID, 1, 20)
	if err != nil {
		t.Fatalf("企业侧投递列表失败: %v", err)
	}
	return res
}

// outletJobListEmpty 职位列表：无职位时 items 是空集。
func outletJobListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.List(0, JobListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("职位列表失败: %v", err)
	}
	return res
}

// outletStudentApplicationListEmpty 学员「我的投递」：零投递时 items 是空集。
// 服务方法只回 `[]DTO + total`，信封在 internal/api 组装 ⇒ 这里逐字复现 handler 那一行。
func outletStudentApplicationListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewApplicationService(testutil.NewMemoryDB(t), zap.NewNop(), nil, nil)
	const page, pageSize = 1, 20
	items, total, err := svc.ListForStudent(1, page, pageSize)
	if err != nil {
		t.Fatalf("我的投递列表失败: %v", err)
	}
	return &ApplicationListResult{Items: items, Total: total, Page: page, PageSize: pageSize}
}

// outletJobReportQueueEmpty 管理端职位举报队列：零举报时 items 是空集。
func outletJobReportQueueEmpty(t *testing.T) any {
	t.Helper()
	svc := NewReportService(testutil.NewMemoryDB(t), zap.NewNop())
	const page, pageSize = 1, 20
	items, total, err := svc.ListPendingReports(page, pageSize)
	if err != nil {
		t.Fatalf("职位举报队列失败: %v", err)
	}
	return &ReportListResult{Items: items, Total: total, Page: page, PageSize: pageSize}
}
