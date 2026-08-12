// 答题交互共享模块（useQuestionAnswer.ts）单测：
// 序列化 round-trip / 选项切换状态转移
// 倒计时状态机（start/tick/autosave/expire）见 useCountdown.spec.ts
import { describe, it, expect } from 'vitest'
import {
  TRUE_FALSE_OPTIONS,
  buildQuestionOptions,
  toggleAnswer,
  isAnswerSelected,
  isAnswerEmpty,
  serializeAnswers,
  deserializeAnswers
} from '../useQuestionAnswer'

describe('对/错模板', () => {
  it('模板为 对/正确、错/错误', () => {
    expect(TRUE_FALSE_OPTIONS).toEqual([
      { key: '对', label: '正确' },
      { key: '错', label: '错误' }
    ])
  })

  it('判断题渲染对/错模板，其他题型取 options', () => {
    expect(buildQuestionOptions({ type: 'true_false' })).toEqual({ 对: '正确', 错: '错误' })
    expect(buildQuestionOptions({ type: 'single_choice', options: { A: '甲' } })).toEqual({ A: '甲' })
    expect(buildQuestionOptions({ type: 'short_answer' })).toEqual({})
  })
})

describe('选项切换状态转移', () => {
  it('多选：首次选中加入数组', () => {
    expect(toggleAnswer(undefined, 'A', true)).toEqual(['A'])
  })

  it('多选：再次点击移除（toggle）', () => {
    expect(toggleAnswer(['A', 'B'], 'A', true)).toEqual(['B'])
    expect(toggleAnswer(['A', 'B'], 'C', true)).toEqual(['A', 'B', 'C'])
  })

  it('多选：非数组历史值按空数组处理', () => {
    expect(toggleAnswer('A', 'B', true)).toEqual(['B'])
  })

  it('单选：直接替换为当前 key', () => {
    expect(toggleAnswer('A', 'B', false)).toBe('B')
    expect(toggleAnswer(undefined, 'B', false)).toBe('B')
    expect(toggleAnswer(['A'], 'B', false)).toBe('B')
  })

  it('isAnswerSelected 判定多选包含与单选相等', () => {
    expect(isAnswerSelected(['A', 'B'], 'A', true)).toBe(true)
    expect(isAnswerSelected(['A'], 'C', true)).toBe(false)
    expect(isAnswerSelected('A', 'A', false)).toBe(true)
    expect(isAnswerSelected('B', 'A', false)).toBe(false)
    expect(isAnswerSelected(undefined, 'A', false)).toBe(false)
    expect(isAnswerSelected(null, 'A', true)).toBe(false)
  })

  it('isAnswerEmpty 判定空值/空串/空数组', () => {
    expect(isAnswerEmpty(undefined)).toBe(true)
    expect(isAnswerEmpty(null)).toBe(true)
    expect(isAnswerEmpty('')).toBe(true)
    expect(isAnswerEmpty([])).toBe(true)
    expect(isAnswerEmpty(0)).toBe(false)
    expect(isAnswerEmpty('A')).toBe(false)
    expect(isAnswerEmpty(['A'])).toBe(false)
  })
})

describe('作答状态序列化/反序列化', () => {
  it('round-trip：数字 ID 键序列化后恢复一致', () => {
    const answers = { 1: 'A', 2: ['B', 'C'], 5: '对' }
    const serialized = serializeAnswers(answers)
    expect(serialized).toEqual({ 1: 'A', 2: ['B', 'C'], 5: '对' })
    expect(deserializeAnswers(serialized)).toEqual(answers)
  })

  it('字符串 ID 键序列化原样保留', () => {
    expect(serializeAnswers({ '7': 'D' })).toEqual({ 7: 'D' })
  })

  it('undefined 值在序列化时被剔除', () => {
    expect(serializeAnswers({ 1: undefined, 2: 'A' })).toEqual({ 2: 'A' })
  })

  it('反序列化忽略非法/非正数键', () => {
    expect(deserializeAnswers({ '0': 'x', '-1': 'y', abc: 'z' })).toEqual({})
  })
})
