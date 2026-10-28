// 「任务中心 5 处口径」的配置快照（#2065 / 报告 §8 Q11）——PG 专属。
//
// 为什么要落在**真迁移建出来的库**上：那 5 处错位（发布帖描述带「打卡」、评论带「不少于10字」、
// 每日答题与题库练习两条同码、今日可得 +80、登录描述「登录小程序」）当年全部是「文案 / 分值与
// 后端事实源对不上」，而事实源就是迁移链 000002 → 000021 → 000027 → 000034 → 000045 落到
// points_task_config 里的这批行。SQLite 测试库由 AutoMigrate 建表、**不执行迁移** ⇒ 拿不到这批
// 种子，快照只能落在 PG 面（与 internal/migrate 的迁移类断言同一取舍）。
//
// 快照的价值是「谁改配置就得改这张表」：多一行、少一行、改一个字都会红，
// 于是「顺手把退役任务加回来」「顺手写个字数阈值」都不再可能悄悄发生。
package api

import (
	"strings"
	"testing"

	"forklift-training/internal/testutil"
)

// taskConfigRow 快照一行（列与 points_task_config 的语义列一一对应；created_at 不参与）。
type taskConfigRow struct {
	Code        string
	Title       string
	Group       string
	Points      int
	DailyLimit  int
	TotalLimit  *int
	EventType   string
	Description string
}

// expectedTaskConfigs 任务配置的现行事实（2026-10-28 快照，11 行）。
// 改这里之前先读报告 §8 Q11 与 Q12+Q19：本表同时是「5 处口径」那五条断言的载体。
var expectedTaskConfigs = []taskConfigRow{
	{Code: "browse_course", Title: "浏览 3 门课程", Group: "daily", Points: 5, DailyLimit: 1, EventType: "course_browse", Description: "每日浏览 3 门课程"},
	{Code: "daily_browse", Title: "浏览 3 篇帖子", Group: "daily", Points: 5, DailyLimit: 1, EventType: "forum_view", Description: "每日浏览 3 篇帖子"},
	{Code: "daily_login", Title: "每日登录", Group: "daily", Points: 5, DailyLimit: 1, EventType: "login", Description: "每日进入任务中心即可领取"},
	{Code: "daily_quiz", Title: "每日答题 1 次", Group: "daily", Points: 10, DailyLimit: 1, EventType: "question_submit", Description: "每日完成任意练习/模考 1 次"},
	{Code: "growth_mock", Title: "完成 1 次模考", Group: "daily", Points: 20, DailyLimit: 1, EventType: "mock_submit", Description: "每日完成 1 次模考"},
	{Code: "growth_post", Title: "发布 1 篇帖子", Group: "daily", Points: 10, DailyLimit: 1, EventType: "topic_create", Description: "每日发布 1 篇帖子"},
	{Code: "growth_reply", Title: "回复 3 次", Group: "daily", Points: 10, DailyLimit: 1, EventType: "reply_create", Description: "每日回复 3 次"},
	{Code: "newbie_credential", Title: "选定目标证件", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "credential_onboarding", Description: "完成 onboarding 选定当前证件"},
	{Code: "newbie_first_course", Title: "完成首节课程", Group: "newbie", Points: 20, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "course_complete", Description: "完成首节课程学习"},
	{Code: "newbie_profile_basic", Title: "完善基础资料", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "profile_complete", Description: "上传头像且设置昵称"},
	{Code: "newbie_profile_contact", Title: "完善联系资料", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "profile_complete", Description: "填写单位且绑定手机/邮箱"},
}

func TestTaskCenterConfigSnapshot_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	var rows []taskConfigRow
	if err := db.Table("points_task_config").Order("code ASC").Find(&rows).Error; err != nil {
		t.Fatalf("读任务配置失败: %v", err)
	}

	// 1. 整表快照：多一行 / 少一行 / 改一个字都红。
	if len(rows) != len(expectedTaskConfigs) {
		t.Fatalf("任务配置行数 = %d, want %d（改配置必须同步本快照）", len(rows), len(expectedTaskConfigs))
	}
	for i := range expectedTaskConfigs {
		want, got := expectedTaskConfigs[i], rows[i]
		if got.Code != want.Code || got.Title != want.Title || got.Group != want.Group ||
			got.Points != want.Points || got.DailyLimit != want.DailyLimit ||
			got.EventType != want.EventType || got.Description != want.Description ||
			!sameIntPtr(got.TotalLimit, want.TotalLimit) {
			t.Fatalf("第 %d 行配置与快照不一致：\n got  %+v\n want %+v", i, got, want)
		}
	}

	// 2. 退役任务不再出现（Q11 第 1 条）：daily_checkin 随 ADR-0028 退役、growth_first_experience 随 ADR-0040 退役。
	for _, code := range []string{"daily_checkin", "growth_first_experience"} {
		for _, r := range rows {
			if r.Code == code {
				t.Fatalf("已退役的任务 %s 又回到配置里（退役任务不该出现在可领列表）", code)
			}
		}
	}

	// 3. 无判据的字数阈值（Q11 第 2 条）：没有「发表评论 ≥N 字」这类任务，文案里也不许出现阈值。
	for _, r := range rows {
		for _, banned := range []string{"字数", "字以上", "不少于", "≥"} {
			if strings.Contains(r.Title+r.Description, banned) {
				t.Fatalf("任务 %s 的文案带了无判据的字数阈值（%q）：%s / %s", r.Code, banned, r.Title, r.Description)
			}
		}
	}

	// 4. 同码两任务合一（Q11 第 3 条）：daily_quiz 只有一行，且它的判定同时吃「练习」与「模考」
	//    （两件行为一个码，见 points.taskProgressFor 的 daily_quiz case）。
	quizRows := 0
	for _, r := range rows {
		if r.Code == "daily_quiz" {
			quizRows++
		}
	}
	if quizRows != 1 {
		t.Fatalf("daily_quiz 配置行数 = %d, want 1（原型第 2/8 项是同一件事，不得拆成两码）", quizRows)
	}

	// 5. 今日可得 = daily 组合计（Q11 第 4 条）：不写死数字，按配置求和——
	//    加了「浏览课程」之后是 65（原 60 + 5），页面同样按 daily 组实时算。
	dailyTotal := 0
	for _, r := range rows {
		if r.Group == "daily" {
			dailyTotal += r.Points
		}
	}
	if dailyTotal != 65 {
		t.Fatalf("daily 组今日可得合计 = %d, want 65（60 + 浏览课程 5）；改配置要同步页面口径与本断言", dailyTotal)
	}

	// 6. 领取口径（Q11 第 5 条）：daily_login 的描述说明「进任务中心即可领取」，不再写登录/小程序。
	for _, r := range rows {
		if r.Code != "daily_login" {
			continue
		}
		if r.Description != "每日进入任务中心即可领取" {
			t.Fatalf("daily_login 描述应为「每日进入任务中心即可领取」（ADR-0054），got %q", r.Description)
		}
	}
}

// sameIntPtr 两个 *int 同值判等（nil 与 nil 相等）；快照比对只关心「有没有终身额度」。
func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
