// PROTOTYPE — throwaway mock for 任务中心
import type { TaskItem } from './types'

export const mockTasks: TaskItem[] = [
  // 每日任务
  {
    id: 1,
    group: 'daily',
    title: '每日打卡',
    desc: '完成一次打卡签到',
    points: 5,
    status: 'todo',
    progress: 0,
    total: 1,
  },
  {
    id: 2,
    group: 'daily',
    title: '每日答题 1 次',
    desc: '完成任意题库练习或模拟考试 1 次',
    points: 10,
    status: 'claimable',
    progress: 1,
    total: 1,
  },
  {
    id: 3,
    group: 'daily',
    title: '浏览 3 篇帖子',
    desc: '在学员论坛浏览 3 篇帖子',
    points: 5,
    status: 'todo',
    progress: 1,
    total: 3,
  },
  // 新手任务
  {
    id: 4,
    group: 'newbie',
    title: '完善个人资料',
    desc: '上传头像并完善昵称',
    points: 20,
    status: 'todo',
    progress: 0,
    total: 1,
  },
  {
    id: 5,
    group: 'newbie',
    title: '选定目标证件',
    desc: '完成 onboarding 选择当前证件',
    points: 10,
    status: 'claimed',
    progress: 1,
    total: 1,
  },
  {
    id: 6,
    group: 'newbie',
    title: '完成首节课程',
    desc: '观看任意课程首节并完成',
    points: 20,
    status: 'todo',
    progress: 0,
    total: 1,
  },
  // 成长任务
  {
    id: 7,
    group: 'growth',
    title: '发布 1 篇帖子',
    desc: '在学员论坛发布主题帖',
    points: 10,
    status: 'todo',
    progress: 0,
    total: 1,
  },
  {
    id: 8,
    group: 'growth',
    title: '回复 3 次',
    desc: '在论坛累计回复 3 次',
    points: 10,
    status: 'claimable',
    progress: 3,
    total: 3,
  },
  {
    id: 9,
    group: 'growth',
    title: '完成 1 次模拟考试',
    desc: '完成任意模拟考试并提交',
    points: 20,
    status: 'todo',
    progress: 0,
    total: 1,
  },
]

export const mockPoints = {
  balance: 340,
  todayEarnable: 20,
  totalEarned: 520,
}

export const POINTS_STORAGE_KEY = 'task-center:points:prototype'
export const TASKS_STORAGE_KEY = 'task-center:tasks:prototype'

export function loadPoints(): { balance: number; totalEarned: number } {
  try {
    const raw = localStorage.getItem(POINTS_STORAGE_KEY)
    if (raw) {
      const p = JSON.parse(raw)
      return { balance: p.balance ?? mockPoints.balance, totalEarned: p.totalEarned ?? mockPoints.totalEarned }
    }
  } catch {
    // ignore
  }
  return { balance: mockPoints.balance, totalEarned: mockPoints.totalEarned }
}

export function savePoints(balance: number, totalEarned: number): void {
  try {
    localStorage.setItem(POINTS_STORAGE_KEY, JSON.stringify({ balance, totalEarned }))
  } catch {
    // ignore
  }
}

export function loadTasks(): TaskItem[] {
  try {
    const raw = localStorage.getItem(TASKS_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) return parsed as TaskItem[]
    }
  } catch {
    // ignore
  }
  return mockTasks.map((t) => ({ ...t }))
}

export function saveTasks(tasks: TaskItem[]): void {
  try {
    localStorage.setItem(TASKS_STORAGE_KEY, JSON.stringify(tasks))
  } catch {
    // ignore
  }
}

export function resetAll(): void {
  try {
    localStorage.removeItem(POINTS_STORAGE_KEY)
    localStorage.removeItem(TASKS_STORAGE_KEY)
  } catch {
    // ignore
  }
}
