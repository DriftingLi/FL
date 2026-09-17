package service

import (
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// 采纳奖励防刷策略：纯函数表驱动（internal seam，无库、无事务）。
// ===== 模块 interface 测试（主 seam：forumRewardPolicy，in-process，真实 sqlite + 事务） =====

func newRewardPolicyTest(t *testing.T) (*forumRewardPolicy, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	points := NewPointsService(db, zap.NewNop(), nil, NewNotificationService(db, zap.NewNop()))
	policy := newForumRewardPolicy(points, NewNotificationService(db, zap.NewNop()))
	return policy, db
}

func seedRewardTopic(t *testing.T, db *gorm.DB, ownerID int, title string) *model.ForumTopic {
	t.Helper()
	topic := &model.ForumTopic{UserID: ownerID, Category: ForumCategoryQuestion, Title: title, Content: "正文", ContentFormat: ForumContentFormatText}
	if err := db.Create(topic).Error; err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	return topic
}

func balanceOf(t *testing.T, db *gorm.DB, userID int) int {
	t.Helper()
	var u model.HrwaiUser
	if err := db.First(&u, userID).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	return u.PointsBalance
}

func ledgerReasons(t *testing.T, db *gorm.DB, userID int) []string {
	t.Helper()
	var reasons []string
	if err := db.Model(&model.PointsLedger{}).Where("user_id = ?", userID).Order("id").Pluck("reason", &reasons).Error; err != nil {
		t.Fatalf("查询流水失败: %v", err)
	}
	return reasons
}

// Award(采纳) 一次：答主 +40、楼主 +5，双方站内信同事务落库。
func TestForumRewardPolicy_AwardAcceptPaysBothSides(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "owner", "x")
	answerer := testutil.SeedStudent(t, db, "answerer", "x")
	topic := seedRewardTopic(t, db, owner.ID, "怎么换液压油")

	at := time.Date(2026, 9, 13, 10, 0, 0, 0, clock.Location())
	err := db.Transaction(func(tx *gorm.DB) error {
		return policy.Award(tx, forumRewardFact{
			Kind: forumRewardAccept, TopicID: topic.ID, TopicTitle: topic.Title,
			TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 77, At: at,
		})
	})
	if err != nil {
		t.Fatalf("Award 失败: %v", err)
	}

	if got := balanceOf(t, db, answerer.ID); got != AcceptBonusPoints {
		t.Fatalf("答主余额 = %d, want %d", got, AcceptBonusPoints)
	}
	if got := balanceOf(t, db, owner.ID); got != AcceptActionPoints {
		t.Fatalf("楼主余额 = %d, want %d", got, AcceptActionPoints)
	}
	if got := ledgerReasons(t, db, answerer.ID); len(got) != 1 || got[0] != ReasonAcceptedBonus {
		t.Fatalf("答主流水 = %v, want [%s]", got, ReasonAcceptedBonus)
	}
	if got := ledgerReasons(t, db, owner.ID); len(got) != 1 || got[0] != ReasonAcceptAction {
		t.Fatalf("楼主流水 = %v, want [%s]", got, ReasonAcceptAction)
	}

	var notes []model.Notification
	if err := db.Where("user_id IN ?", []int{owner.ID, answerer.ID}).Order("id").Find(&notes).Error; err != nil {
		t.Fatalf("查询站内信失败: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("站内信条数 = %d, want 2", len(notes))
	}
	if notes[0].Title != answererAcceptTitle || notes[1].Title != ownerAcceptTitle {
		t.Fatalf("站内信标题 = [%s, %s], want [%s, %s]", notes[0].Title, notes[1].Title, answererAcceptTitle, ownerAcceptTitle)
	}
	if notes[0].CreatedAt.UTC() != at.UTC() {
		t.Fatalf("站内信时间 = %v, want %v（注入时刻应原样落库）", notes[0].CreatedAt.UTC(), at.UTC())
	}
}

// 同一主题重复 Award：只发一次（取消后重采场景，判据是流水本身）。
func TestForumRewardPolicy_AwardAcceptIsIdempotent(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "owner2", "x")
	answerer := testutil.SeedStudent(t, db, "answerer2", "x")
	topic := seedRewardTopic(t, db, owner.ID, "重复采纳")
	fact := forumRewardFact{
		Kind: forumRewardAccept, TopicID: topic.ID, TopicTitle: topic.Title,
		TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 78, At: clock.Now(),
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error { return policy.Award(tx, fact) }); err != nil {
			t.Fatalf("第 %d 次 Award 失败: %v", i+1, err)
		}
	}
	if got := balanceOf(t, db, answerer.ID); got != AcceptBonusPoints {
		t.Fatalf("重复发放后答主余额 = %d, want %d（只发一次）", got, AcceptBonusPoints)
	}
	var cnt int64
	db.Model(&model.Notification{}).Where("user_id = ?", answerer.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("答主站内信 = %d 条, want 1（幂等不应重复通知）", cnt)
	}
}

