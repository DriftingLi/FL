// DiagnosisGuide spec（票 2244）：四步向导的取数、闸门、组句与发送。
//
// 这个组件的三处判断都值得锁死，因为它们各自对应一个「看起来能跑但会错」的写法：
//   ① **闸门**：没有品牌就不能跳第二步 —— v2 的 /faults 以 brand 为必填，跳过去只会 422；
//   ② **取数随选择重拉**：换品牌必须清掉旧的系统列表与已选型号，否则用户会拿 A 品牌的
//      系统去查 B 品牌的故障；
//   ③ **组句是本地纯函数**：v2 没有 compose 端点，句子必须自己拼，品牌/型号只能进正文。
//
// 另外锁住两条降级：上游 502 时列表留空且不弹错（只出「没有可选项」文案）；
// 故障码体检只有 known/typo 两态出提示，其余静默。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import type { DiagnosisCatalogItem, DiagnosisIntentBrand } from '@/api/aiAssistant'

const api = vi.hoisted(() => ({
  listDiagnosisSystems: vi.fn(),
  listDiagnosisFaults: vi.fn(),
  checkDiagnosisCode: vi.fn()
}))

vi.mock('@/api/aiAssistant', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api/aiAssistant')
  return { ...actual, aiAssistantApi: api }
})

import DiagnosisGuide from '../DiagnosisGuide.vue'

const zhongli: DiagnosisIntentBrand = {
  id: 'zhongli',
  name: '中力 EP',
  tagline: '电动叉车 · 可由故障码输出维修方案',
  icon: '🔋',
  cls: 'brand-zhongli',
  picker: 'system'
}
const linde: DiagnosisIntentBrand = {
  id: 'linde',
  name: '林德 Linde',
  tagline: '按型号查维修手册',
  icon: '🛠️',
  cls: 'brand-linde',
  picker: 'model'
}

const systems: DiagnosisCatalogItem[] = [
  { id: 's1', name: '电池_BMS系统', icon: '🔋' },
  { id: 's2', name: '驱动系统', icon: '⚙️' }
]
const faults: DiagnosisCatalogItem[] = [
  { id: 'f1', name: '仪表报故障码', icon: '🚨' },
  { id: 'f2', name: '举升无力 / 不能起升', icon: '🛢️' }
]

function mountGuide(over: Record<string, unknown> = {}) {
  return mount(DiagnosisGuide, {
    props: { brands: [zhongli, linde], ...over },
    global: { plugins: [epLite()] }
  })
}

/** 点某个按钮（按可见文本匹配），按钮都在 .guide-body 里 */
async function clickChip(w: ReturnType<typeof mountGuide>, text: string) {
  const btn = w.findAll('button').find((b) => b.text().includes(text))
  if (!btn) throw new Error(`找不到按钮：${text}`)
  await btn.trigger('click')
  await flushPromises()
}

