// Package authz 是「角色 → 能力」的唯一事实源（ADR-0047 §1）。
//
// 分层：authz 不 import service / api / security —— 它是被依赖方，因此 security 与
// middleware 也能引用它。此前 security/session.go 硬编码 "recruiter"、middleware/audit.go
// 硬编码 "admin"/"tutor"，根因正是 security 不能 import service；把角色与能力放进这一层
// 才让那三处硬编码有地方可去。
//
// 词汇（CONTEXT.md「能力（capability）」）：能力是**角色对某个资源域可以做的一类动作的资格**，
// 按 `资源域.动作` 命名。**数据级不变式**（所有权、证件作用域、状态前置）与能力正交，
// 不进本表——它们由拥有该不变式的域以具名谓词单点表达。
//
// 第一期是**骨架**：能力表以现状为准（每个能力列出今天允许的角色），配套一条 api 侧的
// 一致性锁测试，逐域迁移（RoleRequired → CapabilityRequired）在下一片进行。
package authz

import "sort"

// Role 账号角色（与数据库中的角色列同值，字面量只在本文件出现一次）。
type Role string

const (
	RoleStudent   Role = "hrwai_user" // 学员
	RoleTutor     Role = "tutor"      // 讲师
	RoleAdmin     Role = "admin"      // 管理员
	RoleRecruiter Role = "recruiter"  // 企业招聘者
)

// Valid 是否为已知角色。未知角色一律 fail closed（Has 返回 false）。
func (r Role) Valid() bool {
	switch r {
	case RoleStudent, RoleTutor, RoleAdmin, RoleRecruiter:
		return true
	default:
		return false
	}
}

// Capability 能力键（资源域.动作）。
type Capability string

const (
	// ===== 学员侧（training 工作区）=====
	CapStudentAccess      Capability = "student.access"      // 学员工作区入口
	CapCourseLearn        Capability = "course.learn"        // 课程与章节学习
	CapQuestionPractice   Capability = "question.practice"   // 刷题与错题本
	CapMockExamTake       Capability = "mock_exam.take"      // 模拟考试
	CapRealExamTake       Capability = "real_exam.take"      // 真题卷
	CapFavoriteManage     Capability = "favorite.manage"     // 收藏
	CapSearchUse          Capability = "search.use"          // 全局搜索
	CapMaterialRead       Capability = "material.read"       // 学习资料
	CapCheckInUse         Capability = "check_in.use"        // 每日打卡
	CapPointsUse          Capability = "points.use"          // 积分（余额/流水/任务/商城）
	CapNotificationUse    Capability = "notification.use"    // 站内信
	CapAIAssistantUse     Capability = "ai_assistant.use"    // AI 助手与维修诊断
	CapContributionSubmit Capability = "contribution.submit" // 资料投稿（学员侧）
	CapValuationUse       Capability = "valuation.use"       // 残值评估
	CapFaqRead            Capability = "faq.read"            // 帮助中心（FAQ）只读
	CapForumParticipate   Capability = "forum.participate"   // 论坛参与（发帖/回复/互动）
	CapResumeManage       Capability = "resume.manage"       // 简历卡与在线简历
	CapJobApply           Capability = "job.apply"           // 浏览职位与投递
	CapJobReport          Capability = "job.report"          // 举报职位
	CapContactRequest     Capability = "contact.request"     // 联系方式交换：招聘方发起与查看
	CapContactRespond     Capability = "contact.respond"     // 联系方式交换：学员同意/拒绝/撤回
	CapResumePDF          Capability = "resume.pdf"          // 在线简历 PDF：学员看自己那份
	CapResumePDFView      Capability = "recruit.resume_pdf"  // 在线简历 PDF：招聘方查看学员那份

	// ===== 讲师侧 =====
	CapTutorAccess    Capability = "tutor.access"    // 讲师工作区入口
	CapQuestionAuthor Capability = "question.author" // 题库作者（建题/改题/批量导入/上传图片）

	// ===== 管理端 =====
	CapAdminAccess        Capability = "admin.access"        // 管理端入口与用户/招聘者管理
	CapQuestionReview     Capability = "question.review"     // 题库审核（发布/驳回）
	CapContributionReview Capability = "contribution.review" // 投稿审核（讲师与管理员同为审核者）
	CapCatalogManage      Capability = "catalog.manage"      // 培训目录管理（证件/方向/等级/证书模板）
	CapCatalogAuthor      Capability = "catalog.author"      // 目录作者面（题库标签等讲师可维护项）
	CapContentManage      Capability = "content.manage"      // 内容精选与内容生成
	CapProfileReview      Capability = "profile.review"      // 资料审核
	CapPointsAdmin        Capability = "points.admin"        // 积分管理与扣罚
	CapAuditRead          Capability = "audit.read"          // 审计日志
	CapForumModerate      Capability = "forum.moderate"      // 论坛治理（认定/删帖/举报处置）
	CapValuationConfig    Capability = "valuation.config"    // 残值系数配置
	CapInspectionRead     Capability = "inspection.read"     // 只读巡检
	CapExportRun          Capability = "export.run"          // 数据导出
	CapRecruiterManage    Capability = "recruiter.manage"    // 招聘者账号管理
	CapJobReportHandle    Capability = "job_report.handle"   // 职位举报处置
	CapFaqManage          Capability = "faq.manage"          // 帮助中心内容维护（分类与条目 CRUD）
	// 能力键 =「资源域.动作」，**恰一个点**（api 包的命名锁会拦下多点写法）——
	// 故资源域写成 admin_account / admin_role（与既有 job_report.handle 同形）。
	CapAdminAccountManage Capability = "admin_account.manage" // 管理员账号管理（改挂角色等）（#1621）
	CapAdminRoleManage    Capability = "admin_role.manage"    // 管理角色与能力配置（超管专属）（#1621）

	// ===== 招聘方 =====
	CapRecruitAccess     Capability = "recruit.access"     // 招聘工作区入口
	CapJobManage         Capability = "job.manage"         // 职位发布与管理
	CapApplicationReview Capability = "application.review" // 投递处理与简历库
)

