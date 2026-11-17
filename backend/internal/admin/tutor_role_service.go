// 本文件：讲师**角色链**的管理端点（#2599 / 规格 E7）。
//
// 位置为什么在管理域而不在讲师域：本组端点挂 `tutor.manage`，而该键**只授管理员**
// （authz 静态表与 protectedTutorCapabilities 都不含它）。放进 internal/tutor 会让
// 「讲师能不能改自己的角色」变成一个只差一次误挂守卫的问题 —— 放在管理域，
// 讲师侧根本没有这条路由，自锁面从结构上就不存在。
//
// 与 admin 角色管理的对照（role_service.go）：形状刻意对齐（同一套 DTO 语义、同一套护栏），
// 但**能力集收口面不同** —— 讲师角色只接受讲师侧 7 键，管理员侧键一律拒。
// 这不是洁癖：能力解析器把角色能力集原样交给守卫，若允许把 `question.review` 挂进讲师角色，
// 讲师就拿到了「先审后发」的审核权，绕过规格的硬约束 5。
package admin

import (
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// ErrLastFullTutor 最后一个挂着受保护角色的讲师不可被摘掉角色。
//
// 它不是权限自锁（管理员随时能再挂回来），而是**内容面停产**保护：全权讲师是唯一持有
// `contribution.review` / `catalog.author` 的角色，把最后一个全权讲师摘空等于把投稿审核与
// 目录维护的现有操作者清零 —— 与 admin 侧「必须保留至少一个超级管理员」同一类顾虑，
// 只是后果从「进不去管理端」变成「讲师侧没有能审的人」。
//
// 规格 E7 的另一半护栏「不可摘掉自己正在挂的角色」在本侧**结构上不成立**：调用者是管理员，
// 管理员账号没有讲师角色，摘讲师角色永远摘不到自己。本侧对应位置的替代护栏就是这个
// 「至少保留一个全权讲师」——它防的是同一个失败形态（把某个角色的最后持有者清零），
// 只是对象从「操作者自己」变成「唯一能审稿的人」。
var ErrLastFullTutor = errors.New("必须保留至少一个全权讲师")

// 讲师角色侧的两个专用哨兵。
//
// 为什么不复用 admin 侧的 ErrRoleNotFound / ErrRoleInUse：那两个的文案分别是
// 「管理角色不存在」「该角色仍被管理员使用」——原样透出到讲师角色端点会给出错误的对象
// （管理员以为在操作管理角色）。状态码相同，但**文案是给人看的契约**，错对象比错状态更误导。
var (
	ErrTutorRoleNotFound = errors.New("讲师角色不存在")
	ErrTutorRoleInUse    = errors.New("该角色仍被讲师使用")
)

// TutorRoleDTO 讲师角色（列表项 / 单个角色的出口形）。
type TutorRoleDTO struct {
	RoleID       int      `json:"role_id"`
	Name         string   `json:"name"`
	Protected    bool     `json:"protected"`
	Remark       string   `json:"remark"`
	Capabilities []string `json:"capabilities" nullability:"nonnil"`
}

// TutorRoleListDTO 讲师角色列表信封。
type TutorRoleListDTO struct {
	Roles []TutorRoleDTO `json:"roles" nullability:"nonnil"`
}

// normalizeTutorCapabilities 校验并规整讲师角色能力集（按讲师侧 7 键收口）。
//
// 未知键一律 ErrUnknownCapability（不是静默丢弃）：静默丢弃会让「管理员以为授了、实际没授」
// 变成一次无声的授权失败；而管理员侧的键在这里被拒，是防止讲师侧拿到审核权的那道闸。
func normalizeTutorCapabilities(caps []string) ([]string, error) {
	allowed := make(map[string]struct{}, len(authz.ProtectedTutorCapabilities()))
	for _, c := range authz.ProtectedTutorCapabilities() {
		allowed[string(c)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(caps))
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := allowed[c]; !ok {
			return nil, ErrUnknownCapability
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}

// tutorRoleCapabilitiesOf 一个角色的能力集：protected 角色由代码回答、不读表。
func (s *Service) tutorRoleCapabilitiesOf(role model.TutorRole) ([]string, error) {
	if role.Protected {
		protected := authz.ProtectedTutorCapabilities()
		out := make([]string, 0, len(protected))
		for _, c := range protected {
			out = append(out, string(c))
		}
		sort.Strings(out)
		return out, nil
	}
	var rows []model.TutorRoleCapability
	if err := s.db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Capability)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Service) tutorRoleDTO(role model.TutorRole) (TutorRoleDTO, error) {
	caps, err := s.tutorRoleCapabilitiesOf(role)
	if err != nil {
		return TutorRoleDTO{}, err
	}
	return TutorRoleDTO{
		RoleID: role.RoleID, Name: role.Name, Protected: role.Protected,
		Remark: role.Remark, Capabilities: caps,
	}, nil
}

// ListTutorRoles 讲师角色列表。
func (s *Service) ListTutorRoles() (*TutorRoleListDTO, error) {
	var roles []model.TutorRole
	if err := s.db.Order("role_id").Find(&roles).Error; err != nil {
		return nil, err
	}
	out := make([]TutorRoleDTO, 0, len(roles))
	for _, role := range roles {
		dto, err := s.tutorRoleDTO(role)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return &TutorRoleListDTO{Roles: out}, nil
}

// CreateTutorRole 新建讲师角色。
func (s *Service) CreateTutorRole(name, remark string, caps []string) (*TutorRoleDTO, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("角色名不能为空")
	}
	caps, err := normalizeTutorCapabilities(caps)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.Model(&model.TutorRole{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrRoleNameTaken
	}
	role := model.TutorRole{Name: name, Remark: remark}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceTutorRoleCapabilities(tx, role.RoleID, caps)
	}); err != nil {
		return nil, err
	}
	dto, err := s.tutorRoleDTO(role)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// UpdateTutorRole 改角色名 / 备注 / 能力集；受保护角色一律拒绝。
func (s *Service) UpdateTutorRole(roleID int, name, remark string, caps []string) (*TutorRoleDTO, error) {
	var role model.TutorRole
	if err := s.db.Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTutorRoleNotFound
		}
		return nil, err
	}
	if role.Protected {
		return nil, ErrProtectedRole
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = role.Name
	}
	caps, err := normalizeTutorCapabilities(caps)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.Model(&model.TutorRole{}).Where("name = ? AND role_id <> ?", name, roleID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrRoleNameTaken
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TutorRole{}).Where("role_id = ?", roleID).
			Updates(map[string]any{"name": name, "remark": remark}).Error; err != nil {
			return err
		}
		return replaceTutorRoleCapabilities(tx, roleID, caps)
	}); err != nil {
		return nil, err
	}
	role.Name, role.Remark = name, remark
	dto, err := s.tutorRoleDTO(role)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// DeleteTutorRole 删角色；受保护角色与仍被讲师挂着的角色一律拒绝。
