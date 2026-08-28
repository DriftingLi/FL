package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// PointsBalanceResult 余额结果
type PointsBalanceResult struct {
	Balance     int `json:"balance"`
	TotalEarned int `json:"total_earned"`
}

// PointsLedgerItem 流水条目
type PointsLedgerItem struct {
	ID        int64     `json:"id"`
	Delta     int       `json:"delta"`
	Reason    string    `json:"reason"`
	RefType   string    `json:"ref_type"`
	RefID     string    `json:"ref_id"`
	CreatedAt time.Time `json:"created_at"`
}

// PointsLedgerResult 流水分页
type PointsLedgerResult struct {
	Items []PointsLedgerItem `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Pages int                `json:"pages"`
}

// PointsTaskItem 任务项
type PointsTaskItem struct {
	Code     string `json:"code"`
	Group    string `json:"group"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Points   int    `json:"points"`
	Status   string `json:"status"` // todo/claimable/claimed
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
}

// PointsTasksResult 任务列表
type PointsTasksResult struct {
	Tasks []PointsTaskItem `json:"tasks"`
}

// PointsClaimResult 领取结果
type PointsClaimResult struct {
	Balance     int    `json:"balance"`
	TotalEarned int    `json:"total_earned"`
	TaskStatus  string `json:"task_status"`
}

// PointsService 积分服务
type PointsService struct {
	db     *gorm.DB
	logger *zap.Logger
	clk    clock.Clock
}

func NewPointsService(db *gorm.DB, logger *zap.Logger, clk clock.Clock) *PointsService {
	if clk == nil {
		clk = clock.Real()
	}
	return &PointsService{db: db, logger: logger, clk: clk}
}

func (s *PointsService) shanghaiDate() string {
	return s.clk.Now().In(clock.Location()).Format("2006-01-02")
}

func (s *PointsService) shanghaiDateTime() time.Time {
	t := s.clk.Now().In(clock.Location())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, clock.Location())
}

// GetBalance 获取余额与累计
func (s *PointsService) GetBalance(userID int) (*PointsBalanceResult, error) {
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	var totalEarned int64
	_ = s.db.Model(&model.PointsLedger{}).Where("user_id = ? AND delta > 0", userID).Select("COALESCE(SUM(delta),0)").Scan(&totalEarned).Error
	return &PointsBalanceResult{Balance: user.PointsBalance, TotalEarned: int(totalEarned)}, nil
}

// GetLedger 流水分页
func (s *PointsService) GetLedger(userID, page, pageSize int) (*PointsLedgerResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.PointsLedger{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, err
	}
	pages := response.PageCount(total, pageSize)
	if page > pages && pages > 0 {
		page = pages
	}
	offset := (page - 1) * pageSize
	var rows []model.PointsLedger
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]PointsLedgerItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, PointsLedgerItem{
			ID:        r.ID,
			Delta:     r.Delta,
			Reason:    r.Reason,
			RefType:   r.RefType,
			RefID:     r.RefID,
			CreatedAt: r.CreatedAt,
		})
	}
	return &PointsLedgerResult{Items: items, Total: total, Page: page, Pages: pages}, nil
}

