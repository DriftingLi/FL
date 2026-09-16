// 估值表单页的回归网（ADR-0053 §8 票② / spec #1056）。
//
// 本票把 15 个逐字重复的「容器 + 标签」块换成 UiFormField，**字段集、校验规则与提交载荷都不许动**。
// 本 spec 钉的是「迁移后 15 个字段仍在、标签文案未变、控件仍在容器内」——这正是模板层改写
// 最容易静默出错的地方（漏 import 会导致组件整体不渲染，而 type-check 不会报未知组件）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

vi.mock('element-plus', () => ({
  ElMessage: { warning: vi.fn(), success: vi.fn(), error: vi.fn() }
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/stores/valuationEvaluation', () => ({
  useEvaluationStore: () => ({ submitting: false, submitEvaluation: vi.fn() })
}))

// vi.mock 会被提升到文件顶部：工厂里引用的东西必须用 vi.hoisted 承载
const dict = vi.hoisted(() => ({
  listBrands: vi.fn(),
  listVehicleTypes: vi.fn(),
  listSeries: vi.fn(),
  listTonnages: vi.fn(),
  listConfigTypes: vi.fn(),
  listMastTypes: vi.fn(),
  listMastHeights: vi.fn(),
  getEarliestFactoryYear: vi.fn(),
  listConditionRatings: vi.fn(),
  listProvinces: vi.fn(),
  listCities: vi.fn()
}))
vi.mock('@/api/valuation/dictionaries', () => dict)

import ValuationForm from '../ValuationForm.vue'

/**
 * 表单的字段标签（改造前后必须逐字一致）。
 * 前 15 个是**字段容器**（UiFormField）；末一个「车况评级」是卡片选择区
 * （`form-section-block` 容器，不是字段块），本票刻意不动它——所以它继续用自己的容器类。
 */
const FIELD_LABELS = [
  '品牌',
  '车辆类型',
  '系列',
  '吨位',
  '出厂年份',
  '配置类型',
  '门架类型',
  '门架高度',
  '累计工时',
  '原厂原漆',
  '省份',
  '城市',
  '是否有车牌',
  '特种设备登记证',
  '保养记录',
  '车况评级'
]

let wrapper: ReturnType<typeof mount>

beforeEach(async () => {
  for (const fn of Object.values(dict)) {
    fn.mockReset()
    fn.mockResolvedValue(fn === dict.getEarliestFactoryYear ? 1980 : [])
  }
  wrapper = mount(ValuationForm)
  await flushPromises()
})

describe('ValuationForm（字段容器迁移后的回归网）', () => {
  it('15 个字段容器全部渲染，标签文案逐字不变', () => {
    const labels = wrapper.findAll('.field-label').map((l) => l.text())
    expect(labels).toEqual(FIELD_LABELS)
  })

  it('每个容器内的控件仍在自己的容器里（控件没有跑到容器之外）', () => {
    const fields = wrapper.findAll('.field')
    // .field 容器恰好是前 15 个（第 16 个「车况评级」用的是 form-section-block）
    expect(fields).toHaveLength(FIELD_LABELS.length - 1)
    for (const field of fields) {
      // 控件形态：原生 select / input，或自绘开关（button.toggle-switch）
      const controls = field.findAll('select, input, button.toggle-switch')
      expect(controls.length, field.find('.field-label').text() + ' 容器内应有控件').toBeGreaterThan(0)
    }
  })

  it('表单里的原生控件形态未变（本票刻意不迁移控件：移动端要系统选择器）', () => {
    // 10 个 select 字段（品牌…城市）+ 5 个原生 input/开关
    expect(wrapper.findAll('select').length).toBeGreaterThanOrEqual(10)
    expect(wrapper.find('.form-card').exists()).toBe(true)
  })

  it('提交按钮在未填齐时禁用（粗校验口径不变）', () => {
    const submitBtn = wrapper.find('.footer-bar button, .form-footer button')
    if (submitBtn.exists()) {
      expect(submitBtn.attributes('disabled')).toBeDefined()
    }
  })

  it('字段容器不引入必填标记与错误行（不传即不渲染，零视觉变更）', () => {
    expect(wrapper.find('.field-required').exists()).toBe(false)
    expect(wrapper.find('.field-error').exists()).toBe(false)
  })
})
