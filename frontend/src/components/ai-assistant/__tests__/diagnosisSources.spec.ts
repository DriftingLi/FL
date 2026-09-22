// DiagnosisSources.vue 首份 spec（该文件此前无测试，却是三端图片标记解析面里最薄的一处：
// 移动端有 aiSourcesDisplay.test.js 镜像、后端有 proxy/wire 夹具，Web 这份什么都没有）。
// 断言的是**真实渲染产物**（img 的 src、文本残留），不是 prop 传递 —— 先例 ForumContent.spec。
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

describe('DiagnosisSources 图片标记与来源链接（20260921 双静态根）', () => {
  it('旧形状标记（/assistant/static/manual/…）剥前缀后进本站 manual 代理', () => {
    const w = mountSources([
      source('fault-1', '检查蓄能器压力 <<IMAGE:/assistant/static/manual/ep_doc/page_86_2339.png>>')
    ])
    expect(w.find('img').attributes('src')).toBe(
      '/api/ai-assistant/diagnosis/manual/ep_doc/page_86_2339.png'
    )
    // 代理 URL 是本站相对路径：正文里不该再留标记原文与助手内网绝对前缀
    expect(w.text()).not.toContain('<<IMAGE')
    expect(w.text()).not.toContain('/assistant/static/')
  })

  it('案例图（fault_images 根 + 中文目录）保留根名并逐段转义', () => {
    const w = mountSources([
      source('case-31', '更换制动片 <<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png>>')
    ])
    expect(w.find('img').attributes('src')).toBe(
      '/api/ai-assistant/diagnosis/manual/fault_images/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/1721219449286.png'
    )
  })

  it('标记可带「| 描述:」后缀（交付方 postprocess 按第一个 | 截断），后缀不得进 URL', () => {
    const w = mountSources([
      source('case-32', '排气操作 <<IMAGE:/assistant/static/fault_images/制动系统/a.png | 描述:蓄能器接口>>')
    ])
    expect(w.find('img').attributes('src')).toBe(
      '/api/ai-assistant/diagnosis/manual/fault_images/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/a.png'
    )
    expect(w.text()).not.toContain('描述:蓄能器接口')
  })

  it('PDF 来源链接走代理，且 #page 锚点保留', () => {
    const w = mountSources([
      source('fault-2', '正文', '/assistant/static/manual/ep_doc/ep_doc.pdf#page=448')
    ])
    expect(w.get('a').attributes('href')).toBe(
      '/api/ai-assistant/diagnosis/manual/ep_doc/ep_doc.pdf#page=448'
    )
  })

  it('无来源链接也无页码时给「暂无来源资料」，不留空链接', () => {
    const w = mountSources([source('fault-3', '只有文字')])
    expect(w.find('a').exists()).toBe(false)
    expect(w.text()).toContain('暂无来源资料')
  })
})