// GetTasks 获取任务列表（实时算 todo/claimable/claimed，基于真实行为表）
func (s *PointsService) GetTasks(userID int) (*PointsTasksResult, error) {
	var configs []model.PointsTaskConfig
	if err := s.db.Order("code ASC").Find(&configs).Error; err != nil {
		return nil, err
	}
	today := s.shanghaiDate()
	todayStart := s.shanghaiDateTime()
	var claims []model.PointsTaskClaim
	_ = s.db.Where("user_id = ?", userID).Find(&claims).Error
	claimMap := make(map[string]bool, len(claims))
	for _, c := range claims {
		if c.ClaimDate != nil && *c.ClaimDate == today {
			claimMap[c.TaskCode] = true
		} else if c.RefID != nil && *c.RefID != "" {
			claimMap[c.TaskCode] = true
		} else if c.ClaimDate == nil && c.RefID == nil {
			claimMap[c.TaskCode] = true
		}
	}
	// 预查用户与行为数据（减少 N+1）
	var user model.HrwaiUser
	_ = s.db.First(&user, userID).Error
	var checkinCnt int64
	_ = s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date = ?", userID, todayStart).Count(&checkinCnt).Error
	var quizCnt int64
	_ = s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND created_at >= ?", userID, todayStart).Count(&quizCnt).Error
	var mockCnt int64
	_ = s.db.Model(&model.MockExam{}).Where("student_id = ? AND created_at >= ?", userID, todayStart).Count(&mockCnt).Error
	var postCnt int64
	_ = s.db.Model(&model.ForumTopic{}).Where("user_id = ? AND created_at >= ?", userID, todayStart).Count(&postCnt).Error
	var replyCnt int64
	_ = s.db.Model(&model.ForumReply{}).Where("user_id = ? AND created_at >= ?", userID, todayStart).Count(&replyCnt).Error
	var firstCourseDone int64
	_ = s.db.Model(&model.StudyRecord{}).Where("student_id = ? AND progress >= 100", userID).Count(&firstCourseDone).Error

	tasks := make([]PointsTaskItem, 0, len(configs))
	for _, cfg := range configs {
		if claimMap[cfg.Code] {
			total := 1
			if cfg.Code == "daily_browse" || cfg.Code == "growth_reply" {
				total = 3
			}
			tasks = append(tasks, PointsTaskItem{
				Code: cfg.Code, Group: cfg.Group, Title: cfg.Title, Desc: cfg.Description,
				Points: cfg.Points, Status: "claimed", Progress: total, Total: total,
			})
			continue
		}
		status := "todo"
		progress := 0
		total := 1
		switch cfg.Code {
		case "daily_checkin":
			if checkinCnt > 0 {
				status = "claimable"
				progress = 1
			}
		case "daily_quiz":
			if quizCnt > 0 || mockCnt > 0 {
				status = "claimable"
				progress = 1
			}
		case "daily_browse":
			total = 3
			// 无后端浏览历史，首版直接可领（前端已记录历史），或 1/3 占位
			status = "claimable"
			progress = 3
		case "newbie_profile_basic":
			hasAvatar := user.AvatarURL != ""
			hasName := user.Username != "" && user.Username != user.Account
			if hasAvatar && hasName {
				status = "claimable"
				progress = 1
			}
		case "newbie_profile_contact":
			hasCompany := user.Company != ""
			hasPhoneOrEmail := user.Phone != "" || user.Email != ""
			if hasCompany && hasPhoneOrEmail {
				status = "claimable"
				progress = 1
			}
		case "newbie_credential":
			if user.CurrentCredentialID != nil {
				status = "claimable"
				progress = 1
			}
		case "newbie_first_course":
			if firstCourseDone > 0 {
				status = "claimable"
				progress = 1
			}
		case "growth_post":
			if postCnt > 0 {
				status = "claimable"
				progress = 1
			}
		case "growth_reply":
			total = 3
			progress = int(replyCnt)
			if progress > 3 {
				progress = 3
			}
			if replyCnt >= 3 {
				status = "claimable"
			}
		case "growth_mock":
			if mockCnt > 0 {
				status = "claimable"
				progress = 1
			}
		default:
			status = "claimable"
			progress = 1
		}
		if status == "claimable" && progress == 0 {
			progress = 1
		}
		tasks = append(tasks, PointsTaskItem{
			Code: cfg.Code, Group: cfg.Group, Title: cfg.Title, Desc: cfg.Description,
			Points: cfg.Points, Status: status, Progress: progress, Total: total,
		})
	}
	return &PointsTasksResult{Tasks: tasks}, nil
}