// roleCapabilities 能力表（唯一事实源）。**以现状为准**：本表由当前各蓝图注册的
// RoleRequired 角色集合翻译而来，第一期不扩张也不收缩任何角色的可达面。
//
// #1618 段1 起，**管理端角色（admin）的能力不在此表回答**：它由数据层回答（管理员所挂的
// admin_role → 能力集；protected 角色取本表全量）。本表对 admin 一律 fail closed。
var roleCapabilities = map[Capability][]Role{
	CapStudentAccess:      {RoleStudent},
	CapCourseLearn:        {RoleStudent},
	CapQuestionPractice:   {RoleStudent},
	CapMockExamTake:       {RoleStudent},
	CapRealExamTake:       {RoleStudent},
	CapFavoriteManage:     {RoleStudent},
	CapSearchUse:          {RoleStudent},
	CapMaterialRead:       {RoleStudent},
	CapCheckInUse:         {RoleStudent},
	CapPointsUse:          {RoleStudent},
	CapNotificationUse:    {RoleStudent},
	CapAIAssistantUse:     {RoleStudent},
	CapContributionSubmit: {RoleStudent},
	CapValuationUse:       {RoleStudent},
	CapFaqRead:            {RoleStudent},
	CapForumParticipate:   {RoleStudent},
	CapResumeManage:       {RoleStudent},
	CapJobApply:           {RoleStudent},
	CapJobReport:          {RoleStudent},
	CapContactRequest:     {RoleRecruiter},
	CapContactRespond:     {RoleStudent},
	CapResumePDF:          {RoleStudent},
	CapResumePDFView:      {RoleRecruiter},

	CapTutorAccess:    {RoleTutor},
	CapQuestionAuthor: {RoleTutor},

	// ===== 管理端：静态表**不回答** admin 的能力（#1618 段1）=====
	// 空集 =「这个能力键存在，但它的角色可达面由数据层回答」（受保护角色持有，见
	// protectedAdminCapabilities）。写空而不是删行，是因为
	// AllCapabilities() 同时是前端 AuthzCapability 联合类型的来源 —— 删行会让键从前端类型里
	// 消失，页面描述符的 capability 声明随之编译报错。
	// 判据的单一来源是下面的 dynamicRoles；Has(RoleAdmin, …) 对动态角色一律 false（fail closed），
	// 动态路径在 middleware 的能力守卫里（见 middleware.HasCapability）。
	CapAdminAccess:        {},
	CapQuestionReview:     {},
	CapContributionReview: {RoleTutor},
	CapCatalogManage:      {},
	CapCatalogAuthor:      {RoleTutor},
	CapContentManage:      {},
	CapProfileReview:      {},
	CapPointsAdmin:        {},
	CapAuditRead:          {},
	CapForumModerate:      {},
	CapValuationConfig:    {},
	CapInspectionRead:     {},
	CapExportRun:          {},
	CapRecruiterManage:    {},
	CapJobReportHandle:    {},
	CapFaqManage:          {},
	CapAdminAccountManage: {},
	CapAdminRoleManage:    {},

	CapRecruitAccess:     {RoleRecruiter},
	CapJobManage:         {RoleRecruiter},
	CapApplicationReview: {RoleRecruiter},
}

