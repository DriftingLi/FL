// 积分域（CONTEXT.md「积分」）。
package model

import "time"

// ===== 28. 积分系统 =====

// PointsLedger 积分账本（不可变流水，delta>0 赚取、<0 消耗）。
type PointsLedger struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int        `gorm:"column:user_id" json:"user_id"`
	Delta     int        `gorm:"column:delta" json:"delta"`
	Reason    string     `gorm:"column:reason" json:"reason"`
	RefType   string     `gorm:"column:ref_type" json:"ref_type"`
	RefID     string     `gorm:"column:ref_id" json:"ref_id"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
}

func (PointsLedger) TableName() string { return "points_ledger" }

// PointsTaskConfig 积分任务配置（10 任务种子）。
type PointsTaskConfig struct {
	Code        string `gorm:"column:code;primaryKey" json:"code"`
	Title       string `gorm:"column:title" json:"title"`
	Group       string `gorm:"column:group" json:"group"`
	Points      int    `gorm:"column:points" json:"points"`
	DailyLimit  int    `gorm:"column:daily_limit" json:"daily_limit"`
	TotalLimit  *int   `gorm:"column:total_limit" json:"total_limit,omitempty"`
	EventType   string `gorm:"column:event_type" json:"event_type"`
	Description string `gorm:"column:description" json:"description"`
}

func (PointsTaskConfig) TableName() string { return "points_task_config" }

// PointsTaskClaim 任务领取幂等占坑。
type PointsTaskClaim struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	TaskCode  string    `gorm:"column:task_code" json:"task_code"`
	ClaimDate *string   `gorm:"column:claim_date;type:date" json:"claim_date,omitempty"` // YYYY-MM-DD，Asia/Shanghai（#409：对齐同域邻居 ForumCheckIn 的日期类型标注）
	RefID     *string   `gorm:"column:ref_id" json:"ref_id,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PointsTaskClaim) TableName() string { return "points_task_claim" }

// UserDailyLogin 每日登录事实源（user_id + login_date 唯一，Asia/Shanghai 自然日）。
// 登录成功与 refresh 轮换均写入一行；同一自然日幂等。供任务中心 daily_login 判定当日达成。
type UserDailyLogin struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	LoginDate time.Time `gorm:"column:login_date;primaryKey;type:date" json:"login_date"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserDailyLogin) TableName() string { return "user_daily_login" }

// PointsUserProgress 用户任务进度快照。
type PointsUserProgress struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	TaskCode  string    `gorm:"column:task_code;primaryKey" json:"task_code"`
	Progress  int       `gorm:"column:progress" json:"progress"`
	Total     int       `gorm:"column:total" json:"total"`
	Status    string    `gorm:"column:status" json:"status"` // todo/claimable/claimed
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PointsUserProgress) TableName() string { return "points_user_progress" }

// PointsShopItem 积分商城（真题等，课程兑换走 course.points_price）。
type PointsShopItem struct {
	SKU     string `gorm:"column:sku;primaryKey" json:"sku"`
	Title   string `gorm:"column:title" json:"title"`
	Price   int    `gorm:"column:price" json:"price"`
	Stock   *int   `gorm:"column:stock" json:"stock,omitempty"`
	Enabled bool   `gorm:"column:enabled" json:"enabled"`
}

func (PointsShopItem) TableName() string { return "points_shop_item" }

// UserEntitlement 用户权益（课程/真题解锁）。
type UserEntitlement struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	SKU       string    `gorm:"column:sku;primaryKey" json:"sku"`
	RefID     string    `gorm:"column:ref_id;primaryKey" json:"ref_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserEntitlement) TableName() string { return "user_entitlement" }

// PointsEntryIdem 通用积分簿记幂等占坑（ADR-0023）：占坑行即「已处理」标记。
// 「一事件一分」与回收 settle 传确定性键（accepted_bonus:{topicID} / rollback:{topicID} /
// redeem:{ref} / ai_tokens:{requestID} 等），主键冲突 = 事件已处理。
type PointsEntryIdem struct {
	IdemKey   string    `gorm:"column:idem_key;primaryKey;size:128" json:"idem_key"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PointsEntryIdem) TableName() string { return "points_entry_idem" }
