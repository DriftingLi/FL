package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// ErrPointsProcessed 幂等占坑冲突（ADR-0023）：幂等键已存在，事件已处理过。
// 调用方据此把「二次提交」映射为各自的既有语义（已兑换/已领取/跳过回收等），整笔事务回滚。
var ErrPointsProcessed = errors.New("积分事件已处理")

// PointsEntry 单笔积分簿记的参数面（ADR-0023 事务内簿记核心 applyTx 的唯一入参）。
type PointsEntry struct {
	UserID  int
	Delta   int    // 增减分：>0 赚取，<0 消耗/扣罚（0 不写流水——points_ledger CHECK (delta <> 0)）
	Reason  string // 流水原因：task_code / ai_tokens / redeem_* / admin_penalty / rollback / accepted_bonus ...
	RefType string
	RefID   string
	// IdemKey 可选幂等占坑键（points_entry_idem 主键）。非空时占坑冲突返回
	// ErrPointsProcessed（占坑行即「已处理」标记）；键不可得的路径如实留空（ADR-0023 §5）。
	IdemKey string
	// FloorZero 封底 0：扣减量按事务内余额截断、余额钳 0（管理员罚分/违规回收语义）；
	// false（兑换/AI 扣费）走守卫扣减，余额不足报「积分不足」。
	FloorZero bool
}

