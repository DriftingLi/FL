// 内容精选的「指定发布时间（补录）/ 定时发布」语义锁（#1748，机制与被否备选见 ADR-0076）。
//
// 断言口径与 spec「Testing Decisions」一致：只断言**外部行为** —— 一次写请求之后库里是什么状态、
// 公开接口看不看得见、管理端筛出来是什么，以及一次守护 tick 之后哪些行变成已发布。
// 不断言 SQL 文本、不断言内部状态变量：语义矩阵换实现时本文件应当一字不改。
package featured

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// rfc3339 把时刻写成编辑器提交的形态（业务时区偏移，#1748 契约：严格 RFC3339）。
func rfc3339(t time.Time) string {
	return t.In(clock.Location()).Format(time.RFC3339)
}

// loadFeatured 回读一行（判据是「库里是什么状态」，不是「返回值说了什么」）。
func loadFeatured(t *testing.T, db *gorm.DB, id int) model.FeaturedContent {
	t.Helper()
	var row model.FeaturedContent
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("回读精选内容 %d 失败: %v", id, err)
	}
	return row
}

// assertSameInstant 断言指针时间与期望时刻**是同一时刻**（时区表示不同不影响判据）。
func assertSameInstant(t *testing.T, label string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil，期望 %s", label, rfc3339(want))
	}
	if delta := got.Sub(want); delta > time.Second || delta < -time.Second {
		t.Fatalf("%s = %s，期望 %s（差 %s）", label, rfc3339(*got), rfc3339(want), delta)
	}
}

