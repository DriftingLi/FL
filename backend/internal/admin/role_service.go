// 本文件：管理角色与管理员账号的服务面（#1621 段4）。
//
// 边界（本批落地的一半）：**角色 CRUD + 能力配置 + 管理员改挂角色**；
// 管理员口令重置与账号停用不在本批（前者要接 core 的口令写面与全会话吊销、后者要给 admin 表加
// status 列并让登录与能力解析都尊重它 —— 都是独立的一小片，另开票）。
//
// 三层防自锁（#1618 段1 的决策，本文件是它的执行点）：
//  1. 受保护角色（protected）不可改能力、不可删；
//  2. **最后一个超管**不可降级（改挂非受保护角色）——否则管理端再无能授权的人；
//  3. 授权界面本身只挂在受保护角色持有的能力上（admin.role.manage / admin.account.manage）。
package admin

import (
	"context"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/core"
	"forklift-training/internal/model"
)

var (
	// ErrProtectedRole 受保护角色（超级管理员）不可改不可删。
	ErrProtectedRole = errors.New("受保护角色不可修改")
	// ErrRoleNotFound 管理角色不存在。
	ErrRoleNotFound = errors.New("管理角色不存在")
	// ErrRoleInUse 角色仍被管理员挂着，先摘干净再删。
	ErrRoleInUse = errors.New("该角色仍被管理员使用")
	// ErrInvalidAdminID 管理员 id 不合法（非管理员 id 一族，ADR-0064 决策 3）。
	ErrInvalidAdminID = errors.New("管理员 ID 非法")
	// ErrAdminNotFound 管理员不存在。
	ErrAdminNotFound = errors.New("管理员不存在")
	// ErrLastSuperAdmin 最后一个超管不可降级（防自锁第二层）。
	ErrLastSuperAdmin = errors.New("必须保留至少一个超级管理员")
	// ErrUnknownCapability 能力键不在 authz 能力表里（写面按能力表收口，不接受任意字符串）。
	ErrUnknownCapability = errors.New("存在未知的能力键")
	// ErrRoleNameTaken 角色名重复。
	ErrRoleNameTaken = errors.New("角色名已存在")
	// ErrAdminUsernameTaken 管理员账号名重复（admin.username 有唯一索引）。
	ErrAdminUsernameTaken = errors.New("该账号名已存在")
	// ErrSelfDelete 不能删除自己的账号（删掉自己即当场失去管理端入口，且没有任何补救面）。
	ErrSelfDelete = errors.New("不能删除自己的账号")
)

// AdminRoleDTO 管理角色（含能力键，按字典序稳定输出）。
type AdminRoleDTO struct {
	RoleID       int      `json:"role_id"`
	Name         string   `json:"name"`
	Protected    bool     `json:"protected"`
	Remark       string   `json:"remark"`
	Capabilities []string `json:"capabilities" nullability:"nonnil"`
}

// AdminRoleListDTO 角色列表。
type AdminRoleListDTO struct {
	Roles []AdminRoleDTO `json:"roles" nullability:"nonnil"`
}

