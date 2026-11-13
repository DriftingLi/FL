// 详情正文按权益空化（#2448，ADR-0062 决策 3）：详情是发现面——章节元信息全量下发，
// 但 content 只发给有权益的读者，未解锁时置空串（字段保留、形状不变）。
// 泄漏在响应体，与 UI 是否渲染无关：这里锁的是「响应里根本没有正文」，不是「前端不渲染」。
package course

import (
	"context"
	"strconv"
	"testing"

	"forklift-training/internal/entitlement"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"

	"go.uber.org/zap"
)

// TestGetCourseDetailBlanksLockedChapterContent 未解锁：正文空化、元信息保留；
// 同一门课兑换后正文回来（证明空化不是恒空的死闸）。
func TestGetCourseDetailBlanksLockedChapterContent(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	student := testutil.SeedStudent(t, db, "detail_lock_stu", "hash")

	spec := model.Specialty{Code: "detail-lock", Name: "空化方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "detail-lock-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	price := 100
	priced := model.Course{Name: "付费课", Status: 1, PointsPrice: &price,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&priced).Error; err != nil {
		t.Fatalf("播种付费课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: priced.CourseID, Title: "万用表使用", Content: "本节讲万用表的读法",
		OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	// C11：学员读面限已发布槽——夹具走真实过审路径出槽
	if err := SubmitChapterForReview(context.Background(), db, ch.ChapterID, 7); err != nil {
		t.Fatalf("夹具提交审核失败: %v", err)
	}
	if err := ApproveChapterReview(context.Background(), db, ch.ChapterID); err != nil {
		t.Fatalf("夹具过审失败: %v", err)
	}

	// 未兑换学员：正文空串，标题/序号照发（「未解锁」态展示所需的元信息不受影响）。
	detail, err := svc.GetCourseDetail(priced.CourseID, student.ID)
	if err != nil {
		t.Fatalf("读详情失败: %v", err)
	}
	if len(detail.Chapters) != 1 {
		t.Fatalf("章节应 1 条, got %d", len(detail.Chapters))
	}
	got := detail.Chapters[0]
	if got.Content != "" {
		t.Fatalf("未解锁章节正文必须为空串, got %q", got.Content)
	}
	if got.Title != "万用表使用" || got.OrderNum != 1 {
		t.Fatalf("章节元信息不得被空化波及: title=%q order=%d", got.Title, got.OrderNum)
	}
	if detail.CourseInfo.Entitled == nil || *detail.CourseInfo.Entitled {
		t.Fatalf("未兑换时 entitled 应为 false, got %v", detail.CourseInfo.Entitled)
	}

	// 无主体（未登录）也不算权益：付费课正文同样不下发。
	anon, err := svc.GetCourseDetail(priced.CourseID, 0)
	if err != nil {
		t.Fatalf("匿名读详情失败: %v", err)
	}
	if anon.Chapters[0].Content != "" {
		t.Fatalf("未登录读付费课不得下发正文, got %q", anon.Chapters[0].Content)
	}

	// 兑换后：正文完整回来。
	ent := model.UserEntitlement{UserID: student.ID, SKU: entitlement.CourseSKU(priced.CourseID),
		RefID: strconv.Itoa(priced.CourseID)}
	if err := db.Create(&ent).Error; err != nil {
		t.Fatalf("播种权益行失败: %v", err)
	}
	detail, err = svc.GetCourseDetail(priced.CourseID, student.ID)
	if err != nil {
		t.Fatalf("兑换后读详情失败: %v", err)
	}
	if detail.Chapters[0].Content != "本节讲万用表的读法" {
		t.Fatalf("已兑换正文必须原样返回, got %q", detail.Chapters[0].Content)
	}
	if detail.CourseInfo.Entitled == nil || !*detail.CourseInfo.Entitled {
		t.Fatalf("兑换后 entitled 应为 true, got %v", detail.CourseInfo.Entitled)
	}
}

// TestGetCourseDetailFreeCourseContentUnchanged 免费课（points_price NULL）不受空化影响：
// courseEntitled 对免费课恒 true，正文照发（含匿名读者）。
func TestGetCourseDetailFreeCourseContentUnchanged(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "detail-free", Name: "免费方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "detail-free-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	free := model.Course{Name: "免费课", Status: 1,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&free).Error; err != nil {
		t.Fatalf("播种免费课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: free.CourseID, Title: "安全须知", Content: "免费正文",
		OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	// C11：学员读面限已发布槽——夹具走真实过审路径出槽
	if err := SubmitChapterForReview(context.Background(), db, ch.ChapterID, 7); err != nil {
		t.Fatalf("夹具提交审核失败: %v", err)
	}
	if err := ApproveChapterReview(context.Background(), db, ch.ChapterID); err != nil {
		t.Fatalf("夹具过审失败: %v", err)
	}

	detail, err := svc.GetCourseDetail(free.CourseID, 0)
	if err != nil {
		t.Fatalf("读详情失败: %v", err)
	}
	if detail.Chapters[0].Content != "免费正文" {
		t.Fatalf("免费课正文不得被空化, got %q", detail.Chapters[0].Content)
	}
}
