// 本地搜索历史（ADR-0049 决策 7）：历史只在端上，去重前置 + 上限 10 + 单条删除/清空。
import { beforeEach, describe, expect, it } from 'vitest'
import {
  SEARCH_HISTORY_MAX,
  clearSearchHistory,
  loadSearchHistory,
  pushSearchHistory,
  removeSearchHistory
} from '../searchHistory'

beforeEach(() => {
  window.localStorage.clear()
})

describe('searchHistory 本地搜索历史', () => {
  it('新词进最前，重复词去重前置', () => {
    pushSearchHistory('液压')
    pushSearchHistory('故障码')
    expect(loadSearchHistory()).toEqual(['故障码', '液压'])
    pushSearchHistory('液压')
    expect(loadSearchHistory()).toEqual(['液压', '故障码'])
  })

  it('空白词不入历史', () => {
    pushSearchHistory('   ')
    expect(loadSearchHistory()).toEqual([])
  })

  it(`超过 ${SEARCH_HISTORY_MAX} 条时截断保留最新`, () => {
    for (let i = 0; i < SEARCH_HISTORY_MAX + 3; i++) pushSearchHistory('kw' + i)
    const list = loadSearchHistory()
    expect(list).toHaveLength(SEARCH_HISTORY_MAX)
    expect(list[0]).toBe('kw' + (SEARCH_HISTORY_MAX + 2))
    expect(list).not.toContain('kw0')
  })

  it('支持单条删除与清空', () => {
    pushSearchHistory('a')
    pushSearchHistory('b')
    expect(removeSearchHistory('a')).toEqual(['b'])
    clearSearchHistory()
    expect(loadSearchHistory()).toEqual([])
  })

  it('存储损坏时返回空列表而不抛错', () => {
    window.localStorage.setItem('search_history', '{不是 JSON')
    expect(loadSearchHistory()).toEqual([])
    window.localStorage.setItem('search_history', JSON.stringify({ a: 1 }))
    expect(loadSearchHistory()).toEqual([])
  })
})
