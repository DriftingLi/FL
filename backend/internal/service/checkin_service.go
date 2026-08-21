// Package service 实现业务服务层。
// 本文件：每日打卡（签到/日历/连击/排行榜，Asia/Shanghai 自然日语义）。
// 由 ForumService 拆出为独立 module（spec #279），与 ForumService 共享 ForumAuthor seam；
// 路由前缀 /api/forum/check-in/* 与响应契约保持不变。
package service

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// CheckInResult 打卡结果。
type CheckInResult struct {
	Checked      bool `json:"checked"`
	Streak       int  `json:"streak"`
	Total        int  `json:"total"`
	TodayChecked bool `json:"today_checked"`
}

// CheckInCalendarResult 日历结果。
type CheckInCalendarResult struct {
	Dates        []string `json:"dates"`
	Streak       int      `json:"streak"`
	Total        int      `json:"total"`
	TodayChecked bool     `json:"today_checked"`
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
}

// NewCheckInService 构造打卡服务。
func NewCheckInService(db *gorm.DB, logger *zap.Logger) *CheckInService {
	return &CheckInService{db: db, logger: logger}
}

func beijingToday() time.Time {
	now := beijingNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func computeStreakMetrics(dates []time.Time) (streak, total int, todayChecked bool) {
	total = len(dates)
	if total == 0 {
		return 0, 0, false
	}
	set := make(map[string]bool, total)
	loc := beijingNow().Location()
	for _, d := range dates {
		// Normalize to Asia/Shanghai date string to avoid UTC shift.
		set[d.In(loc).Format("2006-01-02")] = true
	}
	todayStr := beijingToday().Format("2006-01-02")
	todayChecked = set[todayStr]
	// 计算连续天数：从今天（若已签）或昨天（若今日未签）往前连续计数；若最近一天与起点间隔>1则 streak=0
	var cur time.Time
	if todayChecked {
		cur = beijingToday()
	} else {
		cur = beijingToday().AddDate(0, 0, -1)
		if !set[cur.Format("2006-01-02")] {
			return 0, total, false
		}
	}
	for {
		if set[cur.Format("2006-01-02")] {
			streak++
			cur = cur.AddDate(0, 0, -1)
		} else {
			break
		}
	}
	return streak, total, todayChecked
}

func (s *CheckInService) checkInStreakAndTotal(userID int) (streak, total int, todayChecked bool) {
	var dates []time.Time
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", userID).Order("check_date ASC").Pluck("check_date", &dates).Error; err != nil {
		return 0, 0, false
	}
	return computeStreakMetrics(dates)
}

// CheckIn 每日打卡（幂等，Asia/Shanghai 自然日）。
func (s *CheckInService) CheckIn(userID int) (*CheckInResult, error) {
	today := beijingToday()
	// 使用 DATE 类型比较时传入 today（00:00）
	var exists int64
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date = ?", userID, today).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		if err := s.db.Create(&model.ForumCheckIn{UserID: userID, CheckDate: today, CreatedAt: beijingNow()}).Error; err != nil {
			// 唯一冲突视为已签（并发幂等）
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "uq_") || strings.Contains(err.Error(), "pk_forum_checkin") || strings.Contains(err.Error(), "UNIQUE") {
				// ignore
			} else {
				return nil, err
			}
		}
	}
	streak, total, todayChecked := s.checkInStreakAndTotal(userID)
	return &CheckInResult{Checked: true, Streak: streak, Total: total, TodayChecked: todayChecked}, nil
}

// GetCheckInCalendar 获取某月已签日期及统计。
func (s *CheckInService) GetCheckInCalendar(userID, year, month int) (*CheckInCalendarResult, error) {
	if year < 2000 || year > 2100 {
		return nil, errors.New("年份无效")
	}
	if month < 1 || month > 12 {
		return nil, errors.New("月份无效")
	}
	loc := beijingNow().Location()
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	// last day: first of next month -1 day
	last := first.AddDate(0, 1, -1)
	var dates []time.Time
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date BETWEEN ? AND ?", userID, first, last).Order("check_date ASC").Pluck("check_date", &dates).Error; err != nil {
		return nil, err
	}
	strs := make([]string, 0, len(dates))
	for _, d := range dates {
		strs = append(strs, d.In(loc).Format("2006-01-02"))
	}
	streak, total, todayChecked := s.checkInStreakAndTotal(userID)
	return &CheckInCalendarResult{Dates: strs, Streak: streak, Total: total, TodayChecked: todayChecked}, nil
}