// 配对衰减：同一「楼主↔答主」已付 3 次后，答主减半、楼主不受衰减影响。
// 配对计数是终身口径（流水自连接、无日期过滤），故造的历史流水放在昨天——
// 否则它们会同时触发当日封顶，测不出衰减本身。
func TestForumRewardPolicy_AwardAcceptAppliesPairDecay(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "decay_owner", "x")
	answerer := testutil.SeedStudent(t, db, "decay_answerer", "x")
	topic := seedRewardTopic(t, db, owner.ID, "配对衰减")
	for i, refID := range []string{"901", "902", "903"} {
		if err := db.Create(&model.PointsLedger{UserID: answerer.ID, Delta: AcceptBonusPoints, Reason: ReasonAcceptedBonus, RefType: forumTopicRefType, RefID: refID, CreatedAt: clock.Now().Add(-24 * time.Hour)}).Error; err != nil {
			t.Fatalf("造配对流水 %d 失败: %v", i, err)
		}
		if err := db.Create(&model.PointsLedger{UserID: owner.ID, Delta: AcceptActionPoints, Reason: ReasonAcceptAction, RefType: forumTopicRefType, RefID: refID, CreatedAt: clock.Now().Add(-24 * time.Hour)}).Error; err != nil {
			t.Fatalf("造配对流水 %d 失败: %v", i, err)
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return policy.Award(tx, forumRewardFact{Kind: forumRewardAccept, TopicID: topic.ID, TopicTitle: topic.Title, TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 9, At: clock.Now()})
	})
	if err != nil {
		t.Fatalf("Award 失败: %v", err)
	}
	if got, want := balanceOf(t, db, answerer.ID), AcceptBonusPoints/2; got != want {
		t.Fatalf("答主本次入账 = %d, want %d（第 4 次配对减半）", got, want)
	}
	if got := balanceOf(t, db, owner.ID); got != AcceptActionPoints {
		t.Fatalf("楼主入账 = %d, want %d（不参与配对衰减）", got, AcceptActionPoints)
	}
}

// 日封顶：答主当日已付 3 笔后本次归零，楼主仍有 5 分；两人同时触顶时不写任何流水与通知。
func TestForumRewardPolicy_AwardAcceptAppliesDailyCaps(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "cap_owner", "x")
	answerer := testutil.SeedStudent(t, db, "cap_answerer", "x")
	at := time.Date(2026, 9, 13, 10, 0, 0, 0, clock.Location())
	for i, refID := range []string{"801", "802", "803"} {
		if err := db.Create(&model.PointsLedger{UserID: answerer.ID, Delta: AcceptBonusPoints, Reason: ReasonAcceptedBonus, RefType: forumTopicRefType, RefID: refID, CreatedAt: at.Add(-time.Duration(i) * time.Minute)}).Error; err != nil {
			t.Fatalf("造当日流水失败: %v", err)
		}
	}
	topic := seedRewardTopic(t, db, owner.ID, "日封顶")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return policy.Award(tx, forumRewardFact{Kind: forumRewardAccept, TopicID: topic.ID, TopicTitle: topic.Title, TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 8, At: at})
	}); err != nil {
		t.Fatalf("Award 失败: %v", err)
	}
	if got, want := balanceOf(t, db, answerer.ID), 0; got != want {
		t.Fatalf("答主封顶后本次入账 = %d, want %d", got, want)
	}
	if got := balanceOf(t, db, owner.ID); got != AcceptActionPoints {
		t.Fatalf("楼主入账 = %d, want %d", got, AcceptActionPoints)
	}
	var notes int64
	db.Model(&model.Notification{}).Where("user_id = ?", answerer.ID).Count(&notes)
	if notes != 0 {
		t.Fatalf("封顶不应给答主发通知, got %d 条", notes)
	}
}

// 认定奖励：加精与备考经验共用同一笔 +30，每帖一次；文案按认定类型区分。
func TestForumRewardPolicy_AwardDesignationSharesOneBonus(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "desig_owner", "x")
	topic := seedRewardTopic(t, db, owner.ID, "经验帖")
	at := time.Date(2026, 9, 13, 11, 0, 0, 0, clock.Location())
	award := func(designation string) {
		t.Helper()
		if err := db.Transaction(func(tx *gorm.DB) error {
			return policy.Award(tx, forumRewardFact{Kind: forumRewardDesignation, TopicID: topic.ID, TopicTitle: topic.Title, TopicOwner: owner.ID, Designation: designation, At: at})
		}); err != nil {
			t.Fatalf("Award(%s) 失败: %v", designation, err)
		}
	}
	award(DesignationFeatured)
	if got := balanceOf(t, db, owner.ID); got != FeaturedBonusPoints {
		t.Fatalf("加精到账 = %d, want %d", got, FeaturedBonusPoints)
	}
	award(DesignationExperience)
	if got := balanceOf(t, db, owner.ID); got != FeaturedBonusPoints {
		t.Fatalf("认定经验后累计 = %d, want %d（两种认定共用一笔）", got, FeaturedBonusPoints)
	}
	var notes []model.Notification
	db.Where("user_id = ?", owner.ID).Order("id").Find(&notes)
	if len(notes) != 1 || notes[0].Title != "你的帖子被加精" {
		t.Fatalf("站内信 = %d 条, 首条标题 %q（want 1 条「你的帖子被加精」）", len(notes), notes[0].Title)
	}
}

