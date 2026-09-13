// 生成文件，勿手改（ADR-0047 §1 授权能力声明，spec #928）。
// 唯一事实源：后端能力表 backend/internal/authz/authz.go。
// 再生成：cd backend && go run ./cmd/gen-authz
// 同步契约：backend/internal/authz/codegen_test.go 将本文件与能力表渲染结果全等比对，
// 手改或能力表变更未再生成时后端测试即红。
//
// 用法：页面/路由声明「需要什么能力」（AuthzCapability），角色可达面由 ROLE_CAPABILITIES
// 回答。**不要**在前端另抄一份角色清单——那是本文件要消灭的东西。

export type AuthzRole =
  | 'hrwai_user'
  | 'tutor'
  | 'admin'
  | 'recruiter'


export type AuthzCapability =
  | 'admin.access'
  | 'ai_assistant.use'
  | 'application.review'
  | 'audit.read'
  | 'catalog.manage'
  | 'check_in.use'
  | 'contact.exchange'
  | 'content.manage'
  | 'contribution.review'
  | 'contribution.submit'
  | 'course.learn'
  | 'export.run'
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
  | 'recruiter.manage'
  | 'resume.manage'
  | 'resume.pdf'
  | 'search.use'
  | 'student.access'
  | 'tutor.access'
  | 'valuation.config'
  | 'valuation.use'


/** 角色 → 能力集合（按能力键字典序，生成序稳定）。 */
export const ROLE_CAPABILITIES: Readonly<Record<AuthzRole, readonly AuthzCapability[]>> = {
  hrwai_user: ['ai_assistant.use', 'check_in.use', 'contact.exchange', 'contribution.submit', 'course.learn', 'favorite.manage', 'forum.participate', 'job.apply', 'job.report', 'material.read', 'mock_exam.take', 'notification.use', 'points.use', 'question.practice', 'real_exam.take', 'resume.manage', 'resume.pdf', 'search.use', 'student.access', 'valuation.use'],
  tutor: ['contribution.review', 'question.author', 'tutor.access'],
  admin: ['admin.access', 'audit.read', 'catalog.manage', 'content.manage', 'contribution.review', 'export.run', 'forum.moderate', 'inspection.read', 'job_report.handle', 'points.admin', 'profile.review', 'question.author', 'question.review', 'recruiter.manage', 'valuation.config'],
  recruiter: ['application.review', 'contact.exchange', 'job.manage', 'recruit.access', 'resume.pdf'],
}

/** 判定角色是否拥有能力：未知角色或未登记能力一律 false（与后端 Has 同口径，fail closed）。 */
export function hasCapability(role: AuthzRole | '' | null | undefined, capability: AuthzCapability): boolean {
  if (!role) return false
  const caps = ROLE_CAPABILITIES[role]
  return !!caps && caps.includes(capability)
}
