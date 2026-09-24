// 人 / 社区 / 招聘域的 nonnil 行为例（批①-A 第③段）。
//
// 为什么单独成文件：判据 5 的证据跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
//
// 本域的出口有两种形状，两种都不是偷懒：
//   - 分页信封（topics / reports / items / …）：空库直接跑，量到的就是 make(…,0,0) 发出的 `[]`；
//   - **住在列表条目里的数组**（images / resume_certifications / …）：marshalKey 只看顶层键，
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
//     marshalKey 判红（它自己写明「被加了 omitempty ⇒ nonnil 表态就不成立了」）。
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
	// ===== 论坛读面（images 三格走 imageURLsForWire 归一，ADR-0062 决策 12）=====
	"service.ForumTopicPageResult.topics":   outletForumTopicPageEmpty,
	"service.ForumTopicDTO.images":          outletForumTopicNoImages,
	"service.ForumTopicDetailDTO.replies":   outletForumTopicDetailNoReplies,
	"service.ForumReplyDTO.images":          outletForumReplyNoImages,
	"service.MyReplyPageResult.replies":     outletMyReplyPageEmpty,
	"service.MyReplyDTO.images":             outletMyReplyNoImages,
	"service.ForumReportPageResult.reports": outletForumReportPageEmpty,

	// ===== 简历 / 招聘 =====
	"service.RecruitResumeCard.resume_certifications": outletRecruitCardNoCerts,
	"service.RecruiterApplicationListResult.items":    outletRecruiterApplicationListEmpty,
	"service.RecruiterListResult.items":               outletRecruiterListEmpty,
	"service.JobListResult.items":                     outletJobListEmpty,
	"service.HrwaiUserPageResult.list":                outletHrwaiUserPageEmpty,
	"service.TutorListDTO.tutors":                     outletTutorListEmpty,

	// ===== 投稿 / 资料 / 通知 / 积分 / 打卡 / 资料库 / 搜索 / 导师 =====
	"service.ContributionPageResult.items":            outletContributionPageEmpty,
	"service.ContributionReportPageResult.items":      outletContributionReportPageEmpty,
	"service.ProfileChangeRequestPageResult.requests": outletProfileChangeRequestPageEmpty,
	"service.NotificationListPageResult.items":        outletNotificationListEmpty,
	"service.PointsLedgerResult.items":                outletPointsLedgerEmpty,
	"service.PointsTasksResult.tasks":                 outletPointsTasksNone,
	"service.CheckInCalendarResult.days":              outletCheckInCalendar,
	"service.CheckInRankResult.items":                 outletCheckInRankEmpty,
	"service.MaterialPageResult.materials":            outletMaterialPageEmpty,
	"service.SearchSectionDTO.items":                  outletSearchSectionEmpty,
	"service.TutorCourseChaptersDTO.chapters":         outletTutorCourseChaptersEmpty,

	// 三格 handler 一行包出来的信封（见文件头那段）
	"service.ApplicationListResult.items": outletStudentApplicationListEmpty,
	"service.ReportListResult.items":      outletJobReportQueueEmpty,
	"service.PositionListDTO.positions":   outletPositionListEmpty,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsPeople)
}

// ===== 论坛 =====

// outletForumTopicPageEmpty 主题列表：一行帖子都没有时 topics 仍是 make 出来的空集。
func outletForumTopicPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewForumService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListTopics(TopicListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("空库拉主题列表失败: %v", err)
	}
	return res
}

// outletForumTopicNoImages 主题条目里的 images：帖子的 images 列为 NULL（发帖未带图）时，
// 读面出口经 imageURLsForWire 归一成 `[]` —— 这条正是「契约说数组、出口却发 null」那个坑的正面证据。
func outletForumTopicNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	author := seedForumUser(t, db, "无图楼主")
	topic := seedRewardTopic(t, db, author.ID, "无图主题")
	svc := NewForumService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListTopics(TopicListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("拉主题列表失败: %v", err)
	}
	if len(res.Topics) == 0 {
		t.Fatalf("列表里没有取到刚播的主题: %v", topic.ID)
	}
	return res.Topics[0]
}

