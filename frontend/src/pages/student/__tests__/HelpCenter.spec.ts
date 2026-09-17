// 帮助中心（HelpCenter.vue）契约测试（#1079）：
// 1) 首屏一次拉整页并渲染分类与条目；
// 2) 搜索走**端上过滤**（不新增搜索接口）：问题或答案命中即显示，跨分类；
// 3) 左侧分类点击只显示该分类；
// 4) 空态。
// seam：组件层，mock '@/api/faq'（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/faq', () => ({
  faqApi: { getHelpCenter: vi.fn() }
}))

import { faqApi } from '@/api/faq'
import HelpCenter from '../HelpCenter.vue'

const payload = {
  categories: [
    {
      code: 'account',
      title: '账号与登录',
      id: 1,
      sort_order: 1,
      entries: [
        { id: 11, question: '怎么注册学员账号？', answer: '用手机号或邮箱验证码注册。', sort_order: 1 },
        { id: 12, question: '忘记密码怎么办？', answer: '走忘记密码找回。', sort_order: 2 }
      ]
    },
    {
      code: 'points',
      title: '积分与任务',
      id: 2,
      sort_order: 2,
      entries: [{ id: 21, question: '积分怎么获得？', answer: '每日任务与打卡都会给积分。', sort_order: 1 }]
    }
  ]
}

function mountPage() {
  return mount(HelpCenter, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.mocked(faqApi.getHelpCenter).mockReset()
  vi.mocked(faqApi.getHelpCenter).mockResolvedValue(payload as never)
})

describe('帮助中心 渲染（#1079）', () => {
  it('首屏一次拉整页，渲染左侧分类与右侧问题', async () => {
    const w = mountPage()
    await flushPromises()
    expect(faqApi.getHelpCenter).toHaveBeenCalledTimes(1)
    const text = w.text()
    expect(text).toContain('账号与登录')
    expect(text).toContain('积分与任务')
    expect(text).toContain('怎么注册学员账号？')
    expect(text).toContain('积分怎么获得？')
  })
})

describe('帮助中心 端上搜索（#1079）', () => {
  it('关键词命中问题文本：只显示命中的条目，且跨分类', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('input').setValue('积分')
    await flushPromises()
    const text = w.text()
    expect(text).toContain('积分怎么获得？')
    expect(text).not.toContain('怎么注册学员账号？')
  })

  it('关键词命中答案文本也算命中', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('input').setValue('验证码')
    await flushPromises()
    const text = w.text()
    expect(text).toContain('怎么注册学员账号？')
    expect(text).not.toContain('积分怎么获得？')
  })

  it('无命中时走空态文案', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('input').setValue('不存在的关键词xyz')
    await flushPromises()
    expect(w.text()).toContain('没有匹配的问题')
  })
})

describe('帮助中心 分类筛选（#1079）', () => {
  it('点左侧分类只显示该分类下的条目', async () => {
    const w = mountPage()
    await flushPromises()
    // 分类导航项是 button（不是 div）—— 用 button 定位，否则点到外层容器上什么也不会发生
    const nav = w.findAll('button').find((b) => b.text().includes('积分与任务'))
    expect(nav).toBeTruthy()
    await nav!.trigger('click')
    await flushPromises()
    const text = w.text()
    expect(text).toContain('积分怎么获得？')
    expect(text).not.toContain('怎么注册学员账号？')
  })
})