// TestCreatePublishTimeMatrix 创建的语义矩阵（ADR-0076「写入语义」表逐行）。
func TestCreatePublishTimeMatrix(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	now := time.Now()
	past := rfc3339(now.Add(-24 * time.Hour))
	future := rfc3339(now.Add(2 * time.Hour))
	empty := ""
	publish := int16(1)
	draft := int16(0)

	cases := []struct {
		name          string
		status        *int16
		publishedAt   *string
		wantErr       error
		wantStatus    int16
		wantPublished bool // 是否恰为入参时间（true 时按入参断言）
		wantNow       bool // published_at 是否约等于现在
		wantScheduled bool
		wantTime      string // wantPublished 时比对的期望时刻
	}{
		{name: "缺省状态 + 缺省时间 = 草稿", wantStatus: 0},
		{name: "发布 + 缺省时间 = 立即发布", status: &publish, wantStatus: 1, wantNow: true},
		{name: "发布 + 过去时间 = 补录", status: &publish, publishedAt: &past, wantStatus: 1, wantPublished: true, wantTime: past},
		{name: "发布 + 空串 = 用现在", status: &publish, publishedAt: &empty, wantStatus: 1, wantNow: true},
		{name: "发布 + 未来时间 = 定时中", status: &publish, publishedAt: &future, wantStatus: 0, wantScheduled: true, wantTime: future},
		{name: "草稿 + 过去时间 = 400", status: &draft, publishedAt: &past, wantErr: ErrFeaturedDraftWithPublishTime},
		{name: "草稿 + 空串 = 400", status: &draft, publishedAt: &empty, wantErr: ErrFeaturedDraftWithPublishTime},
		{name: "非法格式 = 400", status: &publish, publishedAt: strPtr("2026-13-32T09:00:00+08:00"), wantErr: ErrFeaturedPublishTimeInvalid},
		{name: "越界（1999）= 400", status: &publish, publishedAt: strPtr("1999-12-31T23:59:59+08:00"), wantErr: ErrFeaturedPublishTimeInvalid},
		{name: "越界（11 年后）= 400", status: &publish, publishedAt: strPtr(rfc3339(now.AddDate(11, 0, 0))), wantErr: ErrFeaturedPublishTimeInvalid},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := svc.Create(FeaturedContentInput{
				Title:       "矩阵-" + c.name,
				Category:    "industry",
				Status:      c.status,
				PublishedAt: c.publishedAt,
			})
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("Create err = %v，期望 %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create 失败: %v", err)
			}
			row := loadFeatured(t, db, got.ContentID)
			if row.Status != c.wantStatus {
				t.Fatalf("status = %d，期望 %d", row.Status, c.wantStatus)
			}
			switch {
			case c.wantScheduled:
				want, _ := time.Parse(time.RFC3339, c.wantTime)
				assertSameInstant(t, "scheduled_at", row.ScheduledAt, want)
				if row.PublishedAt != nil {
					t.Fatalf("定时中的 published_at 应为空，got %s", rfc3339(*row.PublishedAt))
				}
			case c.wantPublished:
				want, _ := time.Parse(time.RFC3339, c.wantTime)
				assertSameInstant(t, "published_at", row.PublishedAt, want)
				if row.ScheduledAt != nil {
					t.Fatalf("已发布的 scheduled_at 应为空，got %s", rfc3339(*row.ScheduledAt))
				}
			case c.wantNow:
				assertSameInstant(t, "published_at", row.PublishedAt, now)
				if row.ScheduledAt != nil {
					t.Fatalf("立即发布的 scheduled_at 应为空")
				}
			default:
				if row.PublishedAt != nil || row.ScheduledAt != nil {
					t.Fatalf("草稿不应带时间：published=%v scheduled=%v", row.PublishedAt, row.ScheduledAt)
				}
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// TestUpdatePublishTimeMatrix 更新的语义矩阵（含「已发布改到未来 → 重新定时中」与「取消排期」）。
func TestUpdatePublishTimeMatrix(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	now := time.Now()
	past := rfc3339(now.Add(-48 * time.Hour))
	future := rfc3339(now.Add(3 * time.Hour))
	publish := int16(1)
	draft := int16(0)

	// 起草稿（未携带时间）
	created, err := svc.Create(FeaturedContentInput{Title: "更新矩阵", Category: "industry"})
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	id := created.ContentID

	// 草稿 → 发布 + 过去时间 = 补录（时间轴按入参排）
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Status: &publish, PublishedAt: &past}); err != nil {
		t.Fatalf("草稿→发布+过去时间应成功（补录）: %v", err)
	}
	row := loadFeatured(t, db, id)
	wantPast, _ := time.Parse(time.RFC3339, past)
	assertSameInstant(t, "补录后 published_at", row.PublishedAt, wantPast)

	// 已发布 + 未来时间 → 重新进入定时中（用户故事 11：时间与可见性始终一致）
	if _, err := svc.Update(id, FeaturedContentUpdateInput{PublishedAt: &future}); err != nil {
		t.Fatalf("已发布改到未来应成功: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.Status != 0 {
		t.Fatalf("改到未来后 status = %d，期望 0（定时中）", row.Status)
	}
	if row.PublishedAt != nil {
		t.Fatalf("定时中的 published_at 应为空，got %s", rfc3339(*row.PublishedAt))
	}
	wantFuture, _ := time.Parse(time.RFC3339, future)
	assertSameInstant(t, "定时中的 scheduled_at", row.ScheduledAt, wantFuture)

	// 定时中 + 只改状态（不带时间）→ 立即发布：补当前时间并清空排期
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Status: &publish}); err != nil {
		t.Fatalf("定时中→已发布应成功: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.Status != 1 {
		t.Fatalf("status = %d，期望 1", row.Status)
	}
	if row.ScheduledAt != nil {
		t.Fatalf("发布后 scheduled_at 应被清空，got %s", rfc3339(*row.ScheduledAt))
	}
	assertSameInstant(t, "发布后 published_at", row.PublishedAt, now)

	// 定时中 + 转为草稿（不带时间）→ 取消排期（定时态本来就没有 published_at）
	if _, err := svc.Update(id, FeaturedContentUpdateInput{PublishedAt: &future}); err != nil {
		t.Fatalf("再次排期失败: %v", err)
	}
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Status: &draft}); err != nil {
		t.Fatalf("取消定时应成功: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.Status != 0 || row.ScheduledAt != nil {
		t.Fatalf("取消定时后应为普通草稿：status=%d scheduled=%v", row.Status, row.ScheduledAt)
	}

	// 已发布 + 转为草稿（不带时间）→ published_at 保留（历史发布时刻不被销毁）
	if _, err := svc.Publish(id, nil); err != nil {
		t.Fatalf("重新发布失败: %v", err)
	}
	publishAt := *loadFeatured(t, db, id).PublishedAt
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Status: &draft}); err != nil {
		t.Fatalf("已发布转草稿应成功: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.Status != 0 {
		t.Fatalf("转为草稿后 status = %d，期望 0", row.Status)
	}
	if row.PublishedAt == nil || !row.PublishedAt.Equal(publishAt) {
		t.Fatal("转为草稿应保留 published_at（历史发布时刻不被销毁）")
	}

	// 草稿 + 携带时间 → 400
	if _, err := svc.Update(id, FeaturedContentUpdateInput{PublishedAt: &past}); !errors.Is(err, ErrFeaturedDraftWithPublishTime) {
		t.Fatalf("草稿携带时间应 400，err = %v", err)
	}
}