// outletForumTopicDetailNoReplies 主题详情：有帖无回复时 replies 是 make(0,pageSize) 的空集。
func outletForumTopicDetailNoReplies(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	author := seedForumUser(t, db, "无人回复楼主")
	topic := seedRewardTopic(t, db, author.ID, "无人回复的主题")
	svc := NewForumService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.GetTopic(TopicDetailInput{TopicID: topic.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("主题详情失败: %v", err)
	}
	return res
}

// outletForumReplyNoImages 回复条目里的 images：与主题同一条归一判据（replyRow.toDTO 也走
// imageURLsForWire），故出口单独占一键、各自 marshal。
func outletForumReplyNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	replier := seedForumUser(t, db, "无图回复人")
	topic := seedRewardTopic(t, db, replier.ID, "被回复的主题")
	seedPlainReply(t, db, topic.ID, replier.ID)
	svc := NewForumService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.GetTopic(TopicDetailInput{TopicID: topic.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("主题详情失败: %v", err)
	}
	if len(res.Replies) == 0 {
		t.Fatalf("详情里没取到刚播的回复：出口取不到 ForumReplyDTO，这条证据没有落地")
	}
	return res.Replies[0]
}

// outletMyReplyPageEmpty 「我的回复」：零回复时 replies 是空集。
func outletMyReplyPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewForumService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.MyReplies(1, 1, 20)
	if err != nil {
		t.Fatalf("空回复列表失败: %v", err)
	}
	return res
}

// outletMyReplyNoImages 「我的回复」条目里的 images：与主题/回复同一条归一判据。
func outletMyReplyNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	replier := seedForumUser(t, db, "我的无图回复人")
	topic := seedRewardTopic(t, db, replier.ID, "我的回复所属主题")
	seedPlainReply(t, db, topic.ID, replier.ID)
	svc := NewForumService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.MyReplies(replier.ID, 1, 20)
	if err != nil {
		t.Fatalf("我的回复列表失败: %v", err)
	}
	if len(res.Replies) == 0 {
		t.Fatalf("我的回复里没取到刚播的那条：出口取不到 MyReplyDTO，这条证据没有落地")
	}
	return res.Replies[0]
}

// outletForumReportPageEmpty 管理端举报列表：无举报时 reports 是空集。
func outletForumReportPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewForumModerationService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListReports(1, 20, nil)
	if err != nil {
		t.Fatalf("空举报列表失败: %v", err)
	}
	return res
}

// seedPlainReply 播一条**不带图**的回复（images 列留空），返回其行。
func seedPlainReply(t *testing.T, db *gorm.DB, topicID int64, userID int) *model.ForumReply {
	t.Helper()
	reply := model.ForumReply{
		TopicID: topicID, UserID: userID, Content: "无图回复正文",
		ContentFormat: ForumContentFormatText, CreatedAt: testutil.Now(),
	}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("播回复失败: %v", err)
	}
	return &reply
}

// ===== 简历 / 招聘 =====

// outletRecruitCardNoCerts 脱敏简历卡：持证那一格由 maskCertifications 兜底——脏数据 / 空列
// 都返回 make(0,0)，json.Marshal 出来恒是数组，所以这一格连「列里存 JSON null」那一档都发不出
// null（同一条 desensitize 的另两格走 `len(x)==0` 守卫，那一档会漏，见文件头）。
func outletRecruitCardNoCerts(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := seedForumUser(t, db, "公开简历卡主")
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

// outletRecruiterListEmpty 招聘者列表（#416 那条「硬编码空数组桩」的真实现）：空库发 `[]`。
func outletRecruiterListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewAuthService(testutil.NewMemoryDB(t), nil, nil, "", "", "", zap.NewNop())
	res, err := svc.ListRecruiters(1, 20, "")
	if err != nil {
		t.Fatalf("招聘者列表失败: %v", err)
	}
	return res
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

// outletContributionPageEmpty 投稿审核队列：零投稿时 items 是空集。
func outletContributionPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewContributionService(testutil.NewMemoryDB(t), nil, nil, nil, zap.NewNop(), nil)
	res, err := svc.ListPending(1, 20)
	if err != nil {
		t.Fatalf("投稿队列失败: %v", err)
	}
	return res
}

