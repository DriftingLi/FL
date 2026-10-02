// 人 / 社区 / 招聘域的 nonnil 行为例（批①-A 第③段）。
//
// 为什么单独成文件：判据 5 的证据跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
//
// 本域的出口有两种形状，两种都不是偷懒：
//   - 分页信封（topics / reports / items / …）：空库直接跑，量到的就是 make(…,0,0) 发出的 `[]`；
//   - **住在列表条目里的数组**（images / resume_certifications / …）：testutil.MarshalKey 只看顶层键，
//     所以出口返回那条条目本身（先例：course 表的 outletCatalogLevelNodeNoCourses 返回等级节点）。
//     播的那一行**不带图、不带证件**——要证的恰恰是「这一格空着时发的是什么」，
//     给它塞满数据就等于什么都没证。
//
// 下面是**逐字段跑过真实出口后留下的那半**。留在 `nullable` 的键连同量到的实际发出值记在这里
// （清单是按域扫出来的，判据是跑出来的——没跑过的不改判）：
//   - JobCardDTO.expected_regions / .photos / .resume_certifications / .resume_experiences 与
//     RecruitResumeCard.expected_regions / .resume_experiences ⇒ 六格都是 `service.JSONArray`
//     （`MarshalJSON` 在 `j == nil` 时字面发出 `null`），实测三档：
//     列 = `[]` ⇒ 发 `[]`；列 = SQL NULL ⇒ 明文卡四格发 `null`、脱敏卡的 expected_regions /
//     resume_experiences 发 `[]`（desensitize 的 `len(x)==0` 归一接住了这一档）；
//     列 = JSON 字面量 `null` ⇒ **两边的这两格都发 `null`**（4 字节长，`len==0` 那条守卫漏过）。
//     ⇒ 那句 `nullable` 是真的，属批①-B 补 x-nullable 的候选（契约上还没落 flag）。
//     「列里存 JSON null」并非只存在于想象：job_cards 的四列是 `NOT NULL DEFAULT '[]'`，
//     NOT NULL 挡得住 SQL NULL、挡不住 `'null'::jsonb`。
//   - ContributionItemDTO.files ⇒ `json:"files,omitempty"`：空集时**键整个缺席**，
//     testutil.MarshalKey 判红（它自己写明「被加了 omitempty ⇒ nonnil 表态就不成立了」）。
//     要举出非 null 只能投一份带文件的稿，那证的是「有内容」那一档，不是「空集也不为 null」。
//   - FavoritePageResult.favorites / NotePageDTO.items ⇒ 宿主文件由另一在飞分支持有，本波不改。
//
// 另有三格（ApplicationListResult.items / ReportListResult.items / PositionListDTO.positions）
// 的 DTO **组装点在 internal/api/**（服务层只回 `[]DTO + total`）。那三处按 handler 那一行
// 逐字复现包装（切片仍取自同一条服务方法），与本批 catalog 段同一口径——包体本身不碰。
package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nonnilOutletsPeople = map[string]func(t *testing.T) any{
	// 论坛读面（topics / images / replies / reports）的举证已随域包搬去 internal/forum/nonnil_outlets_test.go（ADR-0070 波 2b-2）。

	// ===== 简历 / 招聘 =====
	"service.RecruitResumeCard.resume_certifications": outletRecruitCardNoCerts,
	"service.RecruiterApplicationListResult.items":    outletRecruiterApplicationListEmpty,
	// 招聘者列表（auth.RecruiterListResult.items）的举证已随域包搬去 internal/auth/nonnil_outlets_test.go（ADR-0070 波 3a）。
	"service.JobListResult.items":      outletJobListEmpty,
	"service.HrwaiUserPageResult.list": outletHrwaiUserPageEmpty,
	"service.TutorListDTO.tutors":      outletTutorListEmpty,

	// ===== 投稿 / 资料 / 打卡 / 资料库 / 搜索 / 导师 =====
	// 通知域的 items 举证已随域包搬去 internal/notification/nonnil_outlets_test.go（ADR-0070）。
	// 积分域的 items / tasks 举证已随域包搬去 internal/points/nonnil_outlets_test.go（ADR-0070）。
	// 投稿域的 items 举证已随域包搬去 internal/contribution/nonnil_outlets_test.go（ADR-0070）。
	// 资料审核列表（auth.ProfileChangeRequestPageResult.requests）的举证已随域包搬去 internal/auth/nonnil_outlets_test.go（ADR-0070 波 3a）。
	// 打卡域的 days / items 举证已随域包搬去 internal/checkin/nonnil_outlets_test.go（ADR-0070）。
	// 搜索域分区（SearchSectionDTO.items）的举证已随域包搬去 internal/search/nonnil_outlets_test.go（波 4b）。
	"course.TutorCourseChaptersDTO.chapters": outletTutorCourseChaptersEmpty,

	// 岗位字典的 positions 举证已随域包搬去 internal/training/nonnil_outlets_test.go（ADR-0070 波 3b-2）。
	// 三格 handler 一行包出来的信封（见文件头那段）
	"service.ApplicationListResult.items": outletStudentApplicationListEmpty,
	"service.ReportListResult.items":      outletJobReportQueueEmpty,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsPeople)
}

