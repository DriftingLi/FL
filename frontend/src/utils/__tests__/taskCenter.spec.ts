// 「今日可得」口径（#2065 / 报告 §8 Q11 第 4 条）。
//
// 判据只有一条但很容易写错：**只算 daily 组**未领取的任务。旧实现把所有未领取任务加起来，
// 于是新手任务（一次性）也被算进「今天还能拿多少分」——卡片上的数字恒大于真实值
// （原型 +80 / 正确 +60 的那个错位就是这一类）。这里把口径钉在函数上，页面只做透传。
import { describe, it, expect } from 'vitest'
import { todayEarnable } from '../taskCenter'
import type { EarnableTask } from '../taskCenter'

function taskOf(over: Partial<EarnableTask>): EarnableTask {
  return {
    group: 'daily',
    points: 5,
    status: 'claimable',
    ...over
  }
}

describe('todayEarnable 今日可得', () => {
  it('只算 daily 组：新手任务（一次性）不计入今日', () => {
    const tasks = [
      taskOf({ group: 'daily', points: 5, status: 'claimable' }),
      taskOf({ group: 'daily', points: 10, status: 'todo' }),
      taskOf({ group: 'newbie', points: 20, status: 'claimable' }),
      taskOf({ group: 'newbie', points: 10, status: 'todo' })
    ]
    expect(todayEarnable(tasks)).toBe(15)
  })

  it('已领取的每日任务从今日可得里扣掉（不是永远累加）', () => {
    const tasks = [
      taskOf({ group: 'daily', points: 5, status: 'claimed' }),
      taskOf({ group: 'daily', points: 10, status: 'claimable' })
    ]
    expect(todayEarnable(tasks)).toBe(10)
  })

  it('现行配置（7 项每日任务，含浏览课程）合计 = 65', () => {
    const daily = [
      taskOf({ points: 5, status: 'claimable' }), // daily_login
      taskOf({ points: 10, status: 'claimable' }), // daily_quiz
      taskOf({ points: 5, status: 'claimable' }), // daily_browse
      taskOf({ points: 5, status: 'claimable' }), // browse_course
      taskOf({ points: 10, status: 'claimable' }), // growth_post
      taskOf({ points: 10, status: 'claimable' }), // growth_reply
      taskOf({ points: 20, status: 'claimable' }) // growth_mock
    ]
    expect(todayEarnable(daily)).toBe(65)
  })

  it('空列表 = 0（未加载时不该显示 NaN 或历史值）', () => {
    expect(todayEarnable([])).toBe(0)
  })
})