// AdminAccountDTO 管理员账号（RoleID/RoleName 为 0/” 表示尚未挂角色）。
type AdminAccountDTO struct {
	AdminID   int    `json:"admin_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	RoleID    int    `json:"role_id"`
	RoleName  string `json:"role_name"`
	Protected bool   `json:"protected"`
}

// AdminAccountListDTO 管理员列表。
type AdminAccountListDTO struct {
	Accounts []AdminAccountDTO `json:"accounts" nullability:"nonnil"`
}

// normalizeCapabilities 校验并归一能力键（去重 + 字典序）。
// **写面按 authz 能力表收口**：不接受表外字符串，否则库里会攒出永远不会被任何判据认出的键。
func normalizeCapabilities(caps []string) ([]string, error) {
	seen := make(map[string]bool, len(caps))
	out := make([]string, 0, len(caps))
	known := make(map[authz.Capability]bool)
	for _, c := range authz.AllCapabilities() {
		known[c] = true
	}
	for _, raw := range caps {
		c := strings.TrimSpace(raw)
		if c == "" || seen[c] {
			continue
		}
		if !known[authz.Capability(c)] {
			return nil, ErrUnknownCapability
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}

// roleCapabilitiesOf 读某角色的能力键（受保护角色返回受保护能力全集，不读表）。
func (s *Service) roleCapabilitiesOf(role model.AdminRole) ([]string, error) {
	if role.Protected {
		caps := make([]string, 0, len(authz.ProtectedAdminCapabilities()))
		for _, c := range authz.ProtectedAdminCapabilities() {
			caps = append(caps, string(c))
		}
		sort.Strings(caps)
		return caps, nil
	}
	var rows []model.AdminRoleCapability
	if err := s.db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Capability)
	}
	sort.Strings(out)
	return out, nil
}

// ListAdminRoles 角色列表（含能力集，按 role_id 升序稳定输出）。
func (s *Service) ListAdminRoles() (*AdminRoleListDTO, error) {
	var roles []model.AdminRole
	if err := s.db.Order("role_id").Find(&roles).Error; err != nil {
		return nil, err
	}
	out := make([]AdminRoleDTO, 0, len(roles))
	for _, role := range roles {
		caps, err := s.roleCapabilitiesOf(role)
		if err != nil {
			return nil, err
		}
		out = append(out, AdminRoleDTO{
			RoleID: role.RoleID, Name: role.Name, Protected: role.Protected,
			Remark: role.Remark, Capabilities: caps,
		})
	}
	return &AdminRoleListDTO{Roles: out}, nil
}

// CreateAdminRole 新建角色并落能力行。
func (s *Service) CreateAdminRole(name, remark string, caps []string) (*AdminRoleDTO, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("角色名不能为空")
	}
	normalized, err := normalizeCapabilities(caps)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.Model(&model.AdminRole{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrRoleNameTaken
	}
	role := model.AdminRole{Name: name, Remark: strings.TrimSpace(remark)}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleCapabilities(tx, role.RoleID, normalized)
	})
	if err != nil {
		return nil, err
	}
	return &AdminRoleDTO{RoleID: role.RoleID, Name: role.Name, Remark: role.Remark, Capabilities: normalized}, nil
}

// UpdateAdminRole 改角色名/备注/能力集。受保护角色一律拒绝（防自锁第一层）。
func (s *Service) UpdateAdminRole(roleID int, name, remark string, caps []string) (*AdminRoleDTO, error) {
	var role model.AdminRole
	if err := s.db.Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	if role.Protected {
		return nil, ErrProtectedRole
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("角色名不能为空")
	}
	normalized, err := normalizeCapabilities(caps)
	if err != nil {
		return nil, err
	}
	var dup int64
	if err := s.db.Model(&model.AdminRole{}).Where("name = ? AND role_id <> ?", name, roleID).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, ErrRoleNameTaken
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AdminRole{}).Where("role_id = ?", roleID).
			Updates(map[string]any{"name": name, "remark": strings.TrimSpace(remark)}).Error; err != nil {
			return err
		}
		return replaceRoleCapabilities(tx, roleID, normalized)
	})
	if err != nil {
		return nil, err
	}
	return &AdminRoleDTO{RoleID: roleID, Name: name, Remark: strings.TrimSpace(remark), Capabilities: normalized}, nil
}

// DeleteAdminRole 删除角色。受保护角色拒绝；仍被管理员挂着时拒绝（先摘干净）。
func (s *Service) DeleteAdminRole(roleID int) error {
	var role model.AdminRole
	if err := s.db.Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return err
	}
	if role.Protected {
		return ErrProtectedRole
	}
	var used int64
	if err := s.db.Model(&model.Admin{}).Where("role_id = ?", roleID).Count(&used).Error; err != nil {
		return err
	}
	if used > 0 {
		return ErrRoleInUse
	}
	return s.db.Where("role_id = ?", roleID).Delete(&model.AdminRole{}).Error
}

// ListAdminAccounts 管理员列表（含所挂角色名与是否超管）。
func (s *Service) ListAdminAccounts() (*AdminAccountListDTO, error) {
	var admins []model.Admin
	if err := s.db.Order("admin_id").Find(&admins).Error; err != nil {
		return nil, err
	}
	roles := map[int]model.AdminRole{}
	var all []model.AdminRole
	if err := s.db.Find(&all).Error; err != nil {
		return nil, err
	}
	for _, r := range all {
		roles[r.RoleID] = r
	}
	out := make([]AdminAccountDTO, 0, len(admins))
	for _, a := range admins {
		dto := AdminAccountDTO{AdminID: a.AdminID, Username: a.Username, Name: a.Name}
		if a.RoleID != nil {
			if r, ok := roles[*a.RoleID]; ok {
				dto.RoleID, dto.RoleName, dto.Protected = r.RoleID, r.Name, r.Protected
			}
		}
		out = append(out, dto)
	}
	return &AdminAccountListDTO{Accounts: out}, nil
}

// AssignAdminRole 改挂角色（防自锁第二层：不允许把**最后一个**超管降级）。
//
// 判据：目标账号当前挂的是受保护角色，且改挂目标**不是**受保护角色，且库里再没有第二个
// 挂受保护角色的账号 ⇒ 拒绝。这条只在「降级超管」这一动作上生效，挂普通角色/换普通角色不受影响。
func (s *Service) AssignAdminRole(adminID, roleID int) (*AdminAccountDTO, error) {
	if adminID <= 0 {
		return nil, ErrInvalidAdminID
	}
	var admin model.Admin
	if err := s.db.Where("admin_id = ?", adminID).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}
	var target model.AdminRole
	if err := s.db.Where("role_id = ?", roleID).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	currentIsSuper := false
	if admin.RoleID != nil {
		var current model.AdminRole
		if err := s.db.Where("role_id = ?", *admin.RoleID).First(&current).Error; err == nil {
			currentIsSuper = current.Protected
		}
	}
	if currentIsSuper && !target.Protected {
		var others int64
		if err := s.db.Model(&model.Admin{}).
			Where("admin_id <> ? AND role_id IN (SELECT role_id FROM admin_role WHERE protected = ?)", adminID, true).
			Count(&others).Error; err != nil {
			return nil, err
		}
		if others == 0 {
			return nil, ErrLastSuperAdmin
		}
	}

	if err := s.db.Model(&model.Admin{}).Where("admin_id = ?", adminID).
		Update("role_id", target.RoleID).Error; err != nil {
		return nil, err
	}
	return &AdminAccountDTO{
		AdminID: admin.AdminID, Username: admin.Username, Name: admin.Name,
		RoleID: target.RoleID, RoleName: target.Name, Protected: target.Protected,
	}, nil
}

// CreateAdminAccount 新建管理员账号（#1632）。
//
// 口令走 core 的长度规则（6-20 位，与学员/讲师/招聘者**同一处声明**）后 bcrypt 落库 ——
// 新建账号是口令写面的一种，不在这里另立一套长度规则，也不在这里发会话（新账号自己登）。
// 姓名留空时回落为账号名（展示用，不是判据）。
//
// role_id 缺省 0 = **未授权**：账号建出来但什么都看不到（能力解析对 NULL role_id fail closed），
// 由超管随后在管理员管理页给它挂角色 —— 与「新建角色默认无能力」同一姿势。
func (s *Service) CreateAdminAccount(username, name, password string, roleID int) (*AdminAccountDTO, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("账号不能为空")
	}
	if err := core.ValidatePasswordLength(password); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = username
	}
	var dup int64
	if err := s.db.Model(&model.Admin{}).Where("username = ?", username).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, ErrAdminUsernameTaken
	}
	var role *model.AdminRole
	if roleID > 0 {
		var target model.AdminRole
		if err := s.db.Where("role_id = ?", roleID).First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrRoleNotFound
			}
			return nil, err
		}
		role = &target
	}
	hashed, err := core.HashPassword(password)
	if err != nil {
		return nil, err
	}
	admin := model.Admin{Username: username, Name: name, Password: hashed}
	if role != nil {
		admin.RoleID = &role.RoleID
	}
	if err := s.db.Create(&admin).Error; err != nil {
		return nil, err
	}
	dto := &AdminAccountDTO{AdminID: admin.AdminID, Username: admin.Username, Name: admin.Name}
	if role != nil {
		dto.RoleID, dto.RoleName, dto.Protected = role.RoleID, role.Name, role.Protected
	}
	return dto, nil
}

// DeleteAdminAccount 删除管理员账号（#1632）。
//
// 三层拒绝（与改挂角色共用同一批不变式）：
//  1. 受保护角色的**最后一个**账号不可删（防自锁第二层在删除动作上的同一判据）；
//  2. 不能删自己（删完当场失去入口，且这条路径连「谁删的」都无从追问）；
//  3. 账号不存在 → 404 哨兵（不是静默成功）。
//
// 会话吊销**先于**落库删除（注销族语义，ADR-0064 决策 4）：删除是不可回退的动作，而令牌在过期前
// 仍然有效 —— 标记写失败就整体不生效（账号还在，人还能重新登录），而不是留下一个「人已删、
// 凭证还活着」的中间态。吊销成功后旧 refresh 链失效；被删账号的 access token 即便在手，
// 能力解析也会因账号不存在而 fail closed（admincap 的 not-found 分支）。
func (s *Service) DeleteAdminAccount(ctx context.Context, actorID, adminID int) error {
	if adminID <= 0 {
		return ErrInvalidAdminID
	}
	var admin model.Admin
	if err := s.db.Where("admin_id = ?", adminID).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminNotFound
		}
		return err
	}
	if actorID == adminID {
		return ErrSelfDelete
	}
	if admin.RoleID != nil {
		var role model.AdminRole
		if err := s.db.Where("role_id = ?", *admin.RoleID).First(&role).Error; err == nil && role.Protected {
			var others int64
			if err := s.db.Model(&model.Admin{}).
				Where("admin_id <> ? AND role_id IN (SELECT role_id FROM admin_role WHERE protected = ?)", adminID, true).
				Count(&others).Error; err != nil {
				return err
			}
			if others == 0 {
				return ErrLastSuperAdmin
			}
		}
	}
	if err := s.session.RevokeIdentity(ctx, string(authz.RoleAdmin), adminID); err != nil {
		return err
	}
	return s.db.Where("admin_id = ?", adminID).Delete(&model.Admin{}).Error
}

// replaceRoleCapabilities 以「先删后插」替换角色的能力行（同事务内，避免中间态）。
func replaceRoleCapabilities(tx *gorm.DB, roleID int, caps []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.AdminRoleCapability{}).Error; err != nil {
		return err
	}
	if len(caps) == 0 {
		return nil
	}
	rows := make([]model.AdminRoleCapability, 0, len(caps))
	for _, c := range caps {
		rows = append(rows, model.AdminRoleCapability{RoleID: roleID, Capability: c})
	}
	return tx.Create(&rows).Error
}
