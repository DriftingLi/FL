// Package tutorcap 讲师**有效能力集**的解析与缓存（#2599 / 规格 E3）。
//
// 为什么独立成包（而不是留在 internal/tutor）：与 admincap 同一条理由 —— 能力解析是授权
// 关注点，被两类消费面共用：生产路径（middleware 的能力守卫，经装配根注入）与测试路径
// （各域自建路由的契约测试）。域包测试 import internal/tutor 会成环（tutor → course /
// tutorfile），而本包只依赖 model / authz / gorm，任何域都能安全引用。
//
// 与 admincap 的形态刻意并列而不是泛化成一个「按角色查能力」的通用解析器：泛化会波及
// 既有测试 stub 与估值模块的注入面（规格 E4 明确否掉），而两个解析器各自只有 150 行。
package tutorcap

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// TTL 有效能力集的进程内缓存时长（与 admincap.TTL 同口径）。
//
// 40s 的取法：它是「撤销多久生效」与「每请求查库开销」之间的折中 —— 授权界面刚点完的变更，
// 操作者自己刷新页面时基本已过 TTL；而热路径上每 40s 才落一次库。规格 E8 已把「最多延迟
// 40s 生效」写成对外承诺：讲师禁用/重置口令是即时吊销会话，角色变更不做即时失效。
const TTL = 40 * time.Second

// Resolver 按讲师解析有效能力集（带进程内短缓存）。
type Resolver struct {
	db     *gorm.DB
	logger *zap.Logger

	mu    sync.RWMutex
	cache map[int]entry
}

// entry 一条缓存：granted=false 表示「无角色/不存在」——也要缓存，否则一个未授权讲师的每次
// 请求都会打库（而它恰恰是最可能被反复尝试的那种请求）。
type entry struct {
	caps    map[authz.Capability]struct{}
	granted bool
	expires time.Time
}

// New 创建解析器。logger 可为 nil（测试装配）。
func New(db *gorm.DB, logger *zap.Logger) *Resolver {
	return &Resolver{db: db, logger: logger}
}

// TutorCapabilities 返回该讲师的有效能力集。
//
// 第二个返回值 = 是否**已挂角色**：false 表示未授权（NULL role_id）或账号不存在，调用方一律
// fail closed。空集与未授权是两件事 —— 一个空角色是「有权但什么都还没勾」，仍算已授权；
// 这不是安全边界（两种都拿不到任何能力），但读起来不歧义。
//
// 第三个返回值 = 查询故障：非 nil 时守卫渲染 500 而非 403（故障不得伪装成权限问题）。
func (r *Resolver) TutorCapabilities(tutorID int) (map[authz.Capability]struct{}, bool, error) {
	if tutorID <= 0 {
		return nil, false, nil
	}
	if caps, granted, ok := r.cached(tutorID); ok {
		return caps, granted, nil
	}
	caps, granted, err := r.load(tutorID)
	if err != nil {
		// 故障**不入缓存**（规格 E3）：否则一次抖动会让该讲师在 TTL 内持续被判无权限。
		return nil, false, err
	}
	r.store(tutorID, caps, granted)
	return caps, granted, nil
}

// load 落库解析：tutor.role_id → tutor_role → 能力。
// protected 角色取 authz 讲师侧能力全集（能力词表仍只有 authz.go 一个事实源）。
//
// 账号不存在 / 未挂角色是**判定结果**（不报错、按未授权），只有真正的查询故障才返回 error。
func (r *Resolver) load(tutorID int) (map[authz.Capability]struct{}, bool, error) {
	var tutor model.Tutor
	if err := r.db.Select("tutor_id", "role_id").Where("tutor_id = ?", tutorID).First(&tutor).Error; err != nil {
		// 账号不存在不是异常路径（令牌指向了已删除的讲师）；真正的库错误要留下痕迹 ——
		// 否则「所有人突然没权限」会安静得像正常。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		r.warn("查询讲师角色失败", tutorID, err)
		return nil, false, err
	}
	if tutor.RoleID == nil {
		return nil, false, nil
	}

	var role model.TutorRole
	if err := r.db.Where("role_id = ?", *tutor.RoleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 角色被删而讲师还挂着它：按未授权处理（外键本应阻止，这里不把它当故障）。
			return nil, false, nil
		}
		r.warn("查询讲师角色失败", tutorID, err)
		return nil, false, err
	}
	if role.Protected {
		protected := authz.ProtectedTutorCapabilities()
		caps := make(map[authz.Capability]struct{}, len(protected))
		for _, c := range protected {
			caps[c] = struct{}{}
		}
		return caps, true, nil
	}

	var rows []model.TutorRoleCapability
	if err := r.db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
		r.warn("查询讲师角色能力失败", tutorID, err)
		return nil, false, err
	}
	caps := make(map[authz.Capability]struct{}, len(rows))
	for _, row := range rows {
		caps[authz.Capability(row.Capability)] = struct{}{}
	}
	return caps, true, nil
}

func (r *Resolver) cached(tutorID int) (map[authz.Capability]struct{}, bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.cache[tutorID]
	if !ok || time.Now().After(e.expires) {
		return nil, false, false
	}
	return e.caps, e.granted, true
}

func (r *Resolver) store(tutorID int, caps map[authz.Capability]struct{}, granted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		r.cache = make(map[int]entry)
	}
	// 顺手清过期项：本缓存键空间 = 讲师数量（个数量级），无需独立回收协程。
	now := time.Now()
	for k, e := range r.cache {
		if now.After(e.expires) {
			delete(r.cache, k)
		}
	}
	r.cache[tutorID] = entry{caps: caps, granted: granted, expires: now.Add(TTL)}
}

func (r *Resolver) warn(msg string, tutorID int, err error) {
	if r.logger == nil {
		return
	}
	r.logger.Warn(msg, zap.Int("tutor_id", tutorID), zap.Error(err))
}
