// JobManage（招聘端「职位管理」）的「需持证件」多选（#2066 后端 + #2073 Web）。
// 票面三条验收：多选提交携带数组 / 编辑态回显已选项 / 清空后提交为空（等价「不限」）。
// seam：页面组件层，mock `@/api/job` / `@/api/position` / `@/api/credential`（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const ElMessage = vi.hoisted(() => ({ warning: vi.fn(), error: vi.fn(), success: vi.fn() }))
// 页面从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响
vi.mock('element-plus', () => ({ ElMessage }))

vi.mock('@/api/job', () => ({
  jobApi: {
    listMyJobs: vi.fn(),
    createJob: vi.fn(),
    updateJob: vi.fn(),
    toggleJobStatus: vi.fn()
  }
}))
vi.mock('@/api/position', () => ({ positionApi: { listPublic: vi.fn() } }))
vi.mock('@/api/credential', () => ({ credentialApi: { listCredentials: vi.fn() } }))

import { jobApi, type JobPosting } from '@/api/job'
import { positionApi } from '@/api/position'
import { credentialApi, type CredentialDict } from '@/api/credential'
import UiButton from '@/components/ui/UiButton.vue'
import UiCheckboxGroup from '@/components/ui/UiCheckboxGroup.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import JobManage from '../JobManage.vue'

const CRED_DRIVER = 11
const CRED_REPAIR = 12

function credentialOf(id: number, name: string): CredentialDict {
  return {
    id,
    code: `CRED_${id}`,
    name,
    category: 'special_operation',
    level: null,
    description: '',
    sort_order: id,
    status: 1,
    created_at: '2026-10-01 10:00',
    updated_at: '2026-10-01 10:00'
  }
}

function jobOf(over: Partial<JobPosting> = {}): JobPosting {
  return {
    id: 1,
    recruiter_id: 1,
    title: '叉车维修技师',
    position_id: 1,
    position_name: '叉车维修技师',
    region: '江苏苏州',
    salary_text: '6-9K',
    experience_req: '2年',
    description: '负责叉车日常维修保养',
    status: 'open',
    forced_offline: false,
    published_at: '2026-10-01 10:00',
    created_at: '2026-10-01 10:00',
    updated_at: '2026-10-01 10:00',
    required_credentials: [],
    ...over
  }
}

function mountPage() {
  // 列表行里的「投递列表」是 router-link：测试不装路由，stub 掉（与其它页面 spec 同口径）
  return mount(JobManage, { global: { plugins: [epLite()], stubs: { RouterLink: true } } })
}

/** 点某个文案的 UiButton（页面里的「发布职位」/「编辑」都是封装层按钮）。 */
async function clickButton(w: ReturnType<typeof mountPage>, text: string) {
  const btn = w.findAllComponents(UiButton).find((b) => b.text().trim() === text)
  expect(btn, `应有「${text}」按钮`).toBeTruthy()
  await btn!.trigger('click')
  await flushPromises()
}

/** 表单必填项：职位名 + 岗位（不填会被既有校验拦下，与证件无关）。 */
async function fillRequired(w: ReturnType<typeof mountPage>) {
  w.findAllComponents(UiInput)[0].vm.$emit('update:modelValue', '叉车维修技师')
  w.findComponent(UiSelect).vm.$emit('update:modelValue', 1)
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(jobApi.listMyJobs).mockResolvedValue({ items: [jobOf()], total: 1 } as never)
  vi.mocked(jobApi.createJob).mockResolvedValue(jobOf() as never)
  vi.mocked(jobApi.updateJob).mockResolvedValue(jobOf() as never)
  vi.mocked(positionApi.listPublic).mockResolvedValue({
    positions: [{ position_id: 1, name: '叉车维修技师' }]
  } as never)
  vi.mocked(credentialApi.listCredentials).mockResolvedValue({
    credentials: [credentialOf(CRED_DRIVER, '叉车司机证'), credentialOf(CRED_REPAIR, '叉车维修证')]
  } as never)
})

describe('JobManage 需持证件多选（#2073）', () => {
  it('发布时勾选的证件随提交携带 ID 数组', async () => {
    const w = mountPage()
    await flushPromises()

    await clickButton(w, '发布职位')
    await fillRequired(w)

    const group = w.findComponent(UiCheckboxGroup)
    expect(group.exists(), '表单里应有「需持证件」多选组').toBe(true)
    // 选项来自培训域证件字典（复用，不建第二套）
    expect(group.text()).toContain('叉车司机证')
    expect(group.text()).toContain('叉车维修证')

    group.vm.$emit('update:modelValue', [CRED_DRIVER, CRED_REPAIR])
    await flushPromises()
    w.findComponent(UiDialog).vm.$emit('confirm')
    await flushPromises()

    expect(jobApi.createJob).toHaveBeenCalledTimes(1)
    const payload = vi.mocked(jobApi.createJob).mock.calls[0][0]
    expect(payload.required_credential_ids).toEqual([CRED_DRIVER, CRED_REPAIR])
  })

  it('编辑态回显已选证件（DTO 的 required_credentials → 勾选态）', async () => {
    vi.mocked(jobApi.listMyJobs).mockResolvedValue({
      items: [jobOf({ required_credentials: [{ credential_id: CRED_REPAIR, name: '叉车维修证' }] })],
      total: 1
    } as never)

    const w = mountPage()
    await flushPromises()
    await clickButton(w, '编辑')

    expect(w.findComponent(UiCheckboxGroup).props('modelValue')).toEqual([CRED_REPAIR])
  })

  it('全部取消勾选后提交为空数组（等价「不限证件」）', async () => {
    vi.mocked(jobApi.listMyJobs).mockResolvedValue({
      items: [jobOf({
        required_credentials: [
          { credential_id: CRED_DRIVER, name: '叉车司机证' },
          { credential_id: CRED_REPAIR, name: '叉车维修证' }
        ]
      })],
      total: 1
    } as never)

    const w = mountPage()
    await flushPromises()
    await clickButton(w, '编辑')

    const group = w.findComponent(UiCheckboxGroup)
    expect(group.props('modelValue')).toEqual([CRED_DRIVER, CRED_REPAIR])

    group.vm.$emit('update:modelValue', [])
    await flushPromises()
    w.findComponent(UiDialog).vm.$emit('confirm')
    await flushPromises()

    expect(jobApi.updateJob).toHaveBeenCalledTimes(1)
    const payload = vi.mocked(jobApi.updateJob).mock.calls[0][1]
    expect(payload.required_credential_ids).toEqual([])
  })

  it('职位没有证件要求时是空勾选态，不是 undefined', async () => {
    const w = mountPage()
    await flushPromises()
    await clickButton(w, '编辑')

    expect(w.findComponent(UiCheckboxGroup).props('modelValue')).toEqual([])
  })
})
