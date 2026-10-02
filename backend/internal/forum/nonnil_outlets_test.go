// Package forum 域内 nonnil 证据表（ADR-0070 决策 9：证据表住域包；
// 跨包「一条事实一个证据」由 internal/apitypes/nullability_lock_test.go 钉住）。
package forum

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// 本域出口两种形状（承接原 internal/service/nonnil_outlets_people_test.go 的判据）：
//   - 分页信封（topics / replies / reports）：空库直接跑，量到的就是 make(…,0,0) 发出的 []；
//   - 住在列表条目里的数组（images 三格）：testutil.MarshalKey 只看顶层键，所以出口返回那条条目本身，
//     播的行不带图 —— 要证的恰恰是「这一格空着时发的是什么」。
var nonnilOutletsForum = map[string]func(t *testing.T) any{
	// ===== 论坛读面（images 三格走 imageURLsForWire 归一，ADR-0062 决策 12）=====
	"forum.ForumTopicPageResult.topics":   outletForumTopicPageEmpty,
	"forum.ForumTopicDTO.images":          outletForumTopicNoImages,
	"forum.ForumTopicDetailDTO.replies":   outletForumTopicDetailNoReplies,
	"forum.ForumReplyDTO.images":          outletForumReplyNoImages,
	"forum.MyReplyPageResult.replies":     outletMyReplyPageEmpty,
	"forum.MyReplyDTO.images":             outletMyReplyNoImages,
	"forum.ForumReportPageResult.reports": outletForumReportPageEmpty,
}

func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsForum})
}

// outletForumTopicPageEmpty 主题列表：一行帖子都没有时 topics 仍是 make 出来的空集。
func outletForumTopicPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListTopics(TopicListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("空库拉主题列表失败: %v", err)
	}
	return res
}

// outletForumTopicNoImages 主题条目里的 images：帖子的 images 列为 NULL（发帖未带图）时，
// 读面出口经 imageURLsForWire 归一成 [] —— 这条正是「契约说数组、出口却发 null」那个坑的正面证据。
func outletForumTopicNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	author := seedForumUser(t, db, "无图楼主")
	topic := seedRewardTopic(t, db, author.ID, "无图主题")
	svc := NewService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListTopics(TopicListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("列表失败: %v", err)
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
	svc := NewService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.GetTopic(TopicDetailInput{TopicID: topic.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("主题详情失败: %v", err)
	}
	return res
}

// outletForumReplyNoImages 回复条目里的 images：replyRow.toDTO 也走 imageURLsForWire，
// 故回复这一格单独占一键、各自 marshal。
func outletForumReplyNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	replier := seedForumUser(t, db, "无图回复人")
	topic := seedRewardTopic(t, db, replier.ID, "被回复的主题")
	seedPlainReply(t, db, topic.ID, replier.ID)
	svc := NewService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.GetTopic(TopicDetailInput{TopicID: topic.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("主题详情失败: %v", err)
	}
	if len(res.Replies) == 0 {
		t.Fatalf("详情里没取到刚播的回复：出口取不到 ForumReplyDTO，这条证据没有落地")
	}
	return res.Replies[0]
}

// outletMyReplyPageEmpty 我的回复：一条回复都没有时 replies 仍是空集。
func outletMyReplyPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.MyReplies(1, 1, 20)
	if err != nil {
		t.Fatalf("空回复列表失败: %v", err)
	}
	return res
}

// outletMyReplyNoImages 我的回复条目里的 images（MyReplyDTO 走同一条归一）。
func outletMyReplyNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	replier := seedForumUser(t, db, "我的无图回复人")
	topic := seedRewardTopic(t, db, replier.ID, "我的回复所属主题")
	seedPlainReply(t, db, topic.ID, replier.ID)
	svc := NewService(db, nil, nil, nil, nil, zap.NewNop())
	res, err := svc.MyReplies(replier.ID, 1, 20)
	if err != nil {
		t.Fatalf("我的回复列表失败: %v", err)
	}
	if len(res.Replies) == 0 {
		t.Fatalf("我的回复里没取到刚播的那条：出口取不到 MyReplyDTO，这条证据没有落地")
	}
	return res.Replies[0]
}

// outletForumReportPageEmpty 举报列表（管理端）：空库时 reports 仍是空集。
func outletForumReportPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewModerationService(testutil.NewMemoryDB(t), nil, nil, nil, nil, zap.NewNop())
	res, err := svc.ListReports(1, 20, nil)
	if err != nil {
		t.Fatalf("空举报列表失败: %v", err)
	}
	return res
}

// seedPlainReply 播一条**不带图**的回复（images 列留空），返回其行。
func seedPlainReply(t *testing.T, db *gorm.DB, topicID int64, userID int) *model.ForumReply {
	t.Helper()
	reply := model.ForumReply{TopicID: topicID, UserID: userID, Content: "无图回复正文", ContentFormat: ForumContentFormatText, CreatedAt: testutil.Now()}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("播回复失败: %v", err)
	}
	return &reply
}