// outletContributionReportPageEmpty 投稿举报队列：零举报时 items 是空集。
func outletContributionReportPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewContributionService(testutil.NewMemoryDB(t), nil, nil, nil, zap.NewNop(), nil)
	res, err := svc.ListReports(1, 20, nil)
	if err != nil {
		t.Fatalf("投稿举报队列失败: %v", err)
	}
	return res
}

// outletProfileChangeRequestPageEmpty 资料审核列表：零申请时 requests 是空集。
func outletProfileChangeRequestPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewProfileReviewService(testutil.NewMemoryDB(t), nil, nil, zap.NewNop())
	res, err := svc.ListRequests("", 1, 20)
	if err != nil {
		t.Fatalf("资料审核列表失败: %v", err)
	}
	return res
}

// outletNotificationListEmpty 站内信列表：零消息时 items 是空集。
func outletNotificationListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewNotificationService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.List(1, 1, 20)
	if err != nil {
		t.Fatalf("站内信列表失败: %v", err)
	}
	return res
}

// outletPointsLedgerEmpty 积分流水：零流水时 items 是空集。
func outletPointsLedgerEmpty(t *testing.T) any {
	t.Helper()
	svc := NewPointsService(testutil.NewMemoryDB(t), zap.NewNop(), nil, nil)
	res, err := svc.GetLedger(1, 1, 20, "")
	if err != nil {
		t.Fatalf("积分流水失败: %v", err)
	}
	return res
}

// outletPointsTasksNone 任务列表：一条任务配置都没有时 tasks 是空集。
// 播一个用户不是为了让 tasks 有内容，而是 loadTaskMeta 先读资料（读不到就 error）。
func outletPointsTasksNone(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "任务学员", "x")
	svc := NewPointsService(db, zap.NewNop(), nil, nil)
	res, err := svc.GetTasks(student.ID)
	if err != nil {
		t.Fatalf("任务列表失败: %v", err)
	}
	return res
}

// outletCheckInCalendar 打卡日历：days 恒是**整月每一天**（未打卡日 checked=false），
// 结构上不存在空月，所以这条举的是「非 null」而不是「空集」——断言放宽后才举得出来。
func outletCheckInCalendar(t *testing.T) any {
	t.Helper()
	svc := NewCheckInService(testutil.NewMemoryDB(t), zap.NewNop(), nil, nil)
	res, err := svc.GetCheckInCalendar(1, 2026, 9)
	if err != nil {
		t.Fatalf("打卡日历失败: %v", err)
	}
	return res
}

// outletCheckInRankEmpty 打卡排行榜：没人打卡时 items 是空集。
func outletCheckInRankEmpty(t *testing.T) any {
	t.Helper()
	svc := NewCheckInService(testutil.NewMemoryDB(t), zap.NewNop(), nil, nil)
	res, err := svc.GetCheckInRank(0, 1, 20)
	if err != nil {
		t.Fatalf("打卡排行榜失败: %v", err)
	}
	return res
}

// outletMaterialPageEmpty 资料列表：零挂载资料时 materials 是空集。
func outletMaterialPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewMaterialService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.ListMaterials(1, 20, 0)
	if err != nil {
		t.Fatalf("资料列表失败: %v", err)
	}
	return res
}

// outletSearchSectionEmpty 聚合搜索的单个分区：SearchAllDTO 的五个分区字段共用这一条出口、
// 各自 marshal（同一处 make，一格一证）。
func outletSearchSectionEmpty(t *testing.T) any {
	t.Helper()
	svc := NewSearchService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.Search("液压泵压力不足", "", 1, 20, nil)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	all, ok := res.(*SearchAllDTO)
	if !ok {
		t.Fatalf("聚合搜索返回了意料之外的类型 %T", res)
	}
	return all.Courses
}

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

// outletPositionListEmpty 岗位字典：空库时 positions 是空集（catalogList 的 make(0,n)）。
func outletPositionListEmpty(t *testing.T) any {
	t.Helper()
	return PositionListDTO{Positions: NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop()).ListPositions(false)}
}