// protectedAdminCapabilities 受保护（超级管理员）角色的能力全集（#1618 段1）。
//
// 它是「管理端能力面」这一件事的载体：静态表里管理端那些**空行**（不支持任何静态角色）正是它的
// 补集，由 authz_test 双向锁定 —— 空行 ⇔ 在本清单里，且静态行 ∪ 本清单 == AllCapabilities()。
// 因此本清单不是静态表的第二份抄写，而是被删掉的那半边的**替代声明**。
//
// 为什么不是「全部能力」：能力是「角色对某资源域的资格」。把 job.apply / course.learn 这类
// 学员与招聘方能力一并授给超管，会让超管以一个不属于它的身份出现在那些域里 —— 语义漂移，
// 且此后每新增一个学员能力都会自动扩权。管理端能力面显式列举才是准确的。
var protectedAdminCapabilities = []Capability{
	CapAdminAccess,        // 管理端入口与用户/招聘者管理
	CapQuestionAuthor,     // 题库作者（与讲师共有）
	CapQuestionReview,     // 题库审核
	CapContributionReview, // 投稿审核（与讲师共有）
	CapCatalogManage,      // 培训目录管理
	CapCatalogAuthor,      // 目录作者面（与讲师共有）
	CapContentManage,      // 内容精选与内容生成
	CapProfileReview,      // 资料审核
	CapPointsAdmin,        // 积分管理与扣罚
	CapAuditRead,          // 审计日志
	CapForumModerate,      // 论坛治理
	CapValuationConfig,    // 残值系数配置
	CapInspectionRead,     // 只读巡检
	CapExportRun,          // 数据导出
	CapRecruiterManage,    // 招聘者账号管理
	CapJobReportHandle,    // 职位举报处置
	CapFaqManage,          // 帮助中心内容维护
	CapAdminAccountManage, // 管理员账号管理（#1621）
	CapAdminRoleManage,    // 管理角色与能力配置（#1621）
}

// ProtectedAdminCapabilities 返回受保护（超级管理员）角色的能力全集（按声明序）。
func ProtectedAdminCapabilities() []Capability {
	out := make([]Capability, len(protectedAdminCapabilities))
	copy(out, protectedAdminCapabilities)
	return out
}

// dynamicRoles 能力**不由静态表回答**的角色（#1618 段1）：它们的可达面来自数据层
// （管理员所挂角色），静态表对这些角色一律 fail closed。
//
// 声明在此（而不是散在 Has / Capabilities / codegen 各写一遍），是为了让「谁是动态角色」只有
// 一个来源：codegen 依此跳过它们的 ROLE_CAPABILITIES 行，锁测试依此确认没有第二个动态角色。
var dynamicRoles = []Role{RoleAdmin}

// DynamicRoles 返回能力动态化的角色（按声明序）。
func DynamicRoles() []Role {
	out := make([]Role, len(dynamicRoles))
	copy(out, dynamicRoles)
	return out
}

// IsDynamicRole 报告该角色的能力是否由数据层回答（静态表不回答它）。
func IsDynamicRole(role Role) bool {
	for _, r := range dynamicRoles {
		if r == role {
			return true
		}
	}
	return false
}

// Has 判定角色是否拥有能力。未知角色、未登记能力、以及**动态角色**一律 false（fail closed）。
// 动态角色（admin）的实际可达面由 middleware 的能力守卫经数据层回答，不走本函数。
func Has(role Role, capability Capability) bool {
	if !role.Valid() {
		return false
	}
	for _, r := range roleCapabilities[capability] {
		if r == role {
			return true
		}
	}
	return false
}

// RolesFor 返回允许某能力的角色集合（按声明顺序，供审查与断言）。
func RolesFor(capability Capability) []Role {
	return roleCapabilities[capability]
}

// Capabilities 返回该角色的全部能力，按字典序稳定输出（供 codegen 与断言）。
func Capabilities(role Role) []Capability {
	if !role.Valid() {
		return nil
	}
	out := make([]Capability, 0, len(roleCapabilities))
	for c, roles := range roleCapabilities {
		for _, r := range roles {
			if r == role {
				out = append(out, c)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// AllCapabilities 返回全部能力键（按字典序稳定输出）。
func AllCapabilities() []Capability {
	out := make([]Capability, 0, len(roleCapabilities))
	for c := range roleCapabilities {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
