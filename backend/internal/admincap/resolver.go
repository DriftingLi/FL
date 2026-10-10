// Package admincap 管理端**有效能力集**的解析与缓存（#1618 段1）。
//
// 为什么独立成包（而不是留在 internal/admin）：能力解析是授权关注点，被两类消费面共用 ——
// 生产路径（middleware 的能力守卫，经装配根注入）与测试路径（各域自建路由的契约测试）。
// 域包测试 import internal/admin 会成环（admin → aiassistant → points，实测于 points 包），
// 而本包只依赖 model / authz / gorm，任何域都能安全引用。
package admincap

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// TTL 有效能力集的进程内缓存时长。
//
// 40s 的取法：它是「撤销多久生效」与「每请求查库开销」之间的折中 —— 授权界面刚点完的变更，
// 操作者自己刷新页面时基本已过 TTL；而热路径上每 40s 才落一次库。
const TTL = 40 * time.Second

// Resolver 按管理员解析有效能力集（带进程内短缓存）。
type Resolver struct {
	db     *gorm.DB
	logger *zap.Logger

	mu    sync.RWMutex
	cache map[int]entry
}

// entry 一条缓存：granted=false 表示「无角色/不存在」——也要缓存，否则一个未授权账号的每次请求
// 都会打库（而它恰恰是最可能被反复尝试的那种请求）。
type entry struct {
	caps    map[authz.Capability]struct{}
	granted bool
	expires time.Time
}

// New 创建解析器。logger 可为 nil（测试装配）。
func New(db *gorm.DB, logger *zap.Logger) *Resolver {
	return &Resolver{db: db, logger: logger}
}

// AdminCapabilities 返回该管理员的有效能力集。
//
// 第二个返回值 = 是否**已挂角色**：false 表示未授权（NULL role_id）或账号不存在，调用方一律
// fail closed。空集与未授权是两件事 —— 一个空角色是「有权但什么都还没勾」，仍算已授权；
// 这不是安全边界（两种都拿不到任何能力），但读起来不歧义。
//
// 第三个返回值 = 查询故障：非 nil 时守卫渲染 500 而非 403（故障不得伪装成权限问题）。
func (r *Resolver) AdminCapabilities(adminID int) (map[authz.Capability]struct{}, bool, error) {
	if adminID <= 0 {
		return nil, false, nil
	}
	if caps, granted, ok := r.cached(adminID); ok {
		return caps, granted, nil
	}
	caps, granted, err := r.load(adminID)
	if err != nil {
		// 故障**不入缓存**：否则一次抖动会让该管理员在 TTL 内持续被判无权限。
		return nil, false, err
	}
	r.store(adminID, caps, granted)
	return caps, granted, nil
}

// load 落库解析：admin.role_id → admin_role → 能力。
// protected 角色取 authz 受保护角色能力全集（能力词表仍只有 authz.go 一个事实源）。
//
// 账号不存在 / 未挂角色是**判定结果**（不报错、按未授权），只有真正的查询故障才返回 error。
func (r *Resolver) load(adminID int) (map[authz.Capability]struct{}, bool, error) {
	var admin model.Admin
	if err := r.db.Select("admin_id", "role_id").Where("admin_id = ?", adminID).First(&admin).Error; err != nil {
		// 账号不存在不是异常路径（令牌指向了已删除的管理员）；真正的库错误要留下痕迹 ——
		// 否则「所有人突然没权限」会安静得像正常。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		r.warn("查询管理员角色失败", adminID, err)
		return nil, false, err
	}
	if admin.RoleID == nil {
		return nil, false, nil
	}

	var role model.AdminRole
	if err := r.db.Where("role_id = ?", *admin.RoleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 角色被删而管理员还挂着它：按未授权处理（外键本应阻止，这里不把它当故障）。
			return nil, false, nil
		}
		r.warn("查询管理角色失败", adminID, err)
		return nil, false, err
	}
	if role.Protected {
		caps := make(map[authz.Capability]struct{}, len(authz.ProtectedAdminCapabilities()))
		for _, c := range authz.ProtectedAdminCapabilities() {
			caps[c] = struct{}{}
		}
		return caps, true, nil
	}

	var rows []model.AdminRoleCapability
	if err := r.db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
		r.warn("查询管理角色能力失败", adminID, err)
		return nil, false, err
	}
	caps := make(map[authz.Capability]struct{}, len(rows))
	for _, row := range rows {
		caps[authz.Capability(row.Capability)] = struct{}{}
	}
	return caps, true, nil
}

func (r *Resolver) cached(adminID int) (map[authz.Capability]struct{}, bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.cache[adminID]
	if !ok || time.Now().After(e.expires) {
		return nil, false, false
	}
	return e.caps, e.granted, true
}

func (r *Resolver) store(adminID int, caps map[authz.Capability]struct{}, granted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		r.cache = make(map[int]entry)
	}
	// 顺手清过期项：本缓存键空间 = 管理员数量（个数量级），无需独立回收协程。
	now := time.Now()
	for k, e := range r.cache {
		if now.After(e.expires) {
			delete(r.cache, k)
		}
	}
	r.cache[adminID] = entry{caps: caps, granted: granted, expires: now.Add(TTL)}
}

func (r *Resolver) warn(msg string, adminID int, err error) {
	if r.logger == nil {
		return
	}
	r.logger.Warn(msg, zap.Int("admin_id", adminID), zap.Error(err))
}
