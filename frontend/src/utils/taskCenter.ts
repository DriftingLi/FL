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

export const groupDescMap: Record<TaskGroup, string> = {
  daily: '每日 0 点重置',
  newbie: '一次性 · 完成后不再出现',
  growth: '累计达成可领取',
}

const DEFAULT_TASKS: TaskItem[] = [
  { id: 1, group: 'daily', title: '每日打卡', desc: '完成一次打卡签到', points: 5, status: 'todo', progress: 0, total: 1 },
  { id: 2, group: 'daily', title: '每日答题 1 次', desc: '完成任意题库练习或模拟考试 1 次', points: 10, status: 'claimable', progress: 1, total: 1 },
  { id: 3, group: 'daily', title: '浏览 3 篇帖子', desc: '在学员论坛浏览 3 篇帖子', points: 5, status: 'todo', progress: 1, total: 3 },
  { id: 4, group: 'newbie', title: '完善个人资料', desc: '上传头像并完善昵称', points: 20, status: 'todo', progress: 0, total: 1 },
  { id: 5, group: 'newbie', title: '选定目标证件', desc: '完成 onboarding 选择当前证件', points: 10, status: 'claimed', progress: 1, total: 1 },
  { id: 6, group: 'newbie', title: '完成首节课程', desc: '观看任意课程首节并完成', points: 20, status: 'todo', progress: 0, total: 1 },
  { id: 7, group: 'growth', title: '发布 1 篇帖子', desc: '在学员论坛发布主题帖', points: 10, status: 'todo', progress: 0, total: 1 },
  { id: 8, group: 'growth', title: '回复 3 次', desc: '在论坛累计回复 3 次', points: 10, status: 'claimable', progress: 3, total: 3 },
  { id: 9, group: 'growth', title: '完成 1 次模拟考试', desc: '完成任意模拟考试并提交', points: 20, status: 'todo', progress: 0, total: 1 },
]

const DEFAULT_POINTS: PointsSummary = { balance: 340, totalEarned: 520 }

function keyFor(kind: string, userId?: number | string): string {
  const suffix = userId != null && userId !== '' ? String(userId) : 'guest'
  return `task-center:${kind}:${suffix}`
}

export function loadTasks(userId?: number | string): TaskItem[] {
  try {
    const raw = localStorage.getItem(keyFor('tasks', userId))
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) return parsed as TaskItem[]
    }
  } catch {
    // ignore
  }
  return DEFAULT_TASKS.map((t) => ({ ...t }))
}

export function saveTasks(tasks: TaskItem[], userId?: number | string): void {
  try {
    localStorage.setItem(keyFor('tasks', userId), JSON.stringify(tasks))
  } catch {
    // ignore
  }
}

export function loadPoints(userId?: number | string): PointsSummary {
  try {
    const raw = localStorage.getItem(keyFor('points', userId))
    if (raw) {
      const parsed = JSON.parse(raw)
      return { balance: parsed.balance ?? DEFAULT_POINTS.balance, totalEarned: parsed.totalEarned ?? DEFAULT_POINTS.totalEarned }
    }
  } catch {
    // ignore
  }
  return { ...DEFAULT_POINTS }
}

export function savePoints(points: PointsSummary, userId?: number | string): void {
  try {
    localStorage.setItem(keyFor('points', userId), JSON.stringify(points))
  } catch {
    // ignore
  }
}

export function resetTaskCenter(userId?: number | string): void {
  try {
    localStorage.removeItem(keyFor('tasks', userId))
    localStorage.removeItem(keyFor('points', userId))
  } catch {
    // ignore
  }
}