// Claim 领取任务
func (s *PointsService) Claim(ctx context.Context, userID int, taskCode string) (*PointsClaimResult, error) {
	var cfg model.PointsTaskConfig
	if err := s.db.Where("code = ?", taskCode).First(&cfg).Error; err != nil {
		return nil, errors.New("任务不存在")
	}
	// Redis 锁
	lockKey := fmt.Sprintf("points:grant:%d:%s", userID, taskCode)
	today := s.shanghaiDate()
	if cfg.Group != "newbie" {
		lockKey += ":" + today
	} else {
		lockKey += ":once"
	}
	locked := false
	if ok, err := cache.SetNX(ctx, lockKey, "1", 5*time.Second); err == nil && ok {
		locked = true
		defer func() { _ = cache.Del(ctx, lockKey) }()
	}
	// 检查是否已领取
	todayDate := s.shanghaiDate()
	var exists int64
	if cfg.Group == "newbie" {
		once := "once"
		_ = s.db.Model(&model.PointsTaskClaim{}).Where("user_id = ? AND task_code = ? AND ref_id = ?", userID, taskCode, once).Count(&exists).Error
		if exists > 0 {
			return nil, errors.New("已领取")
		}
	} else {
		_ = s.db.Model(&model.PointsTaskClaim{}).Where("user_id = ? AND task_code = ? AND claim_date = ?", userID, taskCode, todayDate).Count(&exists).Error
		if exists > 0 {
			return nil, errors.New("今日已领取")
		}
	}
	var balance, totalEarned int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 占坑
		claim := model.PointsTaskClaim{UserID: userID, TaskCode: taskCode}
		if cfg.Group == "newbie" {
			once := "once"
			claim.RefID = &once
		} else {
			claim.ClaimDate = &todayDate
		}
		_ = locked // 已在上层处理锁释放

		if err := tx.Create(&claim).Error; err != nil {
			if isDuplicateError(err) {
				return errors.New("已领取")
			}
			return err
		}
		// 2. 账本
		ledger := model.PointsLedger{UserID: userID, Delta: cfg.Points, Reason: taskCode, RefType: "task", RefID: taskCode}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		// 3. 余额原子更新
		if err := tx.Model(&model.HrwaiUser{}).Where("id = ?", userID).UpdateColumn("points_balance", gorm.Expr("points_balance + ?", cfg.Points)).Error; err != nil {
			return err
		}
		// 4. 进度快照（可选）
		_ = tx.Exec("INSERT INTO points_user_progress (user_id, task_code, progress, total, status, updated_at) VALUES (?,?,?,?,?,NOW()) ON CONFLICT (user_id, task_code) DO UPDATE SET progress=EXCLUDED.progress, status=EXCLUDED.status, updated_at=NOW()", userID, taskCode, 1, 1, "claimed").Error
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 读余额
	bal, _ := s.GetBalance(userID)
	if bal != nil {
		balance = bal.Balance
		totalEarned = bal.TotalEarned
	}
	return &PointsClaimResult{Balance: balance, TotalEarned: totalEarned, TaskStatus: "claimed"}, nil
}

// RedeemResult 兑换结果
type RedeemResult struct {
	Balance     int    `json:"balance"`
	TotalEarned int    `json:"total_earned"`
	SKU         string `json:"sku"`
	RefID       string `json:"ref_id"`
}

