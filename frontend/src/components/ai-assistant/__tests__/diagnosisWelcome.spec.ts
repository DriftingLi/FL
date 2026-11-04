// DiagnosisWelcome spec（票 2244）：欢迎区三件套 —— 品牌卡 / 六张诉求卡 / FAQ。
//
// 三块都来自 intents（品牌与 FAQ）或硬编码（诉求卡，因为 v2 的 faults 以 brand 为必填、
// 不存在跨品牌现象清单）。断言渲染产物与事件载荷：
//   ① 品牌卡按数据渲染、点了发出 brand id；
//   ② 六张诉求卡固定六张、点了发出**语料原话**（不是给人看的短标签）；
//   ③ FAQ 按数据渲染、点了发出问题原句；
//   ④ 数据为空时三块都不渲染 —— 上游未配置时欢迎区退回只显示预设提示词。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DiagnosisWelcome from '../DiagnosisWelcome.vue'
import type { DiagnosisFAQ, DiagnosisIntentBrand } from '@/api/aiAssistant'

function brand(over: Partial<DiagnosisIntentBrand> = {}): DiagnosisIntentBrand {
  return {
    id: 'zhongli',
    name: '中力 EP',
    tagline: '电动叉车 · 可由故障码输出维修方案',
    icon: '🔋',
    cls: 'brand-zhongli',
    picker: 'system',
    ...over
  }
}

const faq: DiagnosisFAQ[] = [
  { q: '仪表上出现故障码，怎么快速查含义？', a: '先选品牌…' },
  { q: '车完全没电、拧钥匙没反应，先检查什么？', a: '按顺序三步…' }
]

function mountWelcome(over: Record<string, unknown> = {}) {
  return mount(DiagnosisWelcome, {
    props: { brands: [brand(), brand({ id: 'linde', name: '林德 Linde', picker: 'model' })], faq, ...over }
  })
}

describe('品牌卡', () => {
  it('按数据渲染名称、标语与图标', () => {
    const w = mountWelcome()
    const cards = w.findAll('.brand-card')
    expect(cards).toHaveLength(2)
    expect(w.text()).toContain('中力 EP')
    expect(w.text()).toContain('电动叉车 · 可由故障码输出维修方案')
    expect(w.text()).toContain('🔋')
  })

  it('点卡发出 brand id（不是名称：id 才是下一次调用的入参）', async () => {
    const w = mountWelcome()
    await w.findAll('.brand-card')[1].trigger('click')
    expect(w.emitted('pick-brand')).toEqual([['linde']])
  })
})

describe('六张诉求卡（硬编码）', () => {
  it('固定六张，名称是给人看的短标签', () => {
    const w = mountWelcome()
    const cards = w.findAll('.scenario-card')
    expect(cards).toHaveLength(6)
    expect(cards.map((c) => c.text().replace(/\s+/g, ''))).toEqual([
      '🚨仪表报警亮了',
      '🔋整车没电',
      '🛢️举升无力',
      '🔌充电异常',
      '🚛不能行走',
      '🛞转向失灵'
    ])
  })

  it('点了发出的是**语料原话**而不是短标签（这句话会进检索）', async () => {
    const w = mountWelcome()
    await w.findAll('.scenario-card')[2].trigger('click')
    // 短标签是「举升无力」，但发出去的必须是第三层的原生措辞
    expect(w.emitted('pick-scene')).toEqual([['举升无力 / 不能起升']])
  })

  it('六张卡本身不读数据 —— 只要品牌或 FAQ 有一份，六张就都在', () => {
    // 注意别把这条读成「数据缺席也照渲染」：票 2244 拍板的降级是**整块留白**
    // （上游 502 时画点不动的卡比留白更糟），见下面「数据缺席」那组。
    const w = mountWelcome({ brands: [], faq })
    expect(w.findAll('.scenario-card')).toHaveLength(6)
    expect(w.findAll('.brand-card')).toHaveLength(0)
    expect(w.findAll('.faq-item')).toHaveLength(2)
  })
})

describe('FAQ', () => {
  it('按 intents.faq 渲染问题，答案进 title 而不是展开区', () => {
    const w = mountWelcome()
    const items = w.findAll('.faq-item')
    expect(items).toHaveLength(2)
    expect(items[0].text()).toContain('仪表上出现故障码，怎么快速查含义？')
    expect(items[0].attributes('title')).toBe('先选品牌…')
  })

  it('点问题直接发出这句问题', async () => {
    const w = mountWelcome()
    await w.findAll('.faq-item')[0].trigger('click')
    expect(w.emitted('ask')).toEqual([['仪表上出现故障码，怎么快速查含义？']])
  })
})

describe('数据缺席（上游未配置时的降级）', () => {
  it('品牌与 FAQ 都空时整块不渲染 —— 连硬编码的六张诉求卡也不画', () => {
    // 拍板理由：诊断服务 502 时，一屏点不动的入口比留白更糟。欢迎区退回只显示预设提示词，
    // 提示条由页面（FeatureChatPage）的 welcome-bottom 槽位出，不在本组件里。
    const w = mountWelcome({ brands: [], faq: [] })
    expect(w.findAll('.brand-cards')).toHaveLength(0)
    expect(w.findAll('.scenario-cards')).toHaveLength(0)
    expect(w.findAll('.faq-list')).toHaveLength(0)
    expect(w.text()).toBe('')
  })

  it('缺省 prop（不传）同样整块不渲染', () => {
    const w = mount(DiagnosisWelcome)
    expect(w.findAll('.scenario-card')).toHaveLength(0)
  })
})
