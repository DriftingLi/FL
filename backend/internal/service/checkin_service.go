// Package service 实现业务服务层。
// 本文件：每日打卡（签到/日历/连击/排行榜/直记积分，Asia/Shanghai 自然日语义）。
// 由 ForumService 拆出为独立 module（spec #279），与 ForumService 共享 ForumAuthor seam；
// 路由前缀 /api/check-in/*（ADR-0028：打卡从论坛域迁出为独立模块，旧 /api/forum/check-in/* 已删除）。
// 时间统一经 internal/clock 构造注入（spec #296）：生产为 Asia/Shanghai 实钟，测试可定格。
package service

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

const (
	// checkInWindowDays streak 统一截断窗口（天）：排行榜条目与 Me 名次口径一致，
	// 行数上界由 O(total_days) 降为常数（spec #296）。
	checkInWindowDays = 400

	// checkInRankOrderBy 排行榜排序 tie-break 单点定义：
	// 分页查询与 Me 名次合并查询共用同一口径，禁止在调用处内联重复书写。
	checkInRankOrderBy = "total DESC, last_date ASC, user_id ASC"

	// checkInAggSubquery 用户维度打卡聚合子查询：分页、总数与 Me 名次合并共用同一来源。
	checkInAggSubquery = "SELECT user_id, COUNT(*) AS total, MAX(check_date) AS last_date FROM forum_checkin GROUP BY user_id"

	// checkInMeRankSQL 当前用户名次：RANK() OVER 复用 checkInRankOrderBy，
	// 与分页 ORDER BY 完全同口径（含并列名次语义）。
	checkInMeRankSQL = "SELECT rk FROM (" +
		"SELECT user_id, RANK() OVER (ORDER BY " + checkInRankOrderBy + ") AS rk " +
		"FROM (" + checkInAggSubquery + ") AS agg) AS ranked WHERE user_id = ?"

	// ===== 打卡积分（ADR-0028：打卡从任务中心剥离，直记发分）=====
	// checkInBasePoints 每日打卡基础分。
	checkInBasePoints = 5
	// checkInReason 打卡流水 reason（ref_type=checkin，ref_id=日期 YYYY-MM-DD）。
	checkInReason = "checkin"
	// checkInRefType 打卡流水业务域。
	checkInRefType = "checkin"
)

// checkInTierBonus 连续签到阶梯奖：连击满 3/7/30 天当日额外 +5/+10/+50。
// 每个连续段都重新发（断签清零、无补签）；恰好跨档触发一次，与基础分合并单笔流水。
var checkInTierBonus = map[int]int{3: 5, 7: 10, 30: 50}

// CheckInResult 打卡结果。
type CheckInResult struct {
	Checked      bool `json:"checked"`
	Streak       int  `json:"streak"`
	Total        int  `json:"total"`
	TodayChecked bool `json:"today_checked"`
	// Points 今日实发积分（基础 + 跨档阶梯，合并单笔；已打卡/重复请求时为 0）。
	Points int `json:"points"`
}

// CheckInDay 日历单日：日期、是否已打卡、当日实发积分（无流水的历史打卡为 0）。
type CheckInDay struct {
	Date    string `json:"date"`
	Checked bool   `json:"checked"`
	Points  int    `json:"points"`
}

// CheckInCalendarResult 日历结果。
type CheckInCalendarResult struct {
	Days         []CheckInDay `json:"days"`
	Streak       int          `json:"streak"`
	Total        int          `json:"total"`
	TodayChecked bool         `json:"today_checked"`
}

// CheckInRankItem 排行榜条目。
type CheckInRankItem struct {
	Rank         int         `json:"rank"`
	User         ForumAuthor `json:"user"`
	Total        int         `json:"total"`
	Streak       int         `json:"streak"`
	TodayChecked bool        `json:"today_checked"`
}

