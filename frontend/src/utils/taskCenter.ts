// 任务中心分组与状态元数据（#409/#410）。
// #388 前占位页遗留的 mock 任务数据与 localStorage 辅助函数（loadTasks/saveTasks/
// loadPoints/savePoints/resetTaskCenter）已移除——任务中心只信任后端实时任务列表，
// 不再有任何本地占位回退（spec #408「占位数据不再是后端异常的正确呈现」）。
// 分组值域（ADR-0054）：growth 已退役并入 daily —— 三项 growth 任务的
// (daily_limit, total_limit) 与 daily_* 逐字一致，group 本就是 total_limit 的投影。
export type TaskGroup = 'daily' | 'newbie'
export type TaskStatus = 'todo' | 'claimable' | 'claimed'

export interface TaskItem {
  id: number
  group: TaskGroup
  title: string
  desc: string
  points: number
  status: TaskStatus
  progress?: number
  total?: number
}

export interface PointsSummary {
  balance: number
  totalEarned: number
}

export const groupLabelMap: Record<TaskGroup, string> = {
  daily: '每日任务',
  newbie: '新手任务',
}

// 分组口径文案与后端配置同源。#410 后 growth 与 daily 同为「每日可领」；
// ADR-0054 干脆把 growth 并入 daily —— 旧文案「当日达成当日领」也随之收紧：
// daily_login 已无行为前置（进任务中心即可领），不再对全组成立。
export const groupDescMap: Record<TaskGroup, string> = {
  daily: '每日 0 点重置，当日各可领一次',
  newbie: '一次性任务，完成后不再出现',
}

// 幂等错误的语义分级（#409）：按任务分组区分提示文案，不再依赖后端中文字串匹配。
export function claimDupMessage(group: TaskGroup): string {
  return group === 'newbie' ? '已领取' : '今日已领取'
}

// 是否属于「额度已用尽」类幂等失败（用于把任务置为已领取的本地自愈）。
export function isClaimExhausted(_group: TaskGroup, kind: string | undefined, _message: string): boolean {
  // 判定只依赖客户端错误分类（kind === 'business'）：领取接口的 4xx 业务失败只有
  // 「额度已用尽/已领取」语义，无其它业务分支；后端调整文案后自愈逻辑不失效（#409）。
  return kind === 'business'
}