// ApplyTx 事务内簿记核心（ADR-0023）：占坑（可选）→ 流水 → 余额增减。
// 封底 0 单一写法（GREATEST 钳 0）；非封底扣减一律带 `points_balance >= ?` 守卫并校验
// RowsAffected（并发双花窗口在此收口）。返回 applied=false 表示占坑冲突（事件已处理，
// 本笔簿记跳过）；调用方在事务闭包里把 ErrPointsProcessed 映射为各自语义。
func ApplyTx(tx *gorm.DB, e PointsEntry) (bool, error) {
	if e.IdemKey != "" {
		if err := tx.Create(&model.PointsEntryIdem{IdemKey: e.IdemKey}).Error; err != nil {
			if isDuplicateError(err) {
				return false, ErrPointsProcessed
			}
			return false, err
		}
	}
	delta := e.Delta
	if delta < 0 && e.FloorZero {
		// 封底 0：扣减量按事务内余额截断；余额为 0 时无流水可写（占坑行即标记）
		var user model.HrwaiUser
		if err := tx.Select("points_balance").First(&user, e.UserID).Error; err != nil {
			return false, err
		}
		if user.PointsBalance <= 0 {
			return true, nil
		}
		if user.PointsBalance < -delta {
			delta = -user.PointsBalance
		}
	}
	if delta != 0 {
		ledger := model.PointsLedger{UserID: e.UserID, Delta: delta, Reason: e.Reason, RefType: e.RefType, RefID: e.RefID}
		if err := tx.Create(&ledger).Error; err != nil {
			return false, err
		}
	}
	switch {
	case delta < 0 && e.FloorZero:
		// 封底 0 单一写法：CASE 钳 0（扣减量已按余额截断，残余并发窗口由钳位兜底；
		// CASE 形态 PG/SQLite 双方言可用）
		if err := tx.Model(&model.HrwaiUser{}).Where("id = ?", e.UserID).
			UpdateColumn("points_balance", gorm.Expr("CASE WHEN points_balance >= ? THEN points_balance - ? ELSE 0 END", -delta, -delta)).Error; err != nil {
			return false, err
		}
	case delta < 0:
		// 守卫扣减：余额不足（并发双花）0 行受影响 → 报错整笔回滚
		res := tx.Model(&model.HrwaiUser{}).Where("id = ? AND points_balance >= ?", e.UserID, -delta).
			UpdateColumn("points_balance", gorm.Expr("points_balance - ?", -delta))
		if res.Error != nil {
			return false, res.Error
		}
		if res.RowsAffected == 0 {
			return false, ErrInsufficientPoints
		}
		// 截断到 0 的兜底（并发窗口残余，维持既有兜底语义）
		_ = tx.Exec("UPDATE hrwai_users SET points_balance = GREATEST(points_balance, 0) WHERE id = ?", e.UserID).Error
	case delta > 0:
		if err := tx.Model(&model.HrwaiUser{}).Where("id = ?", e.UserID).
			UpdateColumn("points_balance", gorm.Expr("points_balance + ?", delta)).Error; err != nil {
			return false, err
		}
	}
	return true, nil
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
			var browseCnt int64
			_ = s.db.Raw("SELECT COUNT(*) FROM forum_topic_views v JOIN forum_topics t ON t.id = v.topic_id WHERE v.user_id = ? AND v.view_date = ? AND t.user_id != ?", userID, today, userID).Scan(&browseCnt).Error
			progress = int(browseCnt)
			if progress > 3 {
				progress = 3
			}
			if browseCnt >= 3 {
				status = "claimable"
			} else {
				status = "todo"
			}
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
		// 2. 簿记核心（流水 + 余额）：占坑由 points_task_claim 承载，无通用幂等键
		if _, err := ApplyTx(tx, PointsEntry{
			UserID: userID, Delta: cfg.Points, Reason: taskCode, RefType: "task", RefID: taskCode,
		}); err != nil {
			return err
		}
		// 3. 进度快照（可选）
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

// SettleRewardTx 事务内「一事件一分」直记落账（ADR-0023 forum 收编通道）：
// 占坑冲突（同键已处理）视为已处理静默跳过、事务继续——「每帖只发一次」由调用方
// 状态 CAS + 占坑双保险；封底/守卫语义由 PointsEntry 声明，其余错误上抛整笔回滚。
func (s *PointsService) SettleRewardTx(tx *gorm.DB, e PointsEntry) error {
	_, err := ApplyTx(tx, e)
	if errors.Is(err, ErrPointsProcessed) {
		return nil
	}
	return err
}

// RedeemResult 兑换结果
type RedeemResult struct {
	Balance     int    `json:"balance"`
	TotalEarned int    `json:"total_earned"`
	SKU         string `json:"sku"`
	RefID       string `json:"ref_id"`
}

// redeemOpts 兑换三胞胎（Course / RealPaper / Shop）的参数化差异面（ADR-0023）：
// 仅锁键 / sku / 权益 ref / 价格 / 流水 reason+refType 不同，簿记与并发守卫收敛于 redeem 单点。
type redeemOpts struct {
	lockKey string
	sku     string
	refID   string
	price   int
	reason  string
	refType string
}

// redeem 兑换唯一实现：锁 → 已拥有校验 → 余额预检 → 事务{权益 + 簿记核心}。
// 幂等键 redeem:{sku}（ADR-0023）：占坑冲突映射为「已兑换」，整笔事务回滚；
// 余额扣减经 ApplyTx 守卫（`points_balance >= ?` + RowsAffected 校验），并发双花不击穿余额。
func (s *PointsService) redeem(ctx context.Context, userID int, o redeemOpts) (*RedeemResult, error) {
	if ok, err := cache.SetNX(ctx, o.lockKey, "1", 5*time.Second); err == nil && ok {
		defer func() { _ = cache.Del(ctx, o.lockKey) }()
	}
	// 已拥有校验
	var entCnt int64
	_ = s.db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ? AND ref_id = ?", userID, o.sku, o.refID).Count(&entCnt).Error
	if entCnt > 0 {
		return nil, errors.New("已兑换")
	}
	// 余额预检
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.PointsBalance < o.price {
		return nil, ErrInsufficientPoints
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		ent := model.UserEntitlement{UserID: userID, SKU: o.sku, RefID: o.refID}
		if err := tx.Create(&ent).Error; err != nil {
			if isDuplicateError(err) {
				return errors.New("已兑换")
			}
			return err
		}
		if _, err := ApplyTx(tx, PointsEntry{
			UserID: userID, Delta: -o.price, Reason: o.reason, RefType: o.refType, RefID: o.refID,
			IdemKey: "redeem:" + o.sku,
		}); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, ErrPointsProcessed) {
		return nil, errors.New("已兑换")
	}
	if err != nil {
		return nil, err
	}
	bal, _ := s.GetBalance(userID)
	res := &RedeemResult{SKU: o.sku, RefID: o.refID}
	if bal != nil {
		res.Balance = bal.Balance
		res.TotalEarned = bal.TotalEarned
	}
	return res, nil
}

// RedeemCourse 兑换课程（课程级整锁）。
func (s *PointsService) RedeemCourse(ctx context.Context, userID, courseID int) (*RedeemResult, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if course.PointsPrice == nil || *course.PointsPrice <= 0 {
		return nil, errors.New("该课程无需兑换")
	}
	return s.redeem(ctx, userID, redeemOpts{
		lockKey: fmt.Sprintf("shop:course:%d:%d", userID, courseID),
		sku:     fmt.Sprintf("course:%d", courseID),
		refID:   fmt.Sprintf("%d", courseID),
		price:   *course.PointsPrice,
		reason:  "redeem_course",
		refType: "course",
	})
}

// realPaperUnlockSKU 商城里真题解锁项的 SKU（价格单点：管理员调整该项即调整全部卷价）。
const realPaperUnlockSKU = "unlock_real_paper"

// realPaperPriceFallback 商城项缺失时的兜底单价。
const realPaperPriceFallback = 300

// RealPaperSKU 真题卷权益 sku（real_paper:<paperID>，ref_id=<paperID>，按套粒度）。
func RealPaperSKU(paperID int) string { return fmt.Sprintf("real_paper:%d", paperID) }

// realPaperPrice 读取真题解锁单价（商城项缺失/停用时回退兜底价）。
func (s *PointsService) realPaperPrice() int {
	var item model.PointsShopItem
	if err := s.db.Where("sku = ? AND enabled = true", realPaperUnlockSKU).First(&item).Error; err != nil {
		return realPaperPriceFallback
	}
	if item.Price <= 0 {
		return realPaperPriceFallback
	}
	return item.Price
}

// RedeemRealPaper 兑换单套真题卷（卷级整锁，语义同 RedeemCourse）。
func (s *PointsService) RedeemRealPaper(ctx context.Context, userID, paperID int) (*RedeemResult, error) {
	var paper model.RealExamPaper
	if err := s.db.Where("paper_id = ? AND status = 1", paperID).First(&paper).Error; err != nil {
		return nil, errors.New("真题卷不存在或已下架")
	}
	return s.redeem(ctx, userID, redeemOpts{
		lockKey: fmt.Sprintf("shop:real_paper:%d:%d", userID, paperID),
		sku:     RealPaperSKU(paperID),
		refID:   strconv.Itoa(paperID),
		price:   s.realPaperPrice(),
		reason:  "redeem_real_paper",
		refType: "real_exam_paper",
	})
}

// RedeemShop 兑换商城物品（真题等）。
func (s *PointsService) RedeemShop(ctx context.Context, userID int, sku string) (*RedeemResult, error) {
	var item model.PointsShopItem
	if err := s.db.Where("sku = ? AND enabled = true", sku).First(&item).Error; err != nil {
		return nil, errors.New("商品不存在或已下架")
	}
	return s.redeem(ctx, userID, redeemOpts{
		lockKey: fmt.Sprintf("shop:sku:%d:%s", userID, sku),
		sku:     sku,
		refID:   sku,
		price:   item.Price,
		reason:  "redeem_" + sku,
		refType: "shop",
	})
}

// HasEntitlement 校验是否已兑换
func (s *PointsService) HasEntitlement(userID int, sku, refID string) bool {
	var cnt int64
	_ = s.db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, refID).Count(&cnt).Error
	return cnt > 0
}

