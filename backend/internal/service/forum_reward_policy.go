package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// 论坛奖励政策 module（ADR-0047 §3；spec #927 片一）。
//
// 这个 module 是论坛域**奖励发放与回收**的唯一实现处：发放事实判定、防刷策略求值、
// 幂等占坑、写流水与站内信构造都在 implementation 里，调用方（采纳、认定、撤精、
// 违规删帖）只声明「发生了什么事实」。
//
// interface（见 forumRewardPolicy）只有三个方法：
//   - Award(tx, 事实)：发放一笔直记奖励（采纳或认定）；
//   - Reclaim(tx, topicID)：违规回收该帖全部直记奖励（触发条件在内部判定）；
//   - AcceptRewardIssued(db, topicIDs)：采纳类奖励是否已发放（reward_issued 的唯一判据）。
//
// 数值口径、幂等键、流水 reason、站内信文案与事务边界全部沿用重构前的实现，零漂移。

// 采纳奖励防刷参数（乙档，ADR-0041）。阈值以「已付次数」为基准：配对 0-2 全额、
// 3-4 答主半额、>=5 答主零分；答主当日 >=3 零分、楼主当日 >=5 零分。
const (
	forumAcceptPairHalfAt  int64 = 3 // 配对数达到此值：答主奖励减半
	forumAcceptPairZeroAt  int64 = 5 // 配对数达到此值：答主奖励归零
	forumAcceptAnswererDay int64 = 3 // 答主当日已付笔数达到此值：归零（第 4 次起）
	forumAcceptAskerDayCap int64 = 5 // 楼主当日已付笔数达到此值：归零（第 6 次起）
)

// forumAcceptRewardCounts 防刷求值的输入：三个计数全部是「本次发放之前」的已付次数。
type forumAcceptRewardCounts struct {
	PairPaid    int64 // 该「楼主↔答主」配对已付费次数（事实源＝积分流水，ADR-0041）
	AnswererDay int64 // 答主当日已获采纳奖励笔数
	AskerDay    int64 // 楼主当日采纳动作笔数
}

// forumAcceptRewardDeltas 防刷求值的输出：两个收款人的实际入账分。
type forumAcceptRewardDeltas struct {
	AnswererDelta int // 答主（配对衰减 + 日封顶都作用于它）
	AskerDelta    int // 楼主（只受日封顶约束，不参与配对衰减）
}

// forumAcceptRewardSplit 采纳奖励分档细则（纯函数，internal seam）：
// 配对衰减只作用于答主奖励，楼主采纳动作分不参与衰减；日封顶对双方各自生效。
func forumAcceptRewardSplit(c forumAcceptRewardCounts) forumAcceptRewardDeltas {
	split := forumAcceptRewardDeltas{AnswererDelta: AcceptBonusPoints, AskerDelta: AcceptActionPoints}
	switch {
	case c.PairPaid >= forumAcceptPairZeroAt:
		split.AnswererDelta = 0
	case c.PairPaid >= forumAcceptPairHalfAt:
		split.AnswererDelta /= 2
	}
	if c.AnswererDay >= forumAcceptAnswererDay {
		split.AnswererDelta = 0
	}
	if c.AskerDay >= forumAcceptAskerDayCap {
		split.AskerDelta = 0
	}
	return split
}

// ===== module interface =====

// forumTopicRefType 论坛主题在积分流水上的 ref_type（回收与发放共用）。
const forumTopicRefType = "forum_topic"

// forumTopicDirectRewardReasons 论坛主题上「全部直记奖励」的流水 reason（ADR-0040/0041）。
// 用途：违规回收范围与回收触发条件。**不要**拿它做 reward_issued——那个字段的判据见
// forumTopicAcceptRewardReasons。
var forumTopicDirectRewardReasons = []string{ReasonAcceptedBonus, ReasonAcceptAction, ReasonFeaturedBonus}