// CheckInRankResult 排行榜分页结果。
type CheckInRankResult struct {
	Items []CheckInRankItem `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Pages int               `json:"pages"`
	Me    *CheckInRankItem  `json:"me"`
}

// CheckInService 每日打卡服务（独立 module，与论坛帖子/回复逻辑解耦）。
type CheckInService struct {
	db     *gorm.DB
	logger *zap.Logger
	clk    clock.Clock
	// points 积分簿记通道（ADR-0028）：打卡直记发分经 SettleRewardTx 同事务落账。
	points *PointsService
}

// NewCheckInService 构造打卡服务；clk 为空时回退生产实钟（Asia/Shanghai）。
// points 为打卡积分簿记通道（打卡即发分，ADR-0028）；可为 nil（测试或未接线时仅记录不发分）。
func NewCheckInService(db *gorm.DB, logger *zap.Logger, clk clock.Clock, points *PointsService) *CheckInService {
	if clk == nil {
		clk = clock.Real()
	}
	return &CheckInService{db: db, logger: logger, clk: clk, points: points}
}

// CheckInTierBonusFor 纯函数：由（跨档后）连续天数计算今日阶梯额外奖励。
// 恰好落在阶梯档位（3/7/30）返回对应额外分，否则 0。每个连续段重新发。
func CheckInTierBonusFor(streak int) int {
	return checkInTierBonus[streak]
}

// startOfShanghaiDay 返回 t 在业务时区（Asia/Shanghai）的自然日起点 00:00。
// 实现委托 clock.DayStart（ADR-0027 自然日边界单点收编）。
func startOfShanghaiDay(t time.Time) time.Time {
	return clock.DayStart(t)
}

// shanghaiDayStr 归一化为业务时区日期字符串，避免 UTC 偏移导致跨日错位。
// 实现委托 clock.DayKey（ADR-0027）。
func shanghaiDayStr(t time.Time) string {
	return clock.DayKey(t)
}

// ComputeStreakMetrics 纯函数：由已签日期集合计算连击指标。
// dates 无序、可含重复，内部按业务时区日期归一化去重；now 为“当前时刻”。
// todayChecked 判定今日；streak 从今日（已签）或昨日（未签）起点连续回溯，
// 起点缺失则 streak=0。若调用方传入窗口截断后的日期（如近 400 天），
// 返回的 streak/total 即窗口内口径——窗口截断由查询侧负责（见 streakInWindow）。
func ComputeStreakMetrics(dates []time.Time, now time.Time) (streak, total int, todayChecked bool) {
	total = len(dates)
	if total == 0 {
		return 0, 0, false
	}
	set := make(map[string]bool, total)
	for _, d := range dates {
		set[shanghaiDayStr(d)] = true
	}
	today := startOfShanghaiDay(now)
	todayChecked = set[shanghaiDayStr(today)]
	var cur time.Time
	if todayChecked {
		cur = today
	} else {
		cur = today.AddDate(0, 0, -1)
		if !set[shanghaiDayStr(cur)] {
			return 0, total, false
		}
	}
	for set[shanghaiDayStr(cur)] {
		streak++
		cur = cur.AddDate(0, 0, -1)
	}
	return streak, total, todayChecked
}

// windowCutoff streak 截断窗口起点（今日 −checkInWindowDays 天），各统计路径共用。
func (s *CheckInService) windowCutoff(now time.Time) time.Time {
	return startOfShanghaiDay(now).AddDate(0, 0, -checkInWindowDays)
}

// checkInDates 取用户 ≤ today 且 ≥ cutoff 的签到日期（连击判定共用取数路径：排行榜窗口口径
// 与签到发分判档同源，避免「全量 vs 400 天窗口」双实现口径分叉）。
// db 参数兼容事务内（tx）与直连（s.db）两种调用点。
func checkInDates(db *gorm.DB, userID int, now time.Time, order string) ([]time.Time, error) {
	var dates []time.Time
	err := db.Model(&model.ForumCheckIn{}).
		Where("user_id = ? AND check_date >= ? AND check_date <= ?", userID, startOfShanghaiDay(now).AddDate(0, 0, -checkInWindowDays), startOfShanghaiDay(now)).
		Order(order).Pluck("check_date", &dates).Error
	return dates, err
}

// streakInWindow 计算窗口内连击与今日已签（排行榜条目与 Me 共用同一截断口径）。
func (s *CheckInService) streakInWindow(userID int, now time.Time) (streak int, todayChecked bool) {
	dates, err := checkInDates(s.db, userID, now, "check_date ASC")
	if err != nil {
		return 0, false
	}
	streak, _, todayChecked = ComputeStreakMetrics(dates, now)
	return streak, todayChecked
}

// checkInStats 连击 + 全量累计天数 + 今日已签（签到/日历统计共用）。
// total 为全生命周期计数（不受 400 天窗口截断），与响应契约保持一致；
// streak/todayChecked 为窗口内口径。
func (s *CheckInService) checkInStats(userID int, now time.Time) (streak, total int, todayChecked bool) {
	var total64 int64
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", userID).Count(&total64).Error; err != nil {
		return 0, 0, false
	}
	streak, todayChecked = s.streakInWindow(userID, now)
	return streak, int(total64), todayChecked
}

// CheckIn 每日打卡（幂等，Asia/Shanghai 自然日）；首次签到即发基础分 + 跨档阶梯分
// （合并单笔直记，幂等键 checkin:{uid}:{date}——同日重复/并发只发一次，ADR-0028）。
func (s *CheckInService) CheckIn(userID int) (*CheckInResult, error) {
	now := s.clk.Now()
	today := startOfShanghaiDay(now)
	awarded := 0
	first := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date = ?", userID, today).Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			if err := tx.Create(&model.ForumCheckIn{UserID: userID, CheckDate: today, CreatedAt: now}).Error; err != nil {
				// 唯一冲突视为已签（并发幂等，共享 isDuplicateError 谓词）
				if !isDuplicateError(err) {
					return err
				}
			} else {
				first = true
			}
		}
		if first && s.points != nil {
			// 首次签到才发分（重复打卡不发）。跨档当日的阶梯奖随基础分合并一笔；
			// streak 取签到后的窗口口径（400 天截断与排行榜同源，见 checkInDates）。
			dates, err := checkInDates(tx, userID, now, "check_date ASC")
			if err != nil {
				return err
			}
			streak, _, _ := ComputeStreakMetrics(dates, now)
			bonus := CheckInTierBonusFor(streak)
			delta := checkInBasePoints + bonus
			if err := s.points.SettleRewardTx(tx, PointsEntry{
				UserID: userID, Delta: delta, Reason: checkInReason,
				RefType: checkInRefType, RefID: shanghaiDayStr(today),
				IdemKey: fmt.Sprintf("checkin:%d:%s", userID, shanghaiDayStr(today)),
			}); err != nil {
				return err
			}
			awarded = delta
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	streak, total, todayChecked := s.checkInStats(userID, now)
	return &CheckInResult{Checked: true, Streak: streak, Total: total, TodayChecked: todayChecked, Points: awarded}, nil
}

// GetCheckInCalendar 获取某月已签日期及统计（逐日带实发积分 points）。
func (s *CheckInService) GetCheckInCalendar(userID, year, month int) (*CheckInCalendarResult, error) {
	if year < 2000 || year > 2100 {
		return nil, errors.New("年份无效")
	}
	if month < 1 || month > 12 {
		return nil, errors.New("月份无效")
	}
	loc := clock.Location()
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	// last day: first of next month -1 day
	last := first.AddDate(0, 1, -1)
	var dates []time.Time
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date BETWEEN ? AND ?", userID, first, last).Order("check_date ASC").Pluck("check_date", &dates).Error; err != nil {
		return nil, err
	}
	// 当日实发积分（流水 fact：ref_type=checkin + ref_id=日期）。批量查询避免 N+1。
	type ledgerRow struct {
		RefID string
		Delta int
	}
	var ledgers []ledgerRow
	if len(dates) > 0 {
		refIDs := make([]string, 0, len(dates))
		for _, d := range dates {
			refIDs = append(refIDs, shanghaiDayStr(d))
		}
		if err := s.db.Model(&model.PointsLedger{}).
			Where("user_id = ? AND ref_type = ? AND ref_id IN ?", userID, checkInRefType, refIDs).
			Select("ref_id, delta").Scan(&ledgers).Error; err != nil {
			return nil, err
		}
	}
	pointsByDate := make(map[string]int, len(ledgers))
	for _, l := range ledgers {
		pointsByDate[l.RefID] = l.Delta
	}
	checkedByDate := make(map[string]bool, len(dates))
	for _, d := range dates {
		checkedByDate[shanghaiDayStr(d)] = true
	}
	// 契约：逐日返回整月每一天 {date, checked, points}（未打卡日 checked=false、points=0），
	// 供日历渲染全月格；跨时区一致性由 Asia/Shanghai 承载。
	days := make([]CheckInDay, 0, last.Day())
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		key := shanghaiDayStr(d)
		days = append(days, CheckInDay{Date: key, Checked: checkedByDate[key], Points: pointsByDate[key]})
	}
	streak, total, todayChecked := s.checkInStats(userID, s.clk.Now())
	return &CheckInCalendarResult{Days: days, Streak: streak, Total: total, TodayChecked: todayChecked}, nil
}

// aggRow 排行榜聚合行（user 维度 total/last_date）。
type aggRow struct {
	UserID   int    `gorm:"column:user_id"`
	Total    int    `gorm:"column:total"`
	LastDate string `gorm:"column:last_date"`
}

// GetCheckInRank 排行榜（累计总榜，tie-break 见 checkInRankOrderBy 单点定义）。
func (s *CheckInService) GetCheckInRank(requesterID, page, pageSize int) (*CheckInRankResult, error) {
	now := s.clk.Now()
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var totalUsers int64
	if err := s.db.Table("(" + checkInAggSubquery + ") AS agg").Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	pages := response.PageCount(totalUsers, pageSize)
	if page > pages && pages > 0 {
		page = pages
	}
	offset := (page - 1) * pageSize
	var rows []aggRow
	if err := s.db.Table("(" + checkInAggSubquery + ") AS agg").
		Select("user_id, total, last_date").
		Order(checkInRankOrderBy).
		Limit(pageSize).Offset(offset).Scan(&rows).Error; err != nil {
		return nil, err
	}
	// 批量取用户信息
	userIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
	}
	userMap := make(map[int]model.HrwaiUser)
	if len(userIDs) > 0 {
		var users []model.HrwaiUser
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}
	// 批量取打卡日期（一次查询取齐本页所有用户的 streak/todayChecked，消除随总天数线性增长的 N+1；
	// 限定至近 checkInWindowDays 天窗口，行数上界为常数；Me 名次与本页条目共用同一截断口径）
	cutoff := s.windowCutoff(now)
	type dateRow struct {
		UserID    int       `gorm:"column:user_id"`
		CheckDate time.Time `gorm:"column:check_date"`
	}
	var allDates []dateRow
	if len(userIDs) > 0 {
		_ = s.db.Model(&model.ForumCheckIn{}).Select("user_id, check_date").Where("user_id IN ? AND check_date >= ?", userIDs, cutoff).Order("check_date ASC").Find(&allDates).Error
	}
	grouped := make(map[int][]time.Time, len(userIDs))
	for _, dr := range allDates {
		grouped[dr.UserID] = append(grouped[dr.UserID], dr.CheckDate)
	}
	items := make([]CheckInRankItem, 0, len(rows))
	for i, r := range rows {
		u := userMap[r.UserID]
		streak, _, todayChecked := ComputeStreakMetrics(grouped[r.UserID], now)
		items = append(items, CheckInRankItem{
			Rank:         offset + i + 1,
			User:         ForumAuthor{UserID: r.UserID, Username: u.Username, AvatarURL: u.AvatarURL},
			Total:        r.Total,
			Streak:       streak,
			TodayChecked: todayChecked,
		})
	}
	// 当前用户排名（若不在当页仍需计算）；名次经 RANK() OVER 复用 checkInRankOrderBy 同口径
	var me *CheckInRankItem
	if requesterID > 0 {
		var myRow aggRow
		err := s.db.Table("("+checkInAggSubquery+") AS agg").
			Select("user_id, total, last_date").
			Where("user_id = ?", requesterID).Scan(&myRow).Error
		if err == nil && myRow.Total > 0 {
			var myRank int
			if err := s.db.Raw(checkInMeRankSQL, requesterID).Scan(&myRank).Error; err != nil {
				return nil, err
			}
			var mu model.HrwaiUser
			_ = s.db.First(&mu, requesterID).Error
			streak, todayChecked := s.streakInWindow(requesterID, now)
			me = &CheckInRankItem{
				Rank:         myRank,
				User:         ForumAuthor{UserID: requesterID, Username: mu.Username, AvatarURL: mu.AvatarURL},
				Total:        myRow.Total,
				Streak:       streak,
				TodayChecked: todayChecked,
			}
		}
	}
	return &CheckInRankResult{Items: items, Total: totalUsers, Page: page, Pages: pages, Me: me}, nil
}
