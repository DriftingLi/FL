// PROTOTYPE — throwaway types for 任务中心 + 积分占位
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
  icon?: string
}

export interface PointsSummary {
  balance: number
  todayEarnable: number
  totalEarned: number
}

export const groupLabelMap: Record<TaskGroup, string> = {
  daily: '每日任务',
  newbie: '新手任务',
  growth: '成长任务',
}

export const groupDescMap: Record<TaskGroup, string> = {
  daily: '每日 0 点重置',
  newbie: '一次性 · 完成后不再出现',
  growth: '累计达成可领取',
}
