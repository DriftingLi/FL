// Package service 实现业务服务层。
// 本文件：积分幂等键构造器单点（#608，ADR-0023 幂等占坑）。
// 全仓积分幂等键（points_entry_idem 主键）的格式字符串只在此处存在：直记键与回收键两类
// 构造器，调用侧（points/forum/contribution/checkin 各域）一律经此构造，禁止手拼。
// 键格式与 ADR-0023 收敛落地时的既有格式逐字一致，由 points_idem_keys_test.go 快照钉住；
// 任何格式变更都会改变占坑唯一性（历史键失配 → 双发/双扣），必须是有意的口径决策
// 并同步 CONTEXT.md 登记。
package service

import (
	"fmt"
	"time"
)

// ===== 直记键（赚取/消耗事件，一事件一坑）=====

// RedeemIdemKey 兑换幂等键：`redeem:{sku}`。课程/真题卷/商城三胞胎共用 redeem 单管线
// （CONTEXT.md「积分商城」口径 redeem:<sku>），占坑冲突映射为「已兑换」。
func RedeemIdemKey(sku string) string { return "redeem:" + sku }

// AITokensIdemKey AI 按 tokens 扣费幂等键：`ai_tokens:{requestID}`（CONTEXT.md「AI 计费」）。
// requestID 由调用方传稳定请求标识，同请求重试/重放只扣一次。
func AITokensIdemKey(requestID string) string { return "ai_tokens:" + requestID }

// AcceptedBonusIdemKey 问答采纳答主奖励幂等键：`accepted_bonus:{topicID}`（ADR-0023）。
// 「每帖只发一次」：取消/更换/并发均占同一坑，与状态 CAS 双保险。
func AcceptedBonusIdemKey(topicID int64) string { return fmt.Sprintf("accepted_bonus:%d", topicID) }

// AcceptActionIdemKey 楼主采纳行为奖励幂等键：`accept_action:{topicID}`。
func AcceptActionIdemKey(topicID int64) string { return fmt.Sprintf("accept_action:%d", topicID) }

// FeaturedBonusIdemKey 帖子加精奖励幂等键：`featured_bonus:{topicID}`（#742）。
// 「每帖只发一次」：取消重精/并发均占同一坑，与状态 CAS 双保险（accepted_bonus 同模式）。
func FeaturedBonusIdemKey(topicID int64) string { return fmt.Sprintf("featured_bonus:%d", topicID) }

// ContributionApprovedIdemKey 投稿过审直记幂等键：`contribution_approved:{contributionID}`。
func ContributionApprovedIdemKey(contributionID int64) string {
	return fmt.Sprintf("contribution_approved:%d", contributionID)
}

// ContributionTierIdemKey 投稿达阶奖励幂等键：`contribution_tier:{contributionID}:{threshold}`。
// 键含档位阈值：每档天然只发一次（跨档判定见 Download）。
func ContributionTierIdemKey(contributionID int64, threshold int) string {
	return fmt.Sprintf("contribution_tier:%d:%d", contributionID, threshold)
}

// CheckInIdemKey 每日打卡直记幂等键：`checkin:{uid}:{date}`（CONTEXT.md「每日打卡」）。
// date 为 Asia/Shanghai 自然日 YYYY-MM-DD（经 shanghaiDayStr/clock.DayKey 归一）。
func CheckInIdemKey(userID int, day time.Time) string {
	return fmt.Sprintf("checkin:%d:%s", userID, shanghaiDayStr(day))
}

// ===== 回收键（违规回收对冲，封底 0）=====

// ForumRollbackIdemKey 论坛问答采纳违规回收幂等键：`rollback:{topicID}`（forum_topic 域）。
// 占坑行即「已处理」标记：删帖/违规回收只对冲一次，两次并发删同一帖不双扣。
// 域名入名（#609 review 裁决，原 RollbackIdemKey）：与 ContributionRollbackIdemKey 对称、
// 消除泛化名歧义；键格式逐字不动——回收事件是终态事件，改格式 = 同一事件重放拿到新键。
func ForumRollbackIdemKey(topicID int64) string { return fmt.Sprintf("rollback:%d", topicID) }

// ContributionRollbackIdemKey 投稿下架追回幂等键：`contribution_rollback:{contributionID}`。
// 追回过审分与达阶分合计（封底 0），并发下架/重试只扣一次。
func ContributionRollbackIdemKey(contributionID int64) string {
	return fmt.Sprintf("contribution_rollback:%d", contributionID)
}
