// 任务中心分组与状态元数据（#409/#410）。
// #388 前占位页遗留的 mock 任务数据与 localStorage 辅助函数（loadTasks/saveTasks/
// loadPoints/savePoints/resetTaskCenter）已移除——任务中心只信任后端实时任务列表，
// 不再有任何本地占位回退（spec #408「占位数据不再是后端异常的正确呈现」）。
export type TaskGroup = 'daily' | 'newbie' | 'growth'
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
  growth: '成长任务',
}

// 分组口径文案与后端配置同源：#410 后成长任务与每日任务同为「每日可领」，
// 不再保留「累计达成可领取」这类与每日口径冲突的占位文案。
export const groupDescMap: Record<TaskGroup, string> = {
  daily: '每日 0 点重置，当日达成当日领',
  newbie: '一次性任务，完成后不再出现',
  growth: '每日达成每日领，当日 0 点重置',
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