// TestPublishEndpointMatrix 发布端点的语义（无 body = 立即发布；带未来时间 = 转为定时中）。
func TestPublishEndpointMatrix(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	now := time.Now()
	future := rfc3339(now.Add(90 * time.Minute))

	draft, err := svc.Create(FeaturedContentInput{Title: "发布矩阵", Category: "industry"})
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 无 body → 立即发布
	if _, err := svc.Publish(draft.ContentID, nil); err != nil {
		t.Fatalf("Publish(nil) 失败: %v", err)
	}
	row := loadFeatured(t, db, draft.ContentID)
	if row.Status != 1 {
		t.Fatalf("status = %d，期望 1", row.Status)
	}
	assertSameInstant(t, "立即发布 published_at", row.PublishedAt, now)

	// 已发布 + 无 body → 幂等（不改 published_at）
	before := *row.PublishedAt
	if _, err := svc.Publish(draft.ContentID, nil); err != nil {
		t.Fatalf("重复发布失败: %v", err)
	}
	if after := *loadFeatured(t, db, draft.ContentID).PublishedAt; !after.Equal(before) {
		t.Fatalf("重复发布改了 published_at：%s → %s", rfc3339(before), rfc3339(after))
	}

	// 已发布 + 未来时间 → 转为定时中
	if _, err := svc.Publish(draft.ContentID, &future); err != nil {
		t.Fatalf("带未来时间发布失败: %v", err)
	}
	row = loadFeatured(t, db, draft.ContentID)
	if row.Status != 0 || row.ScheduledAt == nil || row.PublishedAt != nil {
		t.Fatalf("未来时间应落成定时中：status=%d scheduled=%v published=%v", row.Status, row.ScheduledAt, row.PublishedAt)
	}

	// 立即发布后再带**过去**时间发布 = 补录：发布时间被改成该时刻（不是现在）
	past := rfc3339(now.Add(-72 * time.Hour))
	if _, err := svc.Publish(draft.ContentID, &past); err != nil {
		t.Fatalf("带过去时间发布失败: %v", err)
	}
	row = loadFeatured(t, db, draft.ContentID)
	if row.Status != 1 {
		t.Fatalf("补录后 status = %d，期望 1", row.Status)
	}
	assertSameInstant(t, "补录后的 published_at", row.PublishedAt, now.Add(-72*time.Hour))

	// 非法时间 → 具名 400 事实（不落 500）
	bad := "not-a-time"
	if _, err := svc.Publish(draft.ContentID, &bad); !errors.Is(err, ErrFeaturedPublishTimeInvalid) {
		t.Fatalf("非法时间应返回 ErrFeaturedPublishTimeInvalid，got %v", err)
	}
}