// Reclaim：回收该帖全部直记奖励（答主 + 楼主 + 帖主），封底 0；无奖励时 no-op 不落占坑行。
func TestForumRewardPolicy_ReclaimClawsBackAllDirectRewards(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "reclaim_owner", "x")
	answerer := testutil.SeedStudent(t, db, "reclaim_answerer", "x")
	topic := seedRewardTopic(t, db, owner.ID, "违规帖")
	at := clock.Now()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := policy.Award(tx, forumRewardFact{Kind: forumRewardAccept, TopicID: topic.ID, TopicTitle: topic.Title, TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 5, At: at}); err != nil {
			return err
		}
		return policy.Award(tx, forumRewardFact{Kind: forumRewardDesignation, TopicID: topic.ID, TopicTitle: topic.Title, TopicOwner: owner.ID, Designation: DesignationFeatured, At: at})
	}); err != nil {
		t.Fatalf("发放失败: %v", err)
	}
	if got := balanceOf(t, db, owner.ID); got != AcceptActionPoints+FeaturedBonusPoints {
		t.Fatalf("帖主发放后余额 = %d, want %d", got, AcceptActionPoints+FeaturedBonusPoints)
	}
	clawed, err := reclaimInTx(t, db, policy, topic.ID)
	if err != nil {
		t.Fatalf("Reclaim 失败: %v", err)
	}
	if clawed != AcceptBonusPoints+AcceptActionPoints+FeaturedBonusPoints {
		t.Fatalf("回收分值 = %d, want %d", clawed, AcceptBonusPoints+AcceptActionPoints+FeaturedBonusPoints)
	}
	if got := balanceOf(t, db, answerer.ID); got != 0 {
		t.Fatalf("答主余额应归零, got %d", got)
	}
	if got := balanceOf(t, db, owner.ID); got != 0 {
		t.Fatalf("帖主余额应归零, got %d", got)
	}
	// 二次回收：幂等，余额不再变化
	if _, err := reclaimInTx(t, db, policy, topic.ID); err != nil {
		t.Fatalf("二次 Reclaim 应幂等: %v", err)
	}
	if got := balanceOf(t, db, owner.ID); got != 0 {
		t.Fatalf("二次回收后帖主余额 = %d, want 0", got)
	}
	// 无任何奖励的帖子：no-op，不落占坑行
	bare := seedRewardTopic(t, db, owner.ID, "无奖励帖")
	if _, err := reclaimInTx(t, db, policy, bare.ID); err != nil {
		t.Fatalf("无奖励 Reclaim 应 no-op: %v", err)
	}
	var idem int64
	db.Model(&model.PointsEntryIdem{}).Where("idem_key = ?", ForumRollbackIdemKey(bare.ID)).Count(&idem)
	if idem != 0 {
		t.Fatalf("无奖励帖不应落回收占坑行, got %d", idem)
	}
}

func reclaimInTx(t *testing.T, db *gorm.DB, policy *forumRewardPolicy, topicID int64) (int, error) {
	t.Helper()
	clawed := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		n, err := policy.Reclaim(tx, topicID)
		clawed = n
		return err
	})
	return clawed, err
}

// AcceptRewardIssued：只认采纳类奖励（加精不算），是 reward_issued 的唯一判据。
func TestForumRewardPolicy_AcceptRewardIssuedIgnoresDesignation(t *testing.T) {
	policy, db := newRewardPolicyTest(t)
	owner := testutil.SeedStudent(t, db, "issued_owner", "x")
	answerer := testutil.SeedStudent(t, db, "issued_answerer", "x")
	acceptTopic := seedRewardTopic(t, db, owner.ID, "已采纳")
	desigTopic := seedRewardTopic(t, db, owner.ID, "仅加精")
	at := clock.Now()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := policy.Award(tx, forumRewardFact{Kind: forumRewardAccept, TopicID: acceptTopic.ID, TopicTitle: acceptTopic.Title, TopicOwner: owner.ID, AnswererID: answerer.ID, ReplyID: 3, At: at}); err != nil {
			return err
		}
		return policy.Award(tx, forumRewardFact{Kind: forumRewardDesignation, TopicID: desigTopic.ID, TopicTitle: desigTopic.Title, TopicOwner: owner.ID, Designation: DesignationFeatured, At: at})
	}); err != nil {
		t.Fatalf("发放失败: %v", err)
	}
	got := policy.AcceptRewardIssued(db, []int64{acceptTopic.ID, desigTopic.ID})
	if !got[acceptTopic.ID] {
		t.Fatalf("已采纳帖应判定 reward_issued=true")
	}
	if got[desigTopic.ID] {
		t.Fatalf("仅加精帖不得判定 reward_issued=true（会把「采纳不再产生积分」误报给楼主）")
	}
}