// ErrInsufficientPoints 积分余额不足（哨兵错误，ADR-0023）：调用方一律 errors.Is 判定，
// 禁止再做「积分不足」字符串匹配（守卫扣减/兑换预检/AI 扣费同源）。
var ErrInsufficientPoints = errors.New("积分不足")

// aiMinPoints AI 对话扣费下限：余额预检门槛与扣费下限同源单点（调用侧不得另立常量）。
const aiMinPoints = 5

// AITokensResult AI 对话扣费结果（usage 事件的数据面，JSON 字段即事件负载）。
type AITokensResult struct {
	Points           int `json:"points_cost"`
	TotalTokens      int `json:"total_tokens"`
	Balance          int `json:"balance"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// estimateAITokens 按字符长度估算 tokens（口径单点）：total = ceil((prompt+completion)/4)，
// 合计 <10 时兜底 100；prompt/completion 各自 ceil(len/4)。调用侧不得重复估算。
func estimateAITokens(promptChars, completionChars int) (total, prompt, completion int) {
	prompt = (promptChars + 3) / 4
	completion = (completionChars + 3) / 4
	total = (promptChars + completionChars + 3) / 4
	if total < 10 {
		total = 100
	}
	return total, prompt, completion
}

// aiPointsForTokens tokens → 积分：ceil(tokens/1000)×10，下限 aiMinPoints 上限 100。
func aiPointsForTokens(tokens int) int {
	points := (tokens + 999) / 1000 * 10
	if points < aiMinPoints {
		points = aiMinPoints
	}
	if points > 100 {
		points = 100
	}
	return points
}

// AIPreflight AI 对话余额预检：余额低于下限返回 ErrInsufficientPoints（阻断对话）。
// 余额查询失败时放行——后计量模式下真正的闸门在末端扣费，与既有 fail-open 语义一致。
func (s *PointsService) AIPreflight(userID int) error {
	bal, err := s.GetBalance(userID)
	if err != nil || bal == nil {
		return nil
	}
	if bal.Balance < aiMinPoints {
		return ErrInsufficientPoints
	}
	return nil
}

// DeductAI AI 按 tokens 后计量扣费（ADR-0023）。调用方只传事实（prompt/completion
// 字符长度 + 稳定 requestID），tokens 估算、分桶换算、上下限与幂等全部内聚在积分域：
// 估算见 estimateAITokens，积分换算见 aiPointsForTokens；幂等键 ai_tokens:{requestID}，
// 同请求重试/重放只扣一次。成功返回 usage 事件所需的完整数据面。
func (s *PointsService) DeductAI(ctx context.Context, userID int, requestID string, promptChars, completionChars int) (*AITokensResult, error) {
	total, prompt, completion := estimateAITokens(promptChars, completionChars)
	points := aiPointsForTokens(total)
	lockKey := fmt.Sprintf("ai:tokens:%d:%s", userID, requestID)
	if ok, err := cache.SetNX(ctx, lockKey, "1", 60*time.Second); err == nil && ok {
		defer func() { _ = cache.Del(ctx, lockKey) }()
	}
	// 幂等：同一 requestId 已扣过则直接返回
	var cnt int64
	_ = s.db.Model(&model.PointsLedger{}).Where("user_id = ? AND ref_id = ? AND reason = ?", userID, requestID, "ai_tokens").Count(&cnt).Error
	if cnt > 0 {
		return s.aiTokensResult(points, total, prompt, completion, userID), nil
	}
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.PointsBalance < points {
		// 余额不足（预检后的并发窗口或直连路径）：AI 场景直接拒绝
		return nil, ErrInsufficientPoints
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 幂等键 ai_tokens:{requestID}（ADR-0023）：由调用方传稳定请求标识
		_, err := ApplyTx(tx, PointsEntry{
			UserID: userID, Delta: -points, Reason: "ai_tokens", RefType: "ai_chat", RefID: requestID,
			IdemKey: "ai_tokens:" + requestID,
		})
		if errors.Is(err, ErrPointsProcessed) {
			// 并发窗口同键已扣：与既有 isDuplicateError 分支语义一致，视为成功
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.aiTokensResult(points, total, prompt, completion, userID), nil
}

// aiTokensResult 组装扣费结果（余额实时读取，读取失败时余额置 0 维持既有语义）。
func (s *PointsService) aiTokensResult(points, total, prompt, completion, userID int) *AITokensResult {
	res := &AITokensResult{Points: points, TotalTokens: total, PromptTokens: prompt, CompletionTokens: completion}
	if bal, _ := s.GetBalance(userID); bal != nil {
		res.Balance = bal.Balance
	}
	return res
}

// AdminPenalty 管理员扣罚（自定义 1-500，截断到 0）
func (s *PointsService) AdminPenalty(ctx context.Context, adminID, userID, delta int, reason string) (int, error) {
	if delta <= 0 || delta > 500 {
		return 0, errors.New("扣罚分值需在 1-500 之间")
	}
	if reason == "" {
		return 0, errors.New("扣罚事由不能为空")
	}
	lockKey := fmt.Sprintf("points:penalty:%d", userID)
	if ok, err := cache.SetNX(ctx, lockKey, "1", 5*time.Second); err == nil && ok {
		defer func() { _ = cache.Del(ctx, lockKey) }()
	}
	var user model.HrwaiUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return 0, errors.New("用户不存在")
	}
	actualDeduct := delta
	if user.PointsBalance < delta {
		actualDeduct = user.PointsBalance
	}
	if actualDeduct <= 0 {
		return 0, nil
	}
	requestID := fmt.Sprintf("penalty-%d-%d", userID, time.Now().UnixNano())
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 不传幂等键（ADR-0023 §5）：有意重复罚分合法；封底 0，扣减量按余额截断
		// 站内信与审计由 handler 层触发（此处仅落账）
		_, err := ApplyTx(tx, PointsEntry{
			UserID: userID, Delta: -actualDeduct, Reason: "admin_penalty", RefType: "admin", RefID: requestID,
			FloorZero: true,
		})
		return err
	})
	if err != nil {
		return 0, err
	}
	return actualDeduct, nil
}
