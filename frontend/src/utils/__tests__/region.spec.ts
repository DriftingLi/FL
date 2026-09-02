// #486 地区工具：市级两级级联选项（直辖市叶子）、路径拆分/拼接/回显辅助。
import { describe, it, expect, vi, beforeEach } from 'vitest'

// 与真实 element-china-area-data pcTextArr 形状一致：直辖市 children 为区名（无「市辖区」包装）。
vi.mock('element-china-area-data', () => ({
  pcTextArr: [
    { label: '江苏省', children: [{ label: '苏州市' }, { label: '南京市' }] },
    { label: '浙江省', children: [{ label: '杭州市' }] },
    { label: '北京市', children: [{ label: '东城区' }, { label: '朝阳区' }] },
    { label: '上海市', children: [{ label: '浦东新区' }] },
  ],
}))

import { buildCityLevelRegionOptions, splitRegionPath, joinRegionPath, regionElementsToPaths, cascaderToRegionStrings } from '../region'

beforeEach(() => {
  vi.clearAllMocks()
})

describe('buildCityLevelRegionOptions（#486 级联层数）', () => {
  it('普通省两级：省→市；直辖市为叶子（真实数据区名不入级联）', () => {
    const opts = buildCityLevelRegionOptions()
    const js = opts.find((o) => o.label === '江苏省')!
    expect(js.children?.map((c) => c.label)).toEqual(['苏州市', '南京市'])
    const bj = opts.find((o) => o.label === '北京市')!
    expect(bj.children).toBeUndefined()
    const sh = opts.find((o) => o.label === '上海市')!
    expect(sh.children).toBeUndefined()
  })
})

describe('splitRegionPath / joinRegionPath', () => {
  it('两段拆分与拼接往返', () => {
    expect(splitRegionPath('江苏省/苏州市')).toEqual(['江苏省', '苏州市'])
    expect(splitRegionPath('北京市')).toEqual(['北京市'])
    expect(joinRegionPath(['江苏省', '苏州市'])).toBe('江苏省/苏州市')
  })
})

describe('regionElementsToPaths（回显）', () => {
  it('历史三段元素截断为两段（迁移后数据为两段则原样）', () => {
    expect(regionElementsToPaths(['江苏省/苏州市', '北京市'])).toEqual([['江苏省', '苏州市'], ['北京市']])
  })
})

describe('cascaderToRegionStrings（保存）', () => {
  it('路径数组拼回存储串', () => {
    expect(cascaderToRegionStrings([['江苏省', '苏州市'], ['北京市']])).toEqual(['江苏省/苏州市', '北京市'])
  })
})