func (s *Service) DeleteTutorRole(roleID int) error {
	var role model.TutorRole
	if err := s.db.Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTutorRoleNotFound
		}
		return err
	}
	if role.Protected {
		return ErrProtectedRole
	}
	var used int64
	if err := s.db.Model(&model.Tutor{}).Where("role_id = ?", roleID).Count(&used).Error; err != nil {
		return err
	}
	if used > 0 {
		return ErrTutorRoleInUse
	}
	return s.db.Delete(&model.TutorRole{}, roleID).Error
}

// AssignTutorRole 给讲师挂角色 / 换角色 / 摘角色（roleID = 0 表示摘掉）。
//
// 摘角色时挡「最后一个全权讲师」：受保护角色是讲师侧能力全集的唯一载体，
// 把最后一个摘空会让投稿审核与目录维护当场失去操作者（ErrLastFullTutor）。
func (s *Service) AssignTutorRole(tutorID, roleID int) (*TutorDTO, error) {
	if tutorID <= 0 {
		return nil, ErrInvalidTutorID
	}
	var tutor model.Tutor
	if err := s.db.Where("tutor_id = ?", tutorID).First(&tutor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrTutorNotFound
		}
		return nil, err
	}

	if roleID <= 0 {
		if tutor.RoleID != nil {
			var current model.TutorRole
			if err := s.db.Where("role_id = ?", *tutor.RoleID).First(&current).Error; err == nil && current.Protected {
				var others int64
				if err := s.db.Model(&model.Tutor{}).
					Where("tutor_id <> ? AND role_id IN (SELECT role_id FROM tutor_role WHERE protected = ?)", tutorID, true).
					Count(&others).Error; err != nil {
					return nil, err
				}
				if others == 0 {
					return nil, ErrLastFullTutor
				}
			}
		}
		if err := s.db.Model(&model.Tutor{}).Where("tutor_id = ?", tutorID).
			Update("role_id", nil).Error; err != nil {
			return nil, err
		}
		tutor.RoleID = nil
		return s.tutorDTOWithRole(tutor)
	}

	var role model.TutorRole
	if err := s.db.Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTutorRoleNotFound
		}
		return nil, err
	}
	if err := s.db.Model(&model.Tutor{}).Where("tutor_id = ?", tutorID).
		Update("role_id", role.RoleID).Error; err != nil {
		return nil, err
	}
	tutor.RoleID = &role.RoleID
	return s.tutorDTOWithRole(tutor)
}

// replaceTutorRoleCapabilities 整批替换一个角色的能力行（先删后插，与 admin 侧同姿势）。
func replaceTutorRoleCapabilities(tx *gorm.DB, roleID int, caps []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.TutorRoleCapability{}).Error; err != nil {
		return err
	}
	for _, c := range caps {
		row := model.TutorRoleCapability{RoleID: roleID, Capability: c}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// tutorDTOWithRole 讲师出口形（含所挂角色）。
func (s *Service) tutorDTOWithRole(tutor model.Tutor) (*TutorDTO, error) {
	dto := tutorToDTO(&tutor)
	if tutor.RoleID == nil {
		return &dto, nil
	}
	var role model.TutorRole
	if err := s.db.Where("role_id = ?", *tutor.RoleID).First(&role).Error; err != nil {
		// 角色被删而讲师还挂着它（外键本应阻止）：按「未挂角色」出口，不把列表打红。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &dto, nil
		}
		return nil, err
	}
	dto.RoleID, dto.RoleName, dto.Protected = role.RoleID, role.Name, role.Protected
	return &dto, nil
}
