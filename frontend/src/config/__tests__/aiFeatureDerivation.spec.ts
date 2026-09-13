// AI 功能派生面结构锁（ADR-0047 §7 / spec #934 决策 3；代码审查补交）。
//
// 锁的是「前端不再手写注册表已经给出的事实」：
//   1. 路由白名单必须来自生成物 AI_FEATURE_SLUG_PATTERN，pages.ts 里不得再出现手写 slug 字面量；
//   2. 展示数据文件 aiFeatureUI.ts 不得再手写 routePath（它由注册表 slug 派生）；
//   3. 设置页不得再硬编码助手模式键。
//
// 用源码扫描而非运行期断言：这几条约束是「文件里不该有某种写法」，运行期观察不到。
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const read = (rel: string) => readFileSync(resolve(__dirname, '..', rel), 'utf8')

describe('AI 功能派生面：前端不重复注册表已给出的事实', () => {
  it('路由白名单来自生成物，pages.ts 无手写 slug 字面量', () => {
    const pages = read('pages.ts')
    expect(pages).toContain('AI_FEATURE_SLUG_PATTERN')
    for (const slug of ['maintenance', 'drawing', 'exercise', 'fault-diagnosis']) {
      expect(pages).not.toContain(':featureKey(' + slug)
    }
  })

  it('aiFeatureUI.ts 不再手写 routePath（由注册表 slug 派生）', () => {
    const ui = read('aiFeatureUI.ts')
    expect(ui).not.toContain('routePath')
  })

  it('设置页的助手模式键来自生成物，不再硬编码', () => {
    const page = readFileSync(resolve(__dirname, '../../pages/admin/AISettings.vue'), 'utf8')
    expect(page).toContain('AI_ASSISTANT_MODE_KEYS')
    expect(page).not.toContain('=== \'ai_assistant_normal\'')
    expect(page).not.toContain('=== \'ai_assistant_expert\'')
  })
})
