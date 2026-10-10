// 生成文件，勿手改（ADR-0047 §1 授权能力声明，spec #928）。
// 唯一事实源：后端能力表 backend/internal/authz/authz.go。
// 再生成：cd backend && go run ./cmd/gen-authz
// 同步契约：backend/internal/authz/codegen_test.go 将本文件与能力表渲染结果全等比对，
// 手改或能力表变更未再生成时后端测试即红。
//
// 用法：页面/路由声明「需要什么能力」（AuthzCapability），角色可达面由 ROLE_CAPABILITIES
// 回答。**不要**在前端另抄一份角色清单——那是本文件要消灭的东西。
//
// 例外：'admin' 的能力**不来自本表**（#1618 段1）——管理端权限由超管按角色分配，运行时经
// GET /admin/me/capabilities 下发。ROLE_CAPABILITIES.admin 因此**有意为空**，
// hasCapability('admin', …) 恒为 false 是 fail closed；消费面（路由守卫 / 侧栏过滤）必须读运行时能力集。

export type AuthzRole =
  | 'hrwai_user'
  | 'tutor'
  | 'admin'
  | 'recruiter'


export type AuthzCapability =
  | 'admin.access'
  | 'admin_account.manage'
  | 'admin_role.manage'
  | 'ai_assistant.use'
  | 'application.review'
  | 'audit.read'
  | 'catalog.author'
  | 'catalog.manage'
  | 'check_in.use'
  | 'contact.request'
  | 'contact.respond'
  | 'content.manage'
  | 'contribution.review'
  | 'contribution.submit'
  | 'course.learn'
  | 'export.run'
  | 'faq.manage'
  | 'faq.read'
  | 'favorite.manage'
  | 'forum.moderate'
  | 'forum.participate'
  | 'inspection.read'
  | 'job.apply'
  | 'job.manage'
  | 'job.report'
  | 'job_report.handle'
  | 'material.read'
  | 'mock_exam.take'
  | 'notification.use'
  | 'points.admin'
  | 'points.use'
  | 'profile.review'
  | 'question.author'
  | 'question.practice'
  | 'question.review'
  | 'real_exam.take'
  | 'recruit.access'
  | 'recruit.resume_pdf'
  | 'recruiter.manage'
  | 'resume.manage'
  | 'resume.pdf'
  | 'search.use'
  | 'student.access'
  | 'tutor.access'
  | 'valuation.config'
  | 'valuation.use'


/**
 * 能力由**数据层**回答的角色（#1618 段1）：它们的可达面必须读运行时能力集
 * （GET /admin/me/capabilities），本表对它们 fail closed。
 */
export const DYNAMIC_ROLES: readonly AuthzRole[] = ['admin']

/** 角色 → 能力集合（按能力键字典序，生成序稳定）。 */
export const ROLE_CAPABILITIES: Readonly<Record<AuthzRole, readonly AuthzCapability[]>> = {
  hrwai_user: ['ai_assistant.use', 'check_in.use', 'contact.respond', 'contribution.submit', 'course.learn', 'faq.read', 'favorite.manage', 'forum.participate', 'job.apply', 'job.report', 'material.read', 'mock_exam.take', 'notification.use', 'points.use', 'question.practice', 'real_exam.take', 'resume.manage', 'resume.pdf', 'search.use', 'student.access', 'valuation.use'],
  tutor: ['catalog.author', 'contribution.review', 'question.author', 'tutor.access'],
  // admin 有意为空：其能力由数据层回答（GET /admin/me/capabilities），本表对该角色 fail closed。
  admin: [],
  recruiter: ['application.review', 'contact.request', 'job.manage', 'recruit.access', 'recruit.resume_pdf'],
}

/** 判定角色是否拥有能力：未知角色或未登记能力一律 false（与后端 Has 同口径，fail closed）。 */
export function hasCapability(role: AuthzRole | '' | null | undefined, capability: AuthzCapability): boolean {
  if (!role) return false
  const caps = ROLE_CAPABILITIES[role]
  return !!caps && caps.includes(capability)
}
