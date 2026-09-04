// 账号与认证域（CONTEXT.md「角色」「账号与认证」）。
package model

import "time"

// ===== 1. HRWAI 账号(统一用户表) =====

// HrwaiUser 统一用户表,合并原 student 与 valuation_users 两表。
// 三套登录鉴权(培训学员端 / 残值评估 / AI 助手)共用此表与主体系 JWT。
// admin / tutor 账号仍保留独立表,不并入此表。
type HrwaiUser struct {
	ID        int    `gorm:"column:id;primaryKey" json:"id"`
	UID       int64  `gorm:"column:uid" json:"uid,string"`
	Account   string `gorm:"column:account;uniqueIndex" json:"account"`
	Username  string `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string `gorm:"column:password" json:"-"`
	AvatarURL string `gorm:"column:avatar_url" json:"avatar_url"`
	Phone     string `gorm:"column:phone" json:"phone"`
	Email     string `gorm:"column:email" json:"email,omitempty"`
	Company   string `gorm:"column:company" json:"company,omitempty"`
	// WechatOpenID 微信开放平台 openid（微信扫码登录绑定用，框架字段）。
	WechatOpenID string `gorm:"column:wechat_openid" json:"wechat_openid,omitempty"`
	// WechatUnionID 微信开放平台 unionid（多应用互通，框架字段）。
	WechatUnionID string `gorm:"column:wechat_unionid" json:"wechat_unionid,omitempty"`
	// CurrentCredentialID 当前目标证件（单选上下文，NULL 表示未预筛选）。
	CurrentCredentialID *int      `gorm:"column:current_credential_id" json:"current_credential_id,omitempty"`
	PointsBalance       int       `gorm:"column:points_balance;default:0" json:"points_balance"`
	Status              int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
}

func (HrwaiUser) TableName() string { return "hrwai_users" }

// ===== 2. 管理员 =====

type Admin struct {
	AdminID   int       `gorm:"column:admin_id;primaryKey" json:"admin_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Admin) TableName() string { return "admin" }

// ===== 3. 导师 =====

type Tutor struct {
	TutorID   int       `gorm:"column:tutor_id;primaryKey" json:"tutor_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Tutor) TableName() string { return "tutor" }

// ===== 25. 资料修改审核 =====

// ProfileChangeRequest 用户资料（昵称/头像）修改审核请求。
type ProfileChangeRequest struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int        `gorm:"column:user_id" json:"user_id"`
	FieldType    string     `gorm:"column:field_type" json:"field_type"` // nickname / avatar
	OldValue     string     `gorm:"column:old_value" json:"old_value"`
	NewValue     string     `gorm:"column:new_value" json:"new_value"`
	Status       string     `gorm:"column:status;default:pending" json:"status"`
	RejectReason string     `gorm:"column:reject_reason" json:"reject_reason"`
	ReviewedBy   *int       `gorm:"column:reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ProfileChangeRequest) TableName() string { return "profile_change_requests" }
