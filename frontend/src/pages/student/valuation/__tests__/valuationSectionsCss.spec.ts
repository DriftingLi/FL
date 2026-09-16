/**
 * #1056 后半：估值域共享分区块样式的机械验证。
 *
 * jsdom 不渲染真实 CSS，这里对共享样式文件做**文本级断言**（形态对齐 scripts/gate-predicates
 * 的一致性锁）：锁住「零视觉意图变更」承诺的关键排版量取值，并防止视图/卡片把规则抄回去。
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const ROOT = resolve(__dirname, '../../../../..')
const shared = readFileSync(resolve(ROOT, 'src/assets/styles/valuation-view-sections.css'), 'utf-8')

const view = (name: string) =>
  readFileSync(resolve(ROOT, `src/pages/student/valuation/${name}.vue`), 'utf-8')
const card = (name: string) =>
  readFileSync(resolve(ROOT, `src/components/valuation/${name}.vue`), 'utf-8')

describe('valuation-view-sections.css（共享基座）', () => {
  it('三视图分区块：区间距/内边距与收敛前逐字一致', () => {
    expect(shared).toContain(
      ['.valuation-root .valuation-view .radar-block,', '.valuation-root .valuation-view .section-block {']
        .join('\n') + '\n  margin-top: var(--sp-5);\n  padding: var(--sp-6) var(--sp-7);'
    )
  })

  it('分区标题：fs-lg + medium + 下边距 sp-5（收敛前取值）', () => {
    expect(shared).toContain(
      '.valuation-root .valuation-view .section-title {\n  font-size: var(--fs-lg);'
    )
    expect(shared).toContain('margin: 0 0 var(--sp-5);')
  })

  it('电池视图标题档位：flex 排列（图标行）原值保留', () => {
    expect(shared).toContain(
      '.valuation-root .valuation-view.battery-result-view .section-title {\n  display: flex;\n  align-items: center;\n  gap: 8px;'
    )
  })

  it('768px：分区块缩窄 + 雷达图定高 260px（!important 与收敛前一致）', () => {
    expect(shared).toContain('margin-top: var(--sp-4);\n    padding: var(--sp-5) var(--sp-4);')
    expect(shared).toContain('height: 260px !important;')
  })

  it('metric 网格：结构交集 + 两卡档位差异原值保留', () => {
    // 结构交集
    expect(shared).toContain('grid-template-columns: 1fr 1fr 1fr;')
    expect(shared).toContain('.valuation-root .metric {\n  display: flex;\n  flex-direction: column;')
    // 整机卡档位：16px 行距 / 12px 标签 / mono 数字 + tnum
    expect(shared).toContain('.valuation-root .result-card .metric-row {\n  gap: var(--sp-4, 16px);')
    expect(shared).toContain('.valuation-root .result-card .metric {\n  gap: 4px;')
    expect(shared).toContain('.valuation-root .result-card .metric-label {\n  font-size: var(--text-xs, 12px);')
    expect(shared).toContain(
      ".valuation-root .result-card .metric-value {\n  font-weight: var(--fw-semibold, 600);\n  font-family: var(--font-mono, 'JetBrains Mono', monospace);\n  font-feature-settings: 'tnum' 1;"
    )
    // 电池卡档位：sp-5 行距 / 2px 项距 / 14px 标签 / 中等字重
    expect(shared).toContain('.valuation-root .battery-result-card .metric-row {\n  gap: var(--sp-5);')
    expect(shared).toContain('.valuation-root .battery-result-card .metric {\n  gap: 2px;')
    expect(shared).toContain('.valuation-root .battery-result-card .metric-label {\n  font-size: var(--fs-sm);')
    expect(shared).toContain('.valuation-root .battery-result-card .metric-value {\n  font-weight: var(--fw-medium);')
  })

  it('768px：metric 单列 + 数值降档（收敛前取值）', () => {
    expect(shared).toContain('grid-template-columns: 1fr;\n    gap: var(--sp-3, 12px);')
    expect(shared).toContain('font-size: var(--fs-md, 16px);')
  })

  it('作用域纪律：所有规则都在 .valuation-root 内，section-title 有 valuation-view 锚', () => {
    // 剥掉注释再解析（文件头与分节注释会污染 chunk）
    const css = shared.replace(/\/\*[\s\S]*?\*\//g, '')
    // 顶层规则：以 } 切块后，@media 内部规则与其块头粘在同一 chunk——剥掉 @media(...) 头再判
    const chunks = css.split('}').map((c) => c.replace(/@media[^{]*\{/g, '').trim())
    for (const chunk of chunks) {
      if (!chunk.includes('{')) continue
      const selector = chunk.split('{')[0].trim()
      expect(selector.startsWith('.valuation-root'), `选择器越域: ${selector}`).toBe(true)
    }
    expect(shared).toContain('.valuation-view .section-title')
  })
})

describe('收敛后：视图与卡片不得把共享规则抄回去', () => {
  it('三个视图不再定义 .top-row / .radar-block / .section-block / .section-title 规则', () => {
    for (const name of ['ValuationResultView', 'BatteryResultView', 'ValuationReportView']) {
      const src = view(name)
      expect(src, name).not.toMatch(/^\.(top-row|radar-block|section-block|section-title) \{/m)
    }
  })

  it('三个视图根元素带 valuation-view 锚类（共享规则的命中前提）', () => {
    for (const name of ['ValuationResultView', 'BatteryResultView', 'ValuationReportView']) {
      expect(view(name), name).toContain('valuation-root valuation-view')
    }
  })

  it('两张结果卡不再定义 metric 系列规则（强调变体除外）', () => {
    for (const name of ['ResultCard', 'BatteryResultCard']) {
      const src = card(name)
      expect(src, name).not.toMatch(/^\.(metric-row|metric|metric-label|metric-value) \{/m)
    }
    // 仅整机卡保留强调变体
    expect(card('ResultCard')).toContain('.metric-value-accent {')
    expect(card('BatteryResultCard')).not.toContain('.metric-value-accent')
  })
})