// forumTopicAcceptRewardReasons 「采纳类奖励」的流水 reason：答主被采纳 + 楼主采纳动作。
//
// reward_issued 的**唯一**判据（#367）。语义是「该帖的采纳奖励是否已发放」，不是
// 「该帖是否发过任何奖励」——该字段的唯一消费方是采纳前二次确认（文案「该帖采纳奖励已发放……
// 不再产生积分」）。若把 featured_bonus 也算进来，一篇只是被加精、从未被采纳的问答帖会让
// 楼主看到「采纳不再产生积分」，而答主仍会拿到 40 分——错误提示会劝退真实采纳。
// 故本集合**必须小于** forumTopicDirectRewardReasons，两者不可合并。
var forumTopicAcceptRewardReasons = []string{ReasonAcceptedBonus, ReasonAcceptAction}

// forumRewardKind 奖励事实的种类。
type forumRewardKind int

const (
	forumRewardAccept      forumRewardKind = iota // 采纳：答主 + 楼主 双方
	forumRewardDesignation                        // 认定：帖主一次性（加精与备考经验共用同一笔）
)

// forumRewardFact 一次奖励发放的事实。
//
// 调用方只声明「发生了什么」，不拼 delta、不拼 reason、不判幂等——那些都是 implementation。
type forumRewardFact struct {
	Kind       forumRewardKind
	TopicID    int64
	TopicTitle string
	TopicOwner int   // 帖主（认定奖励的收款人；采纳类里是付款的楼主）
	AnswererID int   // 仅采纳类：答主
	ReplyID    int64 // 仅采纳类：站内信锚点
	// Designation 仅认定类：DesignationFeatured | DesignationExperience，决定站内信文案，
	// 不影响流水（两种认定共用同一笔 featured_bonus）。
	Designation string
	// At 评估时刻（注入）：日封顶的「当日」以它为准，产线传真实时钟，测试传固定时刻。
	At time.Time
}

// forumRewardPolicy 论坛奖励政策 module（interface 见文件头注释）。
type forumRewardPolicy struct {
	points        *PointsService
	notifications *NotificationService
}

func newForumRewardPolicy(points *PointsService, notifications *NotificationService) *forumRewardPolicy {
	return &forumRewardPolicy{points: points, notifications: notifications}
}

// Award 发放一笔直记奖励（调用方事务内执行；站内信与入账同事务）。
func (p *forumRewardPolicy) Award(tx *gorm.DB, fact forumRewardFact) error {
	switch fact.Kind {
	case forumRewardAccept:
		return p.awardAccept(tx, fact)
	case forumRewardDesignation:
		return p.awardDesignation(tx, fact)
	default:
		return fmt.Errorf("未知的奖励事实种类: %d", fact.Kind)
	}
}

