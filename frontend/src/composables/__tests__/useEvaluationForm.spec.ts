// 估值表单状态机与提交的证据面（ADR-0053 §8 票① / spec #1055）。
//
// seam：composable 接口 —— 校验走真实的 utils/valuationValidator，store 与 router 用内存 stub，
// 不触达 API 层。这是「表单校验与联动」里校验与载荷形状的部分（联动在组件层，见 ValuationForm）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

const { warningSpy } = vi.hoisted(() => ({ warningSpy: vi.fn() }))
vi.mock('element-plus', () => ({
  ElMessage: { warning: warningSpy, success: vi.fn(), error: vi.fn() }
}))

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy })
}))

const submitEvaluationSpy = vi.fn()
vi.mock('@/stores/valuationEvaluation', () => ({
  useEvaluationStore: () => ({
    submitting: false,
    submitEvaluation: submitEvaluationSpy
  })
}))

import { useEvaluationForm } from '../useEvaluationForm'

/** 填齐一份合法表单（出厂 2022、成交今年、3000 小时） */
function fillValid(form: ReturnType<typeof useEvaluationForm>['form']) {
  const thisYear = new Date().getFullYear()
  form.brand = '合力'
  form.vehicle_type = '内燃叉车'
  form.series = 'H系列'
  form.tonnage = 3
  form.config_type = '手波/国产发动机'
  form.mast_type = '两级门架'
  form.mast_height_mm = 3000
  form.factory_year = 2022
  form.sale_year = thisYear
  form.usage_hours = 3000
  form.province = '江苏省'
  form.city = '苏州市'
  form.condition_rating = 'A'
}

beforeEach(() => {
  warningSpy.mockClear()
  pushSpy.mockClear()
  submitEvaluationSpy.mockReset()
})

describe('useEvaluationForm（校验与载荷）', () => {
  it('必填缺失：validate 不通过、buildPayload 返回 null、按钮禁用态为 false', () => {
    const { validate, buildPayload, isValid } = useEvaluationForm()
    expect(validate().valid).toBe(false)
    expect(validate().message).toBeTruthy()
    expect(buildPayload()).toBeNull()
    expect(isValid.value).toBe(false)
  })

  it('逐项补全：每缺一项都给出该字段对应的提示（首个失败即返回）', () => {
    const { form, validate } = useEvaluationForm()
    fillValid(form)

    for (const [field, expected] of [
      ['brand', '请选择品牌'],
      ['vehicle_type', '请选择车辆类型'],
      ['series', '请选择系列'],
      ['config_type', '请选择配置类型'],
      ['mast_type', '请选择门架类型'],
      ['province', '请选择所在省份'],
      ['city', '请选择所在城市'],
      ['condition_rating', '请选择车况评级']
    ] as const) {
      const saved = (form as Record<string, unknown>)[field]
      ;(form as Record<string, unknown>)[field] = undefined
      const r = validate()
      expect(r.valid, field + ' 缺失时应不通过').toBe(false)
      expect(r.message).toBe(expected)
      ;(form as Record<string, unknown>)[field] = saved
    }
    expect(validate().valid).toBe(true)
  })

  it('非法值：出厂年份越界 / 成交年份早于出厂 / 工时或吨位越界都被拦（不落到 payload）', () => {
    const { form, validate } = useEvaluationForm()
    fillValid(form)

    form.factory_year = 1800
    expect(validate().message).toContain('出厂年份')
    form.factory_year = 2022

    form.sale_year = 2020 // 早于出厂
    expect(validate().message).toContain('成交年份不能早于出厂年份')
    form.sale_year = new Date().getFullYear()

    form.usage_hours = -1
    expect(validate().valid).toBe(false)
    form.usage_hours = 3000

    form.tonnage = 0
    expect(validate().message).toContain('吨位')
    form.tonnage = 3

    expect(validate().valid).toBe(true)
  })

  it('合法表单：payload 形状逐字段与 form 一致（0 与 false 不被省略）', () => {
    const { form, buildPayload, validate } = useEvaluationForm()
    fillValid(form)
    form.original_paint = false
    form.mast_height_mm = 0 // 「无」哨兵值
    expect(validate().valid).toBe(true)

    const payload = buildPayload()
    expect(payload).toEqual({
      brand: '合力',
      vehicle_type: '内燃叉车',
      series: 'H系列',
      tonnage: 3,
      config_type: '手波/国产发动机',
      mast_type: '两级门架',
      mast_height_mm: 0,
      factory_year: 2022,
      sale_year: new Date().getFullYear(),
      usage_hours: 3000,
      original_paint: false,
      province: '江苏省',
      city: '苏州市',
      has_license_plate: false,
      has_registration_certificate: false,
      has_maintenance_records: false,
      condition_rating: 'A'
    })
  })

  it('reset：回到默认值（成交年份=今年、原漆=true、三个布尔=false）', () => {
    const { form, reset } = useEvaluationForm()
    fillValid(form)
    form.original_paint = false
    form.has_license_plate = true
    reset()

    expect(form.brand).toBeUndefined()
    expect(form.sale_year).toBe(new Date().getFullYear())
    expect(form.original_paint).toBe(true)
    expect(form.has_license_plate).toBe(false)
    expect(form.has_registration_certificate).toBe(false)
    expect(form.has_maintenance_records).toBe(false)
  })

  it('submit：校验不过时不触达 store，只给提示', async () => {
    const { submit } = useEvaluationForm()
    const ok = await submit()
    expect(ok).toBe(false)
    expect(submitEvaluationSpy).not.toHaveBeenCalled()
    expect(warningSpy).toHaveBeenCalled()
    expect(pushSpy).not.toHaveBeenCalled()
  })

  it('submit 成功：提交载荷 → 跳结果页（带 id 查询参数，刷新可恢复）', async () => {
    submitEvaluationSpy.mockResolvedValue(88)
    const { form, submit } = useEvaluationForm()
    fillValid(form)

    const ok = await submit()
    expect(ok).toBe(true)
    expect(submitEvaluationSpy).toHaveBeenCalledTimes(1)
    expect(submitEvaluationSpy.mock.calls[0][0].brand).toBe('合力')
    expect(pushSpy).toHaveBeenCalledWith({ name: 'ValuationResult', query: { id: '88' } })
  })

  it('submit：store 返回空 id 时不跳转（失败已由拦截器提示）', async () => {
    submitEvaluationSpy.mockResolvedValue(null)
    const { form, submit } = useEvaluationForm()
    fillValid(form)

    expect(await submit()).toBe(false)
    expect(pushSpy).not.toHaveBeenCalled()
  })
})
