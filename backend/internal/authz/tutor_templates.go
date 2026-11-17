// 本文件：讲师侧**预置角色模板**的能力集（#2599 / 规格 E6）。
//
// 为什么预置角色写在 authz 而不是只写在迁移 SQL 里：能力词表只有一个事实源（本包），
// 迁移里的 tutor_role_capability 行是它的**副本**。副本会漂移，所以：
//   - protected 角色（全权讲师）的能力**不落数据行**，由 ProtectedTutorCapabilities() 回答；
//   - 三个子角色的能力行由迁移 seed，并有 PG 契约测试把「库里的行」与这里的声明逐键对账。
//
// 与 protectedAdminCapabilities 的分工：那张表是「超管也持有的管理端能力」，
// 本表是「全权讲师持有的讲师端能力」。两者不是补集关系，各自回答自己那一侧的全集。
package authz

// protectedTutorCapabilities 全权讲师（protected）的能力全集：阶段一拆分后的讲师侧 7 键。
//
// 为什么显式列举而不从 roleCapabilities 反查：阶段三翻转（#2600）之后讲师成为动态角色，
// 静态表对它 fail closed（行被清空）。从静态表反查会让「全权讲师」在翻转当天变成空集 ——
// 恰好在最需要它的那一刻失效。本清单是独立事实源，翻转不影响它。
var protectedTutorCapabilities = []Capability{
	CapTutorAccess,        // 讲师工作区入口与课程/章节读
	CapCourseAuthor,       // 章节写、附件增删、提审、图文传图
	CapTutorFileManage,    // 讲师个人文件库
	CapQuestionAuthor,     // 题库作者（建题/改题/批量导入）
	CapContributionReview, // 投稿审核
	CapCatalogAuthor,      // 目录作者面（题库标签等）
	CapNotificationUse,    // 站内信
}

// ProtectedTutorCapabilities 返回全权讲师（受保护角色）的能力全集（按声明序）。
func ProtectedTutorCapabilities() []Capability {
	out := make([]Capability, len(protectedTutorCapabilities))
	copy(out, protectedTutorCapabilities)
	return out
}

// TutorTemplateRole 一个预置讲师角色模板。
//
// Protected=true 的角色能力由 ProtectedTutorCapabilities() 回答、**不落数据行**；
// 其余三个角色的能力由迁移 000061 seed 成 tutor_role_capability 行，
// 内容必须与本结构里的 Capabilities 逐键相等（PG 契约测试对账）。
type TutorTemplateRole struct {
	// Name 角色名（与迁移 SQL 的字面量逐字相同，由契约测试对账）。
	Name string
	// Protected 是否受保护角色（能力由代码回答，不可改能力、不可删除）。
	Protected bool
	// Capabilities 该角色的能力键（protected 角色忽略本字段，取全量）。
	Capabilities []Capability
}

// 预置讲师角色名。字面量只在本文件出现一次，迁移 SQL 与前端词表都以它为准。
const (
	TutorRoleNameFull     = "全权讲师"
	TutorRoleNameBank     = "题库讲师"
	TutorRoleNameCourse   = "课程讲师"
	TutorRoleNameReadonly = "只读讲师"
)

// TutorTemplateRoles 返回四个预置讲师角色（按声明序：全权在最前）。
//
// 三个子角色的能力集取法（规格 E6）：都保留 `tutor.access` 与 `notification.use` ——
// 没有入口键就登不进工作区，没有站内信键就收不到审核结果通知；
// `contribution.review` **只给全权讲师**（讲师端投稿审核 UI 是二期独立工作）；
// 只读讲师**不给** `catalog.author`：标签接口的 CRUD 一体、没有只读键，标签页守卫自然拦住。
func TutorTemplateRoles() []TutorTemplateRole {
	return []TutorTemplateRole{
		{
			Name:      TutorRoleNameFull,
			Protected: true,
			// Capabilities 留空：protected 角色的能力恒为 ProtectedTutorCapabilities() 全量。
		},
		{
			Name: TutorRoleNameBank,
			Capabilities: []Capability{
				CapTutorAccess,
				CapQuestionAuthor,
				CapCatalogAuthor,
				CapNotificationUse,
			},
		},
		{
			Name: TutorRoleNameCourse,
			Capabilities: []Capability{
				CapTutorAccess,
				CapCourseAuthor,
				CapTutorFileManage,
				CapNotificationUse,
			},
		},
		{
			Name: TutorRoleNameReadonly,
			Capabilities: []Capability{
				CapTutorAccess,
				CapNotificationUse,
			},
		},
	}
}