// RedeemCourse 兑换课程（课程级整锁）
func (s *PointsService) RedeemCourse(ctx context.Context, userID, courseID int) (*RedeemResult, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if course.PointsPrice == nil || *course.PointsPrice <= 0 {
		return nil, errors.New("该课程无需兑换")
	}
	price := *course.PointsPrice
	sku := fmt.Sprintf("course:%d", courseID)
	refID := fmt.Sprintf("%d", courseID)
	lockKey := fmt.Sprintf("shop:course:%d:%d", userID, courseID)
	if ok, err := cache.SetNX(ctx, lockKey, "1", 5*time.Second); err == nil && ok {
		defer func() { _ = cache.Del(ctx, lockKey) }()
	}
	// 已拥有校验
	var entCnt int64
	_ = s.db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, refID).Count(&entCnt).Error
	if entCnt > 0 {
		return nil, errors.New("已兑换")
	}
	// 余额校验
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.PointsBalance < price {
		return nil, errors.New("积分不足")
	}
	var balance, totalEarned int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		ent := model.UserEntitlement{UserID: userID, SKU: sku, RefID: refID}
		if err := tx.Create(&ent).Error; err != nil {
			if isDuplicateError(err) {
				return errors.New("已兑换")
			}
			return err
		}
		ledger := model.PointsLedger{UserID: userID, Delta: -price, Reason: "redeem_course", RefType: "course", RefID: refID}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.HrwaiUser{}).Where("id = ? AND points_balance >= ?", userID, price).UpdateColumn("points_balance", gorm.Expr("points_balance - ?", price)).Error; err != nil {
			return err
		}
		// 截断到 0 的兜底（若并发导致负数，CASE 钳 0）
		_ = tx.Exec("UPDATE hrwai_users SET points_balance = GREATEST(points_balance, 0) WHERE id = ?", userID).Error
		return nil
	})
	if err != nil {
		return nil, err
	}
	bal, _ := s.GetBalance(userID)
	if bal != nil {
		balance = bal.Balance
		totalEarned = bal.TotalEarned
	}
	return &RedeemResult{Balance: balance, TotalEarned: totalEarned, SKU: sku, RefID: refID}, nil
}

// RedeemShop 兑换商城物品（真题等）
func (s *PointsService) RedeemShop(ctx context.Context, userID int, sku string) (*RedeemResult, error) {
	var item model.PointsShopItem
	if err := s.db.Where("sku = ? AND enabled = true", sku).First(&item).Error; err != nil {
		return nil, errors.New("商品不存在或已下架")
	}
	lockKey := fmt.Sprintf("shop:sku:%d:%s", userID, sku)
	if ok, err := cache.SetNX(ctx, lockKey, "1", 5*time.Second); err == nil && ok {
		defer func() { _ = cache.Del(ctx, lockKey) }()
	}
	var entCnt int64
	_ = s.db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, sku).Count(&entCnt).Error
	if entCnt > 0 {
		return nil, errors.New("已兑换")
	}
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.PointsBalance < item.Price {
		return nil, errors.New("积分不足")
	}
	var balance, totalEarned int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		ent := model.UserEntitlement{UserID: userID, SKU: sku, RefID: sku}
		if err := tx.Create(&ent).Error; err != nil {
			if isDuplicateError(err) {
				return errors.New("已兑换")
			}
			return err
		}
		ledger := model.PointsLedger{UserID: userID, Delta: -item.Price, Reason: "redeem_" + sku, RefType: "shop", RefID: sku}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.HrwaiUser{}).Where("id = ?", userID).UpdateColumn("points_balance", gorm.Expr("points_balance - ?", item.Price)).Error; err != nil {
			return err
		}
		_ = tx.Exec("UPDATE hrwai_users SET points_balance = GREATEST(points_balance, 0) WHERE id = ?", userID).Error
		return nil
	})
	if err != nil {
		return nil, err
	}
	bal, _ := s.GetBalance(userID)
	if bal != nil {
		balance = bal.Balance
		totalEarned = bal.TotalEarned
	}
	return &RedeemResult{Balance: balance, TotalEarned: totalEarned, SKU: sku, RefID: sku}, nil
}

// HasEntitlement 校验是否已兑换
func (s *PointsService) HasEntitlement(userID int, sku, refID string) bool {
	var cnt int64
	_ = s.db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, refID).Count(&cnt).Error
	return cnt > 0
}