// awardAccept 采纳奖励：答主 + 楼主。每帖只发一次（判据 = accepted_bonus 流水是否已存在）；
// 发送前按防刷策略求值（配对衰减 + 日封顶），双方都为零时不写任何流水与通知。
func (p *forumRewardPolicy) awardAccept(tx *gorm.DB, fact forumRewardFact) error {
	issued, err := p.issued(tx, fact.TopicID, []string{ReasonAcceptedBonus})
	if err != nil {
		return err
	}
	if issued {
		// 取消后重采：状态迁移由调用方完成，奖励不再发第二次
		return nil
	}
	counts := forumAcceptRewardCounts{}
	// 配对次数的事实源是 points_ledger 本身（ADR-0041），不是当前挂着的 accepted_reply_id：
	// 状态列可被取消采纳与删帖回退，拿它计数等于给配对衰减留了重置开关。
	// 同一 topic 的两条流水——accepted_bonus 记在答主、accept_action 记在楼主——按 ref_id
	// 自连接即还原「楼主↔答主」配对；口径是「**付过钱的**配对次数」。
	if err := tx.Raw("SELECT COUNT(*) FROM points_ledger a "+
		"JOIN points_ledger b ON b.ref_type = a.ref_type AND b.ref_id = a.ref_id "+
		"WHERE a.ref_type = ? AND a.reason = ? AND a.user_id = ? "+
		"AND b.reason = ? AND b.user_id = ?",
		forumTopicRefType, ReasonAcceptedBonus, fact.AnswererID, ReasonAcceptAction, fact.TopicOwner).
		Scan(&counts.PairPaid).Error; err != nil {
		return err
	}
	todayStart := clock.DayStart(fact.At)
	if err := tx.Model(&model.PointsLedger{}).
		Where("user_id = ? AND reason = ? AND created_at >= ?", fact.AnswererID, ReasonAcceptedBonus, todayStart).
		Count(&counts.AnswererDay).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.PointsLedger{}).
		Where("user_id = ? AND reason = ? AND created_at >= ?", fact.TopicOwner, ReasonAcceptAction, todayStart).
		Count(&counts.AskerDay).Error; err != nil {
		return err
	}
	split := forumAcceptRewardSplit(counts)
	if split.AnswererDelta == 0 && split.AskerDelta == 0 {
		return nil
	}
	refID := forumTopicRefID(fact.TopicID)
	if split.AnswererDelta > 0 {
		// 占坑键与状态 CAS 双保险「每帖只发一次」（ADR-0023）
		if err := p.points.SettleRewardTx(tx, PointsEntry{
			UserID: fact.AnswererID, Delta: split.AnswererDelta, Reason: ReasonAcceptedBonus,
			RefType: forumTopicRefType, RefID: refID, IdemKey: AcceptedBonusIdemKey(fact.TopicID),
		}); err != nil {
			return err
		}
		if err := p.notifications.CreateForumAcceptEvent(tx,
			NewAnswererAcceptEvent(fact.AnswererID, fact.TopicTitle, fact.TopicID, fact.ReplyID, split.AnswererDelta), fact.At); err != nil {
			return err
		}
	}
	if split.AskerDelta > 0 {
		if err := p.points.SettleRewardTx(tx, PointsEntry{
			UserID: fact.TopicOwner, Delta: split.AskerDelta, Reason: ReasonAcceptAction,
			RefType: forumTopicRefType, RefID: refID, IdemKey: AcceptActionIdemKey(fact.TopicID),
		}); err != nil {
			return err
		}
		if err := p.notifications.CreateForumAcceptEvent(tx,
			NewOwnerAcceptEvent(fact.TopicOwner, fact.TopicTitle, fact.TopicID, fact.ReplyID, split.AskerDelta), fact.At); err != nil {
			return err
		}
	}
	return nil
}

// awardDesignation 认定奖励（加精 / 认定备考经验**共用同一笔**，ADR-0040）：每帖一次性
// 直记 featured_bonus，以流水存在判定幂等（取消重精、先精后认定、先认定后精都只发一次）。
// 流水 reason 与幂等键格式逐字不动（改格式 = 同一事件重放拿到新键 → 双重发分/双重追回）。
func (p *forumRewardPolicy) awardDesignation(tx *gorm.DB, fact forumRewardFact) error {
	issued, err := p.issued(tx, fact.TopicID, []string{ReasonFeaturedBonus})
	if err != nil {
		return err
	}
	if issued {
		return nil
	}
	if err := p.points.SettleRewardTx(tx, PointsEntry{
		UserID: fact.TopicOwner, Delta: FeaturedBonusPoints, Reason: ReasonFeaturedBonus,
		RefType: forumTopicRefType, RefID: forumTopicRefID(fact.TopicID), IdemKey: FeaturedBonusIdemKey(fact.TopicID),
	}); err != nil {
		return err
	}
	// 两种认定共用同一笔流水，但文案必须区分（ADR-0040）。
	if fact.Designation == DesignationExperience {
		return p.notifications.CreateTopicFeaturedEvent(tx,
			NewTopicExperienceEvent(fact.TopicOwner, fact.TopicTitle, fact.TopicID, FeaturedBonusPoints), fact.At)
	}
	return p.notifications.CreateTopicFeaturedEvent(tx,
		NewTopicFeaturedEvent(fact.TopicOwner, fact.TopicTitle, fact.TopicID, FeaturedBonusPoints), fact.At)
}

