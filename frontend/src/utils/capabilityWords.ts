// 能力键的中文词表单点（#1630）。
//
// 为什么在前端：能力键（`admin_role.manage` 这类）是后端的**授权词汇**，而「给管理员看的名字」
// 是展示层的事。键集合不自立清单 —— 直接吃 `config/authz` 的 AuthzCapability（生成物，事实源是
// backend/internal/authz/authz.go），`Record<AuthzCapability, string>` 穷尽：后端加一个能力而这里
// 没跟上，`npm run type-check` 直接报错（authz.ts 与后端能力表的字节锁在后端 codegen_test 里）。
import type { AuthzCapability } from '@/config/authz'

/** 能力键 → 中文名（与后端 authz.go 里每个键的行内注释同词）。 */
export const CAPABILITY_LABELS: Readonly<Record<AuthzCapability, string>> = {
  // #1639 新增的九个管理端细键一律用**与侧栏叶子同名**的短词：授权界面按侧栏分组呈现，
  // 勾选框说「用户管理」而侧栏也写「用户管理」，超管不必在两套词汇之间做翻译。
  // #1645 订正：拆分前这个键盖着用户管理，故旧措辞是「管理端入口与用户管理」；#1639 之后
  // 用户管理独立成 hrwai_user.manage，本键只剩入口与仪表盘（与后端 authz.go 的行内注释同词）。
  'admin.access': '管理端入口与工作台',
  'admin_account.manage': '管理员账号管理',
  'admin_role.manage': '管理角色与权限配置',
  'ai_assistant.use': 'AI 助手与维修诊断',
  'ai_config.manage': 'AI 配置',
  'application.review': '投递处理与简历库',
  'audit.read': '审计日志',
  'catalog.author': '目录作者面（题库标签等）',
  // #2499 C9：章节审核独立一键（队列/快照预览/批量过审驳回）
  'chapter.review': '章节审核（队列/批量过审驳回）',
  'check_in.use': '每日打卡',
  'contact.request': '联系方式交换（发起与查看）',
  'contact.respond': '联系方式交换（同意/拒绝/撤回）',
  'content.generate': '内容生成',
  'contribution.review': '投稿审核',
  'contribution.submit': '资料投稿',
  // #2595：tutor.access 拆三把后新增的两键。
  'course.author': '课程内容编辑（章节写/附件上传删除/提审/图文传图）',
  'course.learn': '课程与章节学习',
  'course.manage': '课程管理',
  'credential.manage': '证件管理',
  'export.run': '数据导出',
  'faq.manage': '帮助中心内容维护',
  'faq.read': '帮助中心（只读）',
  'feedback.manage': '反馈处置（列表与标记已处理）',
  'feedback.submit': '提交反馈',
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
  'tutorfile.manage': '讲师文件库（分片上传/挂接/改名/删除）',
  'valuation.config': '残值系数配置',
  'valuation.use': '残值评估'
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
