// 内容精选的「发布时间 / 定时发布」语义（#1748，机制与被否备选见 ADR-0076）。
//
// 本文件是语义矩阵**非 nil 入参那一格**的唯一实现：创建 / 更新 / 发布三条写路径都经
// applyFeaturedPublishTime 落定「状态 / 发布时间 / 定时时间」，而不是三处各写一遍 if ——
// 三份实现必然漂移，而这三格正是「读者看到的时间」与「访客能不能看见」的全部事实。
//
// 「入参为 nil」那一格**刻意留在各调用处**：它不是同一件事的三份写法，而是三件不同的事
// （Create 首次落态、Update 只改正文保留排期 / 点名状态清排期、Publish 已发布时幂等）。
package featured

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// 发布时间相关的三条「输入不合法」事实（ADR-0064 决策 3）：各自具名 ⇒ 由 handler 落 400。
// 此前本域只有三条同类事实（标题/分类/图片），发布时间校验若写裸 errors.New 会落端点默认面 500。
var (
	// ErrFeaturedPublishTimeInvalid 发布时间格式或范围不合法。
	ErrFeaturedPublishTimeInvalid = errors.New("发布时间无效（需为 RFC3339，且不早于 2000-01-01、不晚于当前时间 10 年以后）")
	// ErrFeaturedDraftWithPublishTime 草稿却携带了发布时间（编辑器不会发这个组合）。
	ErrFeaturedDraftWithPublishTime = errors.New("草稿不能指定发布时间（请先选「发布」再填时间）")
	// ErrFeaturedStateInvalid 管理端列表的状态过滤取值不合法。
	ErrFeaturedStateInvalid = errors.New("状态过滤无效（可选 draft / scheduled / published）")
)

// 管理端列表的三态过滤取值（#1748）。「定时中」不是一个 status 值，而是
// 「status = 0 且 scheduled_at 非空」这一组合 —— 过滤条件因此也按组合写。
const (
	featuredStateDraft     = "draft"
	featuredStateScheduled = "scheduled"
	featuredStatePublished = "published"
)

// featuredPublishTimeFloor 发布时间下限：2000-01-01（业务时区的零点）。
// 用业务时区而不是 UTC：编辑器提交的是 +08:00 写法，用 UTC 会把 2000-01-01 当天的
// 一部分合法时间判成越界（那正是「选了合法时间却 400」的形态）。
var featuredPublishTimeFloor = time.Date(2000, 1, 1, 0, 0, 0, 0, clock.Location())

// featuredPublishPlan 一次写库落定的三字段（语义矩阵的输出）。
type featuredPublishPlan struct {
	Status      int16
	PublishedAt *time.Time
	ScheduledAt *time.Time
}

// validateFeaturedState 校验管理端列表的 state 取值；空串表示「全部」。
func validateFeaturedState(state string) error {
	switch state {
	case "", featuredStateDraft, featuredStateScheduled, featuredStatePublished:
		return nil
	default:
		return ErrFeaturedStateInvalid
	}
}

// parseFeaturedPublishTime 解析编辑器提交的发布时间。
// 空串是「现在」的显式写法（编辑器「留空 = 保存后立即发布」），不是「清空」。
func parseFeaturedPublishTime(raw string, now time.Time) (time.Time, error) {
	if raw == "" {
		return now.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, ErrFeaturedPublishTimeInvalid
	}
	if t.Before(featuredPublishTimeFloor) || t.After(now.AddDate(10, 0, 0)) {
		return time.Time{}, ErrFeaturedPublishTimeInvalid
	}
	// 存的是**时刻**，统一折成 UTC 落库：Postgres 的 timestamptz 本来就按时刻存、与偏移无关；
	// 而 SQLite 把时间存成带偏移的文本、按字符串比较 —— 同一列里混着 `+08:00` 与 `+00:00`
	// 会让 `scheduled_at <= ?` 在某些时区下静默漏行（CI 跑在 UTC 上，本机在东八区）。
	return t.UTC(), nil
}

// applyFeaturedPublishTime 语义矩阵（ADR-0076「写入语义」表）：
//
//	入参时间  目标状态  结果
//	≤ now     1        已发布：published_at = t（补录，时间轴按 t 排）
//	> now     1        定时中：status = 0、scheduled_at = t、published_at = 空
//	任意      0        400（草稿不排期）
//
// 入参为 nil（**未携带**）不归本函数管 —— 那是「维持调用方既有语义」的一格（发布补当前时间、
// 草稿清排期），由 Create / Update / Publish 各自兜住，见各处注释。
func applyFeaturedPublishTime(targetStatus int16, raw string, now time.Time) (featuredPublishPlan, error) {
	if targetStatus == 0 {
		return featuredPublishPlan{}, ErrFeaturedDraftWithPublishTime
	}
	t, err := parseFeaturedPublishTime(raw, now)
	if err != nil {
		return featuredPublishPlan{}, err
	}
	if t.After(now) {
		return featuredPublishPlan{Status: 0, ScheduledAt: &t}, nil
	}
	return featuredPublishPlan{Status: 1, PublishedAt: &t}, nil
}

// PublishDueScheduled 把已到点的定时内容落成已发布，返回本次补发条数（#1748）。
//
// 一条幂等 UPDATE：`status = 0 AND scheduled_at IS NOT NULL` 这两半与部分索引
// idx_featured_content_scheduled_at 的谓词同形（索引侧没有 `<= now` 与 status 的时间条件 ——
// 时间是一个范围、索引谓词只能是常量形状），故扫的是「有排期的那一小块」。
// Postgres 的 SET 右侧取**旧行值**，故同一句里「published_at := scheduled_at」与
// 「scheduled_at := NULL」互不干扰；`status = 0` 的条件保证多实例 / 重跑都不会重复发布。
// 守护停摆不会丢排期：下一轮按 `scheduled_at <= now` 全量补发（#1748 用户故事 18/19）。
func (s *Service) PublishDueScheduled(ctx context.Context, now time.Time) (int64, error) {
	// 参数同样折成 UTC：与写入侧同一口径（见 parseFeaturedPublishTime 的注释）。
	res := s.db.WithContext(ctx).Model(&model.FeaturedContent{}).
		Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?", 0, now.UTC()).
		Updates(map[string]any{
			"status":       int16(1),
			"published_at": gorm.Expr("scheduled_at"),
			"scheduled_at": nil,
			"updated_at":   now,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