// TestScheduledContentInvisibleToPublic 定时中 / 草稿对访客完全不可见（#1748 用户故事 8/13）。
func TestScheduledContentInvisibleToPublic(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	future := rfc3339(time.Now().Add(2 * time.Hour))
	publish := int16(1)

	created, err := svc.Create(FeaturedContentInput{Title: "定时不可见", Category: "industry", Status: &publish, PublishedAt: &future})
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	id := created.ContentID
	if created.ScheduledAt == nil || created.Status != 0 {
		t.Fatalf("创建定时内容应回 status=0 且带 scheduled_at，got status=%d scheduled=%v", created.Status, created.ScheduledAt)
	}

	if list, err := svc.GetPublicList(1, 20, ""); err != nil {
		t.Fatalf("公开列表失败: %v", err)
	} else if list.Total != 0 {
		t.Fatalf("公开列表含定时内容：total = %d", list.Total)
	}
	if _, err := svc.GetPublicDetail(id, false); !errors.Is(err, ErrFeaturedContentNotFound) {
		t.Fatalf("公开详情应 404，got %v", err)
	}
	if _, err := svc.IncrementViewCount(id); !errors.Is(err, ErrFeaturedContentNotFound) {
		t.Fatalf("阅读量端点应 404，got %v", err)
	}

	// 管理端能按 scheduled 筛出来（同一行的两副面孔）
	page, err := svc.AdminList(1, 20, "", featuredStateScheduled)
	if err != nil {
		t.Fatalf("管理端 scheduled 过滤失败: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ContentID != id {
		t.Fatalf("scheduled 过滤结果不对：total=%d items=%d", page.Total, len(page.Items))
	}
	if page.Items[0].ScheduledAt == nil {
		t.Fatal("管理端列表项缺 scheduled_at（三态徽标与排期列都靠它）")
	}

	// 到点后（守护动作）就可见了
	if n, err := svc.PublishDueScheduled(context.Background(), time.Now().Add(3*time.Hour)); err != nil {
		t.Fatalf("守护动作失败: %v", err)
	} else if n != 1 {
		t.Fatalf("到期补发条数 = %d，期望 1", n)
	}
	row := loadFeatured(t, db, id)
	if row.Status != 1 || row.ScheduledAt != nil || row.PublishedAt == nil {
		t.Fatalf("补发后形状不对：status=%d scheduled=%v published=%v", row.Status, row.ScheduledAt, row.PublishedAt)
	}
	if list, err := svc.GetPublicList(1, 20, ""); err != nil || list.Total != 1 {
		t.Fatalf("补发后公开列表应含该篇：err=%v total=%v", err, list.Total)
	}
}

// TestPublishDueScheduledIsIdempotent 守护动作的三条判据：到期翻转 / 未到期不动 / 重复调用 0 行。
func TestPublishDueScheduledIsIdempotent(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	// 与写入侧同口径（UTC）：SQLite 按文本比较带偏移的时间，混用偏移会让 `<=` 静默漏行。
	now := time.Now().UTC()

	seed := func(title string, scheduledAt *time.Time) int {
		t.Helper()
		row := model.FeaturedContent{
			Title:       title,
			Category:    "industry",
			Status:      0,
			ScheduledAt: scheduledAt,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("种入定时行失败: %v", err)
		}
		return row.ContentID
	}

	due1 := now.Add(-2 * time.Hour)
	due2 := now.Add(-1 * time.Minute)
	notDue := now.Add(30 * time.Minute)
	id1 := seed("到期一", &due1)
	id2 := seed("到期二", &due2)
	id3 := seed("未到期", &notDue)

	// 已发布行不受影响（条件里的 status = 0 就是这条保证）
	publishedAt := now.Add(-5 * time.Hour)
	alreadyPublished := model.FeaturedContent{Title: "已发布", Category: "industry", Status: 1, PublishedAt: &publishedAt, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&alreadyPublished).Error; err != nil {
		t.Fatalf("种入已发布行失败: %v", err)
	}

	n, err := svc.PublishDueScheduled(context.Background(), now)
	if err != nil {
		t.Fatalf("守护动作失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("到期补发条数 = %d，期望 2（多行一次补发）", n)
	}
	for _, c := range []struct {
		id   int
		want time.Time
	}{{id1, due1}, {id2, due2}} {
		row := loadFeatured(t, db, c.id)
		if row.Status != 1 {
			t.Fatalf("%d 未落成已发布", c.id)
		}
		assertSameInstant(t, "补发后的 published_at 应等于排期时间", row.PublishedAt, c.want)
		if row.ScheduledAt != nil {
			t.Fatalf("%d 补发后 scheduled_at 应清空", c.id)
		}
	}
	if row := loadFeatured(t, db, id3); row.Status != 0 || row.ScheduledAt == nil {
		t.Fatalf("未到期行被动了：status=%d scheduled=%v", row.Status, row.ScheduledAt)
	}
	if row := loadFeatured(t, db, alreadyPublished.ContentID); row.PublishedAt == nil || !row.PublishedAt.Equal(publishedAt) {
		t.Fatal("已发布行的 published_at 被守护改写了")
	}

	// 重复调用（多实例 / 重跑）：0 行
	if n, err := svc.PublishDueScheduled(context.Background(), now); err != nil || n != 0 {
		t.Fatalf("重复调用应为 0 行，got n=%d err=%v", n, err)
	}
}

// TestAdminListStateFilter 管理端三态过滤（#1748 用户故事 16）。
func TestAdminListStateFilter(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	now := time.Now()

	// 草稿、定时中、已发布各一行（直接种入，避开写面语义：本用例只锁过滤）
	draftRow := model.FeaturedContent{Title: "草稿", Category: "industry", Status: 0, CreatedAt: now, UpdatedAt: now}
	future := now.Add(time.Hour)
	schedRow := model.FeaturedContent{Title: "定时", Category: "industry", Status: 0, ScheduledAt: &future, CreatedAt: now, UpdatedAt: now}
	pubRow := model.FeaturedContent{Title: "已发布", Category: "industry", Status: 1, PublishedAt: &now, CreatedAt: now, UpdatedAt: now}
	for _, r := range []*model.FeaturedContent{&draftRow, &schedRow, &pubRow} {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("种入失败: %v", err)
		}
	}

	cases := []struct {
		state string
		want  int
	}{
		{"", 3},
		{featuredStateDraft, 1},
		{featuredStateScheduled, 1},
		{featuredStatePublished, 1},
	}
	for _, c := range cases {
		page, err := svc.AdminList(1, 20, "", c.state)
		if err != nil {
			t.Fatalf("state=%q 过滤失败: %v", c.state, err)
		}
		if int(page.Total) != c.want {
			t.Fatalf("state=%q 命中 %d 行，期望 %d", c.state, page.Total, c.want)
		}
	}

	// 非法的 state 取值 → 具名事实（由 handler 落 400，不静默当「全部」）
	if _, err := svc.AdminList(1, 20, "", "bogus"); !errors.Is(err, ErrFeaturedStateInvalid) {
		t.Fatalf("非法 state 应返回 ErrFeaturedStateInvalid，got %v", err)
	}
}

// TestUpdateContentOnlyKeepsSchedule 只改正文/标题不动排期（#1748 用户故事 6）。
//
// 这条锁的是「一次状态都没点名的更新」与「点名了状态的更新」之间的分界：前者不能顺手把排期
// 清掉（否则临上线前改个错别字就会让定时失效），后者必须清（草稿/发布的落定本身就是取消排期）。
func TestUpdateContentOnlyKeepsSchedule(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	future := time.Now().Add(3 * time.Hour)
	publish := int16(1)

	created, err := svc.Create(FeaturedContentInput{Title: "只改正文", Category: "industry", Status: &publish, PublishedAt: strPtr(rfc3339(future))})
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	id := created.ContentID

	// 只改正文：排期必须原样保留，state 仍是「定时中」
	body := "改过的正文"
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Content: &body}); err != nil {
		t.Fatalf("只改正文失败: %v", err)
	}
	row := loadFeatured(t, db, id)
	if row.Status != 0 || row.ScheduledAt == nil {
		t.Fatalf("只改正文不应取消排期：status=%d scheduled=%v", row.Status, row.ScheduledAt)
	}
	assertSameInstant(t, "排期时间", row.ScheduledAt, future)

	// 点名状态（不带时间）= 落定：转为草稿即取消定时
	draft := int16(0)
	if _, err := svc.Update(id, FeaturedContentUpdateInput{Status: &draft}); err != nil {
		t.Fatalf("转为草稿失败: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.Status != 0 || row.ScheduledAt != nil {
		t.Fatalf("转为草稿应清空排期：status=%d scheduled=%v", row.Status, row.ScheduledAt)
	}
}

// TestScheduledAtInvariantEnforcedInPostgres 库层 CHECK（#1748 不变量 2）：已发布与排期不可同时成立。
//
// 为什么单开一条 PG-only 契约：SQLite 测试库走 AutoMigrate（不执行 migrations/*.sql），
// 「约束真的存在、真的拦得住」只有真 Postgres 能回答（口径见 testutil.NewPostgresDB 头注释）。
// 本机无 DATABASE_URL 时干净 skip，CI 的 Postgres service 上真跑。
func TestScheduledAtInvariantEnforcedInPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	now := time.Now().UTC()
	future := now.Add(time.Hour)

	conflict := model.FeaturedContent{
		Title: "冲突行", Category: "industry", Status: 1,
		PublishedAt: &now, ScheduledAt: &future,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&conflict).Error; err == nil {
		t.Fatal("status = 1 且 scheduled_at 非空应被库层 CHECK 拒绝（不变量 2）")
	}

	// 反例：草稿 + 排期是合法形状（守护要处理的那一类行）
	legal := model.FeaturedContent{
		Title: "合法定时", Category: "industry", Status: 0,
		ScheduledAt: &future, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&legal).Error; err != nil {
		t.Fatalf("草稿 + 排期应可入库: %v", err)
	}
}