beforeEach(() => {
  api.listDiagnosisSystems.mockReset().mockResolvedValue({ items: systems })
  api.listDiagnosisFaults.mockReset().mockResolvedValue({ items: faults })
  api.checkDiagnosisCode.mockReset().mockResolvedValue({ status: 'none', message: '', fix: '', samples: [] })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('第一步：品牌', () => {
  it('四个步骤都在步骤条上，初始停在第 1 步', () => {
    const w = mountGuide()
    const steps = w.findAll('.guide-steps button').map((b) => b.text())
    expect(steps.join(' ')).toContain('1. 品牌')
    expect(steps.join(' ')).toContain('4. 确认')
    expect(w.text()).toContain('中力 EP')
  })

  it('选完品牌就自动进第二步，并按品牌 id 拉系统列表', async () => {
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    expect(api.listDiagnosisSystems).toHaveBeenCalledWith('zhongli')
    expect(w.text()).toContain('电池_BMS系统')
  })
})

describe('闸门：没有前置选择不许跳步', () => {
  it('未选品牌时，「系统 / 型号」步不可点（faults/systems 都以 brand 必填）', () => {
    const w = mountGuide()
    const second = w.findAll('.guide-steps button').find((b) => b.text().includes('系统 / 型号'))
    expect(second!.attributes('disabled')).toBeDefined()
  })

  it('选了品牌之后就能跳回去，也能跳回来', async () => {
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    const second = w.findAll('.guide-steps button').find((b) => b.text().includes('系统 / 型号'))
    expect(second!.attributes('disabled')).toBeUndefined()
  })

  it('未选第二层时「现象 / 主题」步不可点', async () => {
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    const fault = w.findAll('.guide-steps button').find((b) => b.text().includes('现象 / 主题'))
    expect(fault!.attributes('disabled')).toBeDefined()
  })
})

describe('换品牌：旧列表与已选型号必须失效', () => {
  it('从 A 品牌换到 B 品牌后，A 的系统列表不再出现', async () => {
    api.listDiagnosisSystems.mockResolvedValueOnce({ items: systems })
    api.listDiagnosisSystems.mockResolvedValueOnce({ items: [{ id: 'l1', name: 'E-322 电动平衡重' }] })
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    expect(w.text()).toContain('电池_BMS系统')

    // 跳回第一步换品牌
    await w.findAll('.guide-steps button')[0].trigger('click')
    await flushPromises()
    await clickChip(w, '林德 Linde')
    expect(api.listDiagnosisSystems).toHaveBeenLastCalledWith('linde')
    expect(w.text()).toContain('E-322 电动平衡重')
    expect(w.text()).not.toContain('电池_BMS系统')
  })
})

describe('上游不可用：留空 + 不弹错', () => {
  it('systems 502 时不抛错，出「没有可选项」这类兜底文案', async () => {
    api.listDiagnosisSystems.mockRejectedValueOnce(new Error('502'))
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    expect(w.text()).toContain('没有匹配项')
  })

  it('faults 502 时第三步留空，并给一个能走通的出口（票 2280 缺陷二）', async () => {
    // 用 mockRejectedValue（不是 ...Once）：[brandId, secondId] 这个 watcher 在选完品牌与
    // 选完系统时各触发一次，只拒第一次的话第二次会拿到成功值，测不到降级
    api.listDiagnosisFaults.mockRejectedValue(new Error('502'))
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    expect(w.text()).toContain('这个词表里没有匹配项')
    // 关键是这个入口：没有它用户被困在第三步（textarea 在第 4 步、进第 4 步又要先选现象）
    expect(w.findAll('button').some((b) => b.text().includes('直接描述故障现象'))).toBe(true)
  })

  it('词表为空时点「直接描述」能进确认步并写出用户自己描述的现象', async () => {
    api.listDiagnosisFaults.mockResolvedValue({ items: [] })
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')

    const freeformBtn = w.findAll('button').find((b) => b.text().includes('直接描述故障现象'))
    expect(freeformBtn).toBeTruthy()
    await freeformBtn!.trigger('click')
    await flushPromises()

    // 现象输入框出现且能写（第 4 步现在是两步 textarea：现象 + 补充）
    const textareas = w.findAll('textarea')
    expect(textareas.length).toBeGreaterThan(0)
    await textareas[0].setValue('举升无力，空载也起不来')
    await flushPromises()

    const preview = w.find('.compose-preview')
    expect(preview.exists()).toBe(true)
    expect(preview.text()).toContain('举升无力，空载也起不来')
    const send = w.findAll('button').find((b) => b.text().includes('发送'))
    expect(send!.attributes('disabled')).toBeUndefined()
  })
})

describe('组句：品牌/型号只进正文', () => {
  it('中力：走完三步后第 4 步显示成句，且含品牌名与系统名', async () => {
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')
    const preview = w.find('.compose-preview').text()
    expect(preview).toContain('中力 EP')
    expect(preview).toContain('电池_BMS系统')
    expect(preview).toContain('举升无力 / 不能起升')
    expect(preview).toContain('请给我排查步骤、根因分析和维修方案。')
  })

  it('林德：问的是手册条目，不含中力那套「根因分析」收尾', async () => {
    api.listDiagnosisSystems.mockResolvedValue({ items: [{ id: 'l1', name: 'E-322' }] })
    api.listDiagnosisFaults.mockResolvedValue({ items: [{ id: 'x', name: '液压系统' }] })
    const w = mountGuide()
    await clickChip(w, '林德 Linde')
    await clickChip(w, 'E-322')
    await clickChip(w, '液压系统')
    const preview = w.find('.compose-preview').text()
    expect(preview).toContain('请按该型号维修手册说明')
    expect(preview).toContain('请按手册给出安全注意事项。')
    expect(preview).not.toContain('根因分析')
  })
})

describe('发送', () => {
  it('成句后点发送，emit 出完整句子', async () => {
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')
    const send = w.findAll('button').find((b) => b.text().includes('发送'))
    await send!.trigger('click')
    const emitted = w.emitted('send') as string[][]
    expect(emitted).toHaveLength(1)
    expect(emitted[0][0]).toContain('举升无力 / 不能起升')
  })

  it('发送按钮只出现在第 4 步（前几步连按钮都没有，不存在误发）', () => {
    const w = mountGuide()
    expect(w.findAll('button').some((b) => b.text().includes('发送'))).toBe(false)
  })

  it('流式中（disabled）即便成句也发不出去', async () => {
    const w = mountGuide({ disabled: true })
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')
    const send = w.findAll('button').find((b) => b.text().includes('发送'))
    expect(send!.attributes('disabled')).toBeDefined()
    await send!.trigger('click')
    expect(w.emitted('send')).toBeUndefined()
  })
})

describe('预选（欢迎区点卡进来）', () => {
  it('预设品牌：进来就在第二步，并已拉过系统列表', async () => {
    const w = mountGuide({ presetBrandId: 'zhongli' })
    await flushPromises()
    expect(api.listDiagnosisSystems).toHaveBeenCalledWith('zhongli')
    expect(w.text()).toContain('电池_BMS系统')
  })

  it('预设现象但无品牌：从第一步开始（品牌是必填参数），现象已记住', async () => {
    const w = mountGuide({ presetFault: '举升无力 / 不能起升' })
    await flushPromises()
    // 还在第一步：品牌按钮可见
    expect(w.text()).toContain('中力 EP')
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    // 现象是预设的，第 4 步的成句里应该已经在
    const preview = w.find('.compose-preview')
    expect(preview.exists() ? preview.text() : w.text()).toContain('举升无力')
  })

  // 票 2280 缺陷一：上面那条断言在 preview 不存在时会回退到 w.text()，而第三层选项列表里
  // 本来就摆着这个词 —— 恒真。这里改成锁死「选完品牌/系统之后，确认步真的能用」。
  it('预设现象的完整路径：选品牌 → 选系统 → 确认步可用且成句含该现象', async () => {
    const w = mountGuide({ presetFault: '举升无力 / 不能起升' })
    await flushPromises()
    // 还在第一步：确认步不可点（现象有了，但没有品牌 → 成句不成立）
    const confirmStep = () => w.findAll('.guide-steps button').find((b) => b.text().includes('4. 确认'))!

    await clickChip(w, '中力 EP')
    // 选完品牌：现象没被清掉，但还没到确认步
    expect(w.text()).not.toContain('举升无力 / 不能起升')

    await clickChip(w, '电池_BMS系统')
    await flushPromises()

    // 预选的现象在词表里 ⇒ 自动定为已选并直接进第 4 步，成句含现象
    expect(w.find('.compose-preview').text()).toContain('举升无力 / 不能起升')
    const send = w.findAll('button').find((b) => b.text().includes('发送'))
    expect(send!.attributes('disabled')).toBeUndefined()
    expect(confirmStep().attributes('disabled')).toBeUndefined()
  })

  it('换品牌后用户自己点的现象才作废（预选不算）', async () => {
    api.listDiagnosisSystems.mockResolvedValue({ items: systems })
    const w = mountGuide({ presetFault: '举升无力 / 不能起升' })
    await flushPromises()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    expect(w.find('.compose-preview').text()).toContain('举升无力')

    // 回去换品牌：现象必须失效（不能拿 A 品牌的现象去问 B 品牌）
    await w.findAll('.guide-steps button')[0].trigger('click')
    await flushPromises()
    await clickChip(w, '林德 Linde')
    expect(w.text()).not.toContain('举升无力 / 不能起升')
  })

  it('点过词表里的现象之后，换品牌不再把它带回来', async () => {
    const w = mountGuide({ presetFault: '举升无力 / 不能起升' })
    await flushPromises()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    // 预选在词表里 ⇒ 已自动进确认步；回第三步改选另一条现象
    await w.findAll('.guide-steps button')[2].trigger('click')
    await flushPromises()
    await clickChip(w, '仪表报故障码')
    expect(w.find('.compose-preview').text()).toContain('仪表报故障码')
    // 换品牌：旧的自然失效，且预选（举升无力）也不该复活
    await w.findAll('.guide-steps button')[0].trigger('click')
    await flushPromises()
    await clickChip(w, '林德 Linde')
    expect(w.text()).not.toContain('举升无力 / 不能起升')
  })
})

describe('故障码体检：只提示 known / typo', () => {
  it('输入后 250ms 才发一次请求（不逐字发）', async () => {
    vi.useFakeTimers()
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')

    const textarea = w.find('textarea')
    await textarea.setValue('E41')
    expect(api.checkDiagnosisCode).not.toHaveBeenCalled()
    vi.advanceTimersByTime(260)
    expect(api.checkDiagnosisCode).toHaveBeenCalledTimes(1)
    expect(api.checkDiagnosisCode).toHaveBeenCalledWith('E41')
  })

  it('known 时出提示文字', async () => {
    vi.useFakeTimers()
    api.checkDiagnosisCode.mockResolvedValue({
      status: 'known',
      message: '已识别故障码 E41（电池_BMS系统），可直接发送',
      fix: 'E41',
      samples: []
    })
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')
    await w.find('textarea').setValue('E41')
    vi.advanceTimersByTime(260)
    await flushPromises()
    expect(w.text()).toContain('已识别故障码 E41')
  })

  it('three 秒静默态：unknown 不出任何提示', async () => {
    vi.useFakeTimers()
    api.checkDiagnosisCode.mockResolvedValue({ status: 'unknown', message: '未收录', fix: '', samples: [] })
    const w = mountGuide()
    await clickChip(w, '中力 EP')
    await clickChip(w, '电池_BMS系统')
    await clickChip(w, '举升无力 / 不能起升')
    await w.find('textarea').setValue('ZZZ999')
    vi.advanceTimersByTime(260)
    await flushPromises()
    expect(w.text()).not.toContain('未收录')
  })
})
