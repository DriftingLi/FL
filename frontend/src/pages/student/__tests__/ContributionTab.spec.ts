// ContributionTab 学员投稿 tab（#517）：广场列表 / 我的投稿状态徽章 / 上传表单校验。
// seam：组件层 mock @/api/contribution（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/contribution', () => ({
  contributionApi: {
    listPublic: vi.fn(),
    listMine: vi.fn(),
    detail: vi.fn(),
    uploadFile: vi.fn(),
    create: vi.fn(),
    download: vi.fn(),
    withdraw: vi.fn(),
    report: vi.fn()
  }
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({
    current: { id: 1 }
  })
}))

import { contributionApi } from '@/api/contribution'
import ContributionTab from '../ContributionTab.vue'

function mountTab(props = {}) {
  return mount(ContributionTab, {
    props: { credentialId: 1, ...props },
    global: {
      plugins: [ElementPlus],
      stubs: { RouterLink: { template: '<a><slot /></a>' } }
    }
  })
}

function contributionItem(over: Record<string, unknown> = {}): import('@/api/contribution').ContributionItem {
  return {
    id: 1,
    credential_id: 1,
    title: '叉车液压手册',
    intro: '一线维修整理',
    status: 'approved',
    is_anonymous: false,
    downloads_count: 12,
    created_at: '2026-09-01T10:00:00+08:00',
    author: { user_id: 7, username: '小明' },
    // 列表契约：ListPublic 不加载 files（摘要形状）——详情靠 detail 懒加载（回归守卫见详情用例）
    ...(over as object)
  }
}

beforeEach(() => {
  vi.mocked(contributionApi.listPublic).mockResolvedValue({ items: [contributionItem()], total: 1, page: 1, page_size: 20 })
  vi.mocked(contributionApi.listMine).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  vi.mocked(contributionApi.detail).mockResolvedValue(contributionItem({ files: [{ file_id: 1, file_name: '液压手册.pdf', file_url: '/static/uploads/contributions/a.pdf', file_size: 1024, content_type: 'document' }] }))
  vi.mocked(contributionApi.download).mockResolvedValue({ is_new: true, tier_awarded: 0 })
})

describe('ContributionTab 学员投稿（#517）', () => {
  it('广场展示过审投稿（标题/作者/下载量）', async () => {
    const w = mountTab()
    await flushPromises()
    expect(w.text()).toContain('叉车液压手册')
    expect(w.text()).toContain('小明')
    expect(w.text()).toContain('12 次下载')
  })

  it('广场按当前证件调 listPublic', async () => {
    mountTab()
    await flushPromises()
    expect(vi.mocked(contributionApi.listPublic)).toHaveBeenCalledWith(expect.objectContaining({ credential_id: 1 }))
  })

  it('「我的投稿」显示状态徽章与撤回按钮（pending）', async () => {
    vi.mocked(contributionApi.listMine).mockResolvedValue({
      items: [contributionItem({ id: 2, status: 'pending', title: '待审稿', downloads_count: 0 })],
      total: 1, page: 1, page_size: 20
    })
    const w = mountTab()
    await flushPromises()
    const btns = w.findAll('button')
    const mineBtn = btns.find((b) => b.text().includes('我的投稿'))
    expect(mineBtn).toBeTruthy()
    await mineBtn!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('待审稿')
    expect(w.text()).toContain('审核中')
    expect(w.text()).toContain('撤回')
  })

  it('匿名投稿显示「匿名学员」', async () => {
    vi.mocked(contributionApi.listPublic).mockResolvedValue({
      items: [contributionItem({ id: 3, is_anonymous: true, author: { user_id: 8, username: '', anonymous: true } })],
      total: 1, page: 1, page_size: 20
    })
    const w = mountTab()
    await flushPromises()
    expect(w.text()).toContain('匿名学员')
  })

  it('广场卡片有「查看」入口（列表不带 files 也不受影响——回归守卫）', async () => {
    const w = mountTab()
    await flushPromises()
    const viewBtn = w.findAll('button').find((b) => b.text().includes('查看'))
    expect(viewBtn).toBeTruthy()
    expect(vi.mocked(contributionApi.detail)).not.toHaveBeenCalled()
    await viewBtn!.trigger('click')
    await flushPromises()
    expect(vi.mocked(contributionApi.detail)).toHaveBeenCalledWith(1)
    // 详情对话框（append-to-body）含文件名与下载按钮
    expect(document.body.textContent).toContain('液压手册.pdf')
  })

  it('详情内下载：先计端点再开直链', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const w = mountTab()
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('查看'))!.trigger('click')
    await flushPromises()
    const btns = Array.from(document.querySelectorAll('button'))
    const dlBtn = btns.find((b) => b.textContent?.includes('下载'))
    expect(dlBtn).toBeTruthy()
    dlBtn!.dispatchEvent(new MouseEvent('click'))
    await flushPromises()
    expect(vi.mocked(contributionApi.download)).toHaveBeenCalledWith(1)
    expect(openSpy).toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it('上传抽屉：空标题被拦（不调 create）', async () => {
    const w = mountTab()
    await flushPromises()
    const uploadBtn = w.findAll('button').find((b) => b.text().includes('上传资料'))
    await uploadBtn!.trigger('click')
    await flushPromises()
    // el-drawer append-to-body：内容在 document.body
    const submitBtn = Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.includes('提交审核'))
    expect(submitBtn).toBeTruthy()
    submitBtn!.dispatchEvent(new MouseEvent('click'))
    await flushPromises()
    expect(vi.mocked(contributionApi.create)).not.toHaveBeenCalled()
  })
})