// 论坛读面（topics / images / replies / reports）的举证已随域包搬去 internal/forum/nonnil_outlets_test.go（ADR-0070 波 2b-2）。
// ===== 简历 / 招聘 =====

// outletRecruitCardNoCerts 脱敏简历卡：持证那一格由 maskCertifications 兜底——脏数据 / 空列
// 都返回 make(0,0)，json.Marshal 出来恒是数组，所以这一格连「列里存 JSON null」那一档都发不出
// null（同一条 desensitize 的另两格走 `len(x)==0` 守卫，那一档会漏，见文件头）。
func outletRecruitCardNoCerts(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := testutil.SeedStudent(t, db, "公开简历卡主", "hash")
	seedBlankJobCard(t, db, owner.ID, "open")
	svc := NewRecruitService(db, zap.NewNop())
	res, err := svc.Get(owner.ID)
	if err != nil {
		t.Fatalf("取脱敏卡失败: %v", err)
	}
	return res
}

// seedBlankJobCard 播一张**四个 JSONB 列都没写过**的简历卡（visibility 由调用方给）。
func seedBlankJobCard(t *testing.T, db *gorm.DB, userID int, visibility string) {
	t.Helper()
	card := model.JobCard{
		UserID: userID, RealName: "张三丰", Visibility: visibility,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("播简历卡失败: %v", err)
	}
}

// outletRecruiterApplicationListEmpty 企业侧投递列表：职位存在、零投递时 items 是空集。
func outletRecruiterApplicationListEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	job := seedJobPosting(t, db, 7)
	svc := NewJobApplicationService(db, zap.NewNop(), nil, nil)
	res, err := svc.ListForRecruiter(7, job.ID, 1, 20)
	if err != nil {
		t.Fatalf("企业侧投递列表失败: %v", err)
	}
	return res
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

// outletJobListEmpty 职位列表：无职位时 items 是空集。
func outletJobListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewJobPostingService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.List(0, JobListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("职位列表失败: %v", err)
	}
	return res
}

// outletHrwaiUserPageEmpty 管理端用户列表：空库时 list 是空集。
func outletHrwaiUserPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewAdminService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.ListHrwaiUsers(1, 20, "")
	if err != nil {
		t.Fatalf("用户列表失败: %v", err)
	}
	return res
}

// outletTutorListEmpty 导师列表：空库时 tutors 是空集。
func outletTutorListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewAdminService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.GetTutors(1, 20, "")
	if err != nil {
		t.Fatalf("导师列表失败: %v", err)
	}
	return res
}

// ===== 投稿 / 资料 / 通知 / 积分 / 打卡 / 资料库 / 搜索 / 导师 =====

// outletTutorCourseChaptersEmpty 导师端章节列表：有课程、零章节时 chapters 是空集。
func outletTutorCourseChaptersEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := newTutorServiceForTest(t, db).GetCourseChapters(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("导师端章节列表失败: %v", err)
	}
	return res
}

// ===== 三格「handler 一行包出来」的信封 =====
//
// 服务方法只回 `[]DTO + total`，信封在 internal/api/ 里组装（本波不碰那个目录）。
// 下面三处**逐字**复现 handler 那一行，切片仍取自同一条服务方法 ⇒ 举的是同一个事实：
// 包出来的那一格由 `make(0,n)` 起手、从来不是 nil。

// outletStudentApplicationListEmpty 学员「我的投递」：零投递时 items 是空集。
func outletStudentApplicationListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewJobApplicationService(testutil.NewMemoryDB(t), zap.NewNop(), nil, nil)
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
	svc := NewJobReportService(testutil.NewMemoryDB(t), zap.NewNop())
	const page, pageSize = 1, 20
	items, total, err := svc.ListPendingReports(page, pageSize)
	if err != nil {
		t.Fatalf("职位举报队列失败: %v", err)
	}
	return &ReportListResult{Items: items, Total: total, Page: page, PageSize: pageSize}
}
