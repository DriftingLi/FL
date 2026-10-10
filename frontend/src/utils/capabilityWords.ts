// 能力键的中文词表单点（#1630）。
//
// 为什么在前端：能力键（`admin_role.manage` 这类）是后端的**授权词汇**，而「给管理员看的名字」
// 是展示层的事。键集合不自立清单 —— 直接吃 `config/authz` 的 AuthzCapability（生成物，事实源是
// backend/internal/authz/authz.go），`Record<AuthzCapability, string>` 穷尽：后端加一个能力而这里
// 没跟上，`npm run type-check` 直接报错（authz.ts 与后端能力表的字节锁在后端 codegen_test 里）。
//
// 资源域同样没有第二份清单：域由键派生（`资源域.动作` 的域），CapabilityDomain 是
// AuthzCapability 的模板字面量推断结果，域表也是穷尽 Record。
//
// 收敛前的形态：角色权限页把键与域**原样**印在勾选框上（超管看到的是一屏英文）。
import type { AuthzCapability } from '@/config/authz'

/**
 * 能力键的资源域（`AT.动作` 的前半段）。
 *
 * 写成 `Capability extends infer C ? …` 而不是直接对 AuthzCapability 求条件类型：条件类型只对
 * **裸类型参数**做分布，别名会被整体判定（整联合 extends 模板字面量 ⇒ 推断不出逐成员的结果）。
 */
export type CapabilityDomain = AuthzCapability extends infer C
  ? C extends `${infer Domain}.${string}`
    ? Domain
    : never
  : never

/** 能力键 → 中文名（与后端 authz.go 里每个键的行内注释同词）。 */
export const CAPABILITY_LABELS: Readonly<Record<AuthzCapability, string>> = {
  // #1639 新增的九个管理端细键一律用**与侧栏叶子同名**的短词：授权界面按侧栏分组呈现，
  // 勾选框说「用户管理」而侧栏也写「用户管理」，超管不必在两套词汇之间做翻译。
  'admin.access': '管理端入口与用户管理',
  'admin_account.manage': '管理员账号管理',
  'admin_role.manage': '管理角色与权限配置',
  'ai_assistant.use': 'AI 助手与维修诊断',
  'ai_config.manage': 'AI 配置',
  'application.review': '投递处理与简历库',
  'audit.read': '审计日志',
  'catalog.author': '目录作者面（题库标签等）',
  'check_in.use': '每日打卡',
  'contact.request': '联系方式交换（发起与查看）',
  'contact.respond': '联系方式交换（同意/拒绝/撤回）',
  'content.generate': '内容生成',
  'contribution.review': '投稿审核',
  'contribution.submit': '资料投稿',
  'course.learn': '课程与章节学习',
  'course.manage': '课程管理',
  'credential.manage': '证件管理',
  'export.run': '数据导出',
  'faq.manage': '帮助中心内容维护',
  'faq.read': '帮助中心（只读）',
  'favorite.manage': '收藏',
  'featured.manage': '内容精选',
  'forum.moderate': '论坛治理',
  'forum.participate': '论坛参与（发帖/回复/互动）',
  'hrwai_user.manage': '用户管理',
  'inspection.read': '只读巡检',
  'job.apply': '浏览职位与投递',
  'job.manage': '职位发布与管理',
  'job.report': '举报职位',
  'job_report.handle': '职位举报处置',
  'material.read': '学习资料',
  'mock_exam.take': '模拟考试',
  'notification.use': '站内信',
  'points.admin': '积分管理与扣罚',
  'points.use': '积分（余额/流水/任务/商城）',
  'position.manage': '岗位管理',
  'profile.review': '资料审核',
  'question.author': '题库作者（建题/改题/导入）',
  'question.practice': '刷题与错题本',
  'question.review': '题库审核（发布/驳回）',
  'real_exam.take': '真题卷',
  'recruit.access': '招聘工作区入口',
  'recruit.resume_pdf': '在线简历 PDF（招聘方查看）',
  'recruiter.manage': '招聘者账号管理',
  'resume.manage': '简历卡与在线简历',
  'resume.pdf': '在线简历 PDF（学员本人）',
  'search.use': '全局搜索',
  'statistics.read': '统计分析',
  'student.access': '学员工作区入口',
  'tutor.access': '讲师工作区入口',
  'tutor.manage': '讲师管理',
  'valuation.config': '残值系数配置',
  'valuation.use': '残值评估'
}

/**
 * 资源域 → 中文名。
 *
 * 用途已收窄（#1639）：授权界面的分组标题改成**侧栏分组**（RoleManage.vue 从 pages.ts 派生），
 * 不再按资源域分组；本表只剩「单看一个能力键时要说出它属于哪个域」这类兜底展示。
 * catalog / content 两个域保留 —— catalog.author 与 content.generate 仍在用它们。
 */
export const CAPABILITY_DOMAIN_LABELS: Readonly<Record<CapabilityDomain, string>> = {
  admin: '管理端',
  admin_account: '管理员账号',
  admin_role: '管理角色',
  ai_assistant: 'AI 助手',
  ai_config: 'AI 配置',
  application: '投递',
  audit: '审计',
  catalog: '培训目录',
  check_in: '打卡',
  contact: '联系方式交换',
  content: '内容',
  contribution: '投稿',
  course: '课程',
  credential: '证件',
  export: '数据导出',
  faq: '帮助中心',
  favorite: '收藏',
  featured: '内容精选',
  forum: '论坛',
  hrwai_user: '用户',
  inspection: '巡检',
  job: '职位',
  job_report: '职位举报',
  material: '学习资料',
  mock_exam: '模拟考试',
  notification: '站内信',
  points: '积分',
  position: '岗位',
  profile: '资料',
  question: '题库',
  real_exam: '真题',
  recruit: '招聘方',
  recruiter: '招聘者',
  resume: '简历',
  search: '全局搜索',
  statistics: '统计',
  student: '学员',
  tutor: '讲师',
  valuation: '残值评估'
}

/**
 * 能力键 → 用户看到的中文名。
 *
 * 未登记的键**回落为原键**而不是空串：后端新增能力、前端产物还没再生成时，页面显示英文键
 * 而不是一片空白（降级可见，不静默）。
 */
export function describeCapability(key: string): string {
  return CAPABILITY_LABELS[key as AuthzCapability] ?? key
}

/** 资源域 → 用户看到的中文名（未登记回落为原域，口径同上）。 */
export function describeCapabilityDomain(domain: string): string {
  return CAPABILITY_DOMAIN_LABELS[domain as CapabilityDomain] ?? domain
}