// Reclaim 违规回收该帖的**全部直记奖励**（答主 + 楼主 + 帖主，含认定奖励）：
// 触发条件（该帖存在任一正向直记奖励）与回收范围都在内部判定，无奖励可回收时 no-op
// （不写占坑行，与重构前逐字一致）。
//
// 护栏说明：RollbackByRef 的幂等是 **ref 级一次性**——一个 ref 只能有一次回收事件。
// 故调用方不得在「删回复」等中间动作上调用本方法，否则会永久占掉该帖的回收机会。
func (p *forumRewardPolicy) Reclaim(tx *gorm.DB, topicID int64) (int, error) {
	hasReward, err := p.hasPositiveReward(tx, topicID)
	if err != nil {
		return 0, err
	}
	if !hasReward {
		return 0, nil
	}
	clawed, err := p.points.RollbackByRef(tx, PointsRollback{
		RefType: forumTopicRefType,
		RefID:   forumTopicRefID(topicID),
		Reasons: forumTopicDirectRewardReasons,
		IdemKey: ForumRollbackIdemKey(topicID),
	})
	if errors.Is(err, ErrPointsProcessed) {
		// 占坑冲突（已回收过）按论坛语义静默放行：删帖动作不因重复回收失败
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return clawed, nil
}

// AcceptRewardIssued 批量判定「该帖的采纳奖励是否已发放」（reward_issued 的唯一判据）。
// 与写入侧共用同一份 reason 集合，杜绝两处实现漂移（历史上漂移过一次就是 bug）。
func (p *forumRewardPolicy) AcceptRewardIssued(db *gorm.DB, topicIDs []int64) map[int64]bool {
	out := make(map[int64]bool, len(topicIDs))
	if len(topicIDs) == 0 {
		return out
	}
	ids := make([]string, 0, len(topicIDs))
	seen := make(map[string]struct{}, len(topicIDs))
	for _, id := range topicIDs {
		sid := forumTopicRefID(id)
		if _, ok := seen[sid]; !ok {
			seen[sid] = struct{}{}
			ids = append(ids, sid)
		}
	}
	var issued []string
	if err := db.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND reason IN ? AND ref_id IN ?", forumTopicRefType, forumTopicAcceptRewardReasons, ids).
		Distinct("ref_id").Pluck("ref_id", &issued).Error; err != nil {
		return out
	}
	for _, sid := range issued {
		if id, err := strconv.ParseInt(sid, 10, 64); err == nil {
			out[id] = true
		}
	}
	return out
}

// issued 发放事实判定：该 ref 上是否已存在指定 reason 的流水（**流水即事实源**，ADR-0041）。
// 这是「是否已发过分」的唯一实现——重构前它在四处被独立重写。
func (p *forumRewardPolicy) issued(tx *gorm.DB, topicID int64, reasons []string) (bool, error) {
	var cnt int64
	if err := tx.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND ref_id = ? AND reason IN ?", forumTopicRefType, forumTopicRefID(topicID), reasons).
		Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// hasPositiveReward 该帖是否产生过任一直记奖励（正向流水）——违规回收的触发条件。
// 只认正向流水，回收本身写的 reason=rollback 负向流水不会被误判。
func (p *forumRewardPolicy) hasPositiveReward(tx *gorm.DB, topicID int64) (bool, error) {
	var n int64
	if err := tx.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND ref_id = ? AND delta > 0 AND reason IN ?",
			forumTopicRefType, forumTopicRefID(topicID), forumTopicDirectRewardReasons).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// forumTopicRefID 主题在流水上的 ref_id（整数转字符串，单点）。
func forumTopicRefID(topicID int64) string { return strconv.FormatInt(topicID, 10) }
