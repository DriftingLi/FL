// DiagnosisSources.vue spec：v1 通道退役后（票 2237）来源面板**只回放文字**。
//
// 退役前提：`<<IMAGE:…>>` 标记指向的助手静态资源（/assistant/static/manual/…、
// /assistant/static/fault_images/…）随旧代理一起下线，PDF 原文外链是同一批内网路径。
// 因此本组件的正确行为是：标记剥掉、不产出任何 URL、不留坏图。
// v2 的回答配图在**正文里**（adapter 已归一成 /diagnosis/ossimg/*），不经本组件。
//
// 断言的是真实渲染产物（img / a 是否存在、文本残留），不是 prop 传递 —— 先例 ForumContent.spec。
import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DiagnosisSources from '../DiagnosisSources.vue'
import { epLite } from '@/test/element-lite'
import type { DiagnosisSource } from '@/api/aiAssistant'

// CSS 导入在 vitest 下无意义，替身掉（与 AI 域既有做法一致）
vi.mock('markstream-vue/index.css', () => ({}))

function source(id: string, text: string, sourceUrl = ''): DiagnosisSource {
  return { id, text, metadata: { source_url: sourceUrl, page_start: 0, page_end: 0 } }
}

function mountSources(sources: DiagnosisSource[]) {
  return mount(DiagnosisSources, {
    props: { sources, embedded: true },
    global: { plugins: [epLite()] }
  })
}

describe('DiagnosisSources：退役后只回放文字，不产出 URL', () => {
  it('旧形状标记（/assistant/static/manual/…）被剥掉，且不渲染 <img>', () => {
    const w = mountSources([
      source('fault-1', '检查蓄能器压力 <<IMAGE:/assistant/static/manual/ep_doc/page_86_2339.png>>')
    ])
    expect(w.find('img').exists()).toBe(false)
    expect(w.text()).toContain('检查蓄能器压力')
    expect(w.text()).not.toContain('<<IMAGE')
    expect(w.text()).not.toContain('/assistant/static/')
    // 关键：不得再拼已下线的 manual 代理 URL
    expect(w.html()).not.toContain('/diagnosis/manual/')
  })

  it('案例图标记（fault_images 根 + 中文目录）同样剥掉，不产出坏图', () => {
    const w = mountSources([
      source('case-31', '更换制动片 <<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png>>')
    ])
    expect(w.find('img').exists()).toBe(false)
    expect(w.text()).toContain('更换制动片')
    expect(w.html()).not.toContain('fault_images')
    expect(w.html()).not.toContain('/diagnosis/manual/')
  })

  it('标记可带「| 描述:」后缀：整条标记连后缀一起剥掉', () => {
    const w = mountSources([
      source('case-32', '排气操作 <<IMAGE:/assistant/static/fault_images/制动系统/a.png | 描述:蓄能器接口>>')
    ])
    expect(w.find('img').exists()).toBe(false)
    expect(w.text()).toContain('排气操作')
    expect(w.text()).not.toContain('描述:蓄能器接口')
  })

  it('PDF 来源不再产出「打开 PDF 来源」链接（上游内网路径已下线）', () => {
    const w = mountSources([
      source('fault-2', '正文', '/assistant/static/manual/ep_doc/ep_doc.pdf#page=448')
    ])
    expect(w.find('a').exists()).toBe(false)
    expect(w.html()).not.toContain('/diagnosis/manual/')
  })

  it('有页码时仍显示页码；无来源链接也无页码时给「暂无来源资料」，不留空链接', () => {
    const withPage = mountSources([
      { id: 'fault-4', text: '正文', metadata: { source_url: '', page_start: 448, page_end: 448 } }
    ])
    expect(withPage.text()).toContain('第 448 页')

    const empty = mountSources([source('fault-3', '只有文字')])
    expect(empty.find('a').exists()).toBe(false)
    expect(empty.text()).toContain('暂无来源资料')
  })
})