// GetCheckInRank 排行榜（累计总榜，total DESC, last_date ASC）。
func (s *CheckInService) GetCheckInRank(requesterID, page, pageSize int) (*CheckInRankResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	type aggRow struct {
		UserID   int       `gorm:"column:user_id"`
		Total    int       `gorm:"column:total"`
		LastDate time.Time `gorm:"column:last_date"`
	}
	var totalUsers int64
	if err := s.db.Table("(SELECT user_id FROM forum_checkin GROUP BY user_id) AS t").Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	pages := response.PageCount(totalUsers, pageSize)
	if page > pages && pages > 0 {
		page = pages
	}
	offset := (page - 1) * pageSize
	var rows []aggRow
	if err := s.db.Table("forum_checkin").Select("user_id, COUNT(*) as total, MAX(check_date) as last_date").Group("user_id").Order("total DESC, last_date ASC, user_id ASC").Limit(pageSize).Offset(offset).Scan(&rows).Error; err != nil {
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
	// 批量取打卡日期（一次查询取齐本页所有用户的 streak/todayChecked，消除随总天数线性增长的 N+1）
	type dateRow struct {
		UserID    int       `gorm:"column:user_id"`
		CheckDate time.Time `gorm:"column:check_date"`
	}
	var allDates []dateRow
	if len(userIDs) > 0 {
		_ = s.db.Model(&model.ForumCheckIn{}).Select("user_id, check_date").Where("user_id IN ?", userIDs).Order("check_date ASC").Find(&allDates).Error
	}
	grouped := make(map[int][]time.Time, len(userIDs))
	for _, dr := range allDates {
		grouped[dr.UserID] = append(grouped[dr.UserID], dr.CheckDate)
	}
	items := make([]CheckInRankItem, 0, len(rows))
	for i, r := range rows {
		u := userMap[r.UserID]
		streak, _, todayChecked := computeStreakMetrics(grouped[r.UserID])
		items = append(items, CheckInRankItem{
			Rank:         offset + i + 1,
			User:         ForumAuthor{UserID: r.UserID, Username: u.Username, AvatarURL: u.AvatarURL},
			Total:        r.Total,
			Streak:       streak,
			TodayChecked: todayChecked,
		})
	}
	// 当前用户排名（若不在当页仍需计算）
	var me *CheckInRankItem
	if requesterID > 0 {
		var myTotal int64
		s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", requesterID).Count(&myTotal)
		if myTotal > 0 {
			var myLast time.Time
			s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", requesterID).Select("MAX(check_date)").Scan(&myLast)
			// 计算排名：total 更高者数量 + 同 total 但 last_date 更早 + 同 total 同 last_date 但 user_id 更小 +1
			var rank int64
			// 先 count users with total > myTotal
			var higher int64
			s.db.Table("(SELECT user_id, COUNT(*) as total, MAX(check_date) as last_date FROM forum_checkin GROUP BY user_id) AS t").Where("total > ?", myTotal).Count(&higher)
			// 再 count users with total == myTotal and (last_date < myLast OR (last_date == myLast AND user_id < requesterID))
			var sameRank int64
			s.db.Table("(SELECT user_id, COUNT(*) as total, MAX(check_date) as last_date FROM forum_checkin GROUP BY user_id) AS t").Where("total = ? AND (last_date < ? OR (last_date = ? AND user_id < ?))", myTotal, myLast, myLast, requesterID).Count(&sameRank)
			rank = higher + sameRank + 1
			var mu model.HrwaiUser
			_ = s.db.First(&mu, requesterID).Error
			streak, _, todayChecked := s.checkInStreakAndTotal(requesterID)
			me = &CheckInRankItem{
				Rank:         int(rank),
				User:         ForumAuthor{UserID: requesterID, Username: mu.Username, AvatarURL: mu.AvatarURL},
				Total:        int(myTotal),
				Streak:       streak,
				TodayChecked: todayChecked,
			}
		}
	}
	return &CheckInRankResult{Items: items, Total: totalUsers, Page: page, Pages: pages, Me: me}, nil
}
