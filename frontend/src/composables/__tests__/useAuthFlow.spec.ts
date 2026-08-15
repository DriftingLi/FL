// useAuthFlow：认证页「mode 状态 + validate→submit→afterSuccess」顺序状态机 composable 的接口级测试。
// seam：public interface（useAuthFlow 返回的 mode/setMode/loading/handleSubmit）。
// fake submit/afterSuccess；fake formRef 仅暴露 validate()。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAuthFlow } from '@/composables/useAuthFlow'

type FlowOptions = Parameters<typeof useAuthFlow>[0]

// 内存 fake 表单 ref：仅实现 validate()（Element Plus FormInstance 的结构子集）
function makeFormRef(behavior: 'ok' | 'false' | 'reject'): { validate: () => Promise<boolean> } {
  const validate =
    behavior === 'reject'
      ? () => Promise.reject({ username: { message: '必填' } })
      : behavior === 'false'
        ? () => Promise.resolve(false)
        : () => Promise.resolve(true)
  return { validate }
}

type SubmitMock = ReturnType<typeof vi.fn<(m: string) => Promise<unknown>>>
type AfterSuccessMock = ReturnType<typeof vi.fn<(m: string, result?: unknown) => Promise<unknown>>>

function mount(opts?: {
  modes?: string[]
  submit?: FlowOptions['submit']
  afterSuccess?: FlowOptions['afterSuccess']
}) {
  const submit = (opts?.submit ?? vi.fn<(m: string) => Promise<unknown>>().mockResolvedValue('result-value')) as SubmitMock
  const afterSuccess = (opts?.afterSuccess ?? vi.fn<(m: string, result?: unknown) => Promise<unknown>>().mockResolvedValue(undefined)) as AfterSuccessMock
  const flow = useAuthFlow({
    modes: opts?.modes ?? ['m1', 'm2'],
    submit: submit as FlowOptions['submit'],
    afterSuccess: afterSuccess as FlowOptions['afterSuccess']
  })
  return { flow, submit, afterSuccess }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('mode 状态', () => {
  it('mode 默认取 modes[0] 首项', () => {
    const { flow } = mount({ modes: ['m1', 'm2'] })
    expect(flow.mode.value).toBe('m1')
  })

  it('setMode 切换当前 mode', () => {
    const { flow } = mount({ modes: ['m1', 'm2'] })
    flow.setMode('m2')
    expect(flow.mode.value).toBe('m2')
  })
})

describe('handleSubmit（validate→submit→afterSuccess）', () => {
  it('validate 失败（reject）不调 submit，loading 复位为 false', async () => {
    const { flow, submit } = mount()
    await flow.handleSubmit(makeFormRef('reject'))
    expect(submit).not.toHaveBeenCalled()
    expect(flow.loading.value).toBe(false)
  })

  it('validate 失败（resolve false）不调 submit，loading 复位为 false', async () => {
    const { flow, submit } = mount()
    await flow.handleSubmit(makeFormRef('false'))
    expect(submit).not.toHaveBeenCalled()
    expect(flow.loading.value).toBe(false)
  })

  it('validate 通过：先 submit 后 afterSuccess，且按当前 mode 传参', async () => {
    const { flow, submit, afterSuccess } = mount()
    flow.setMode('m2')
    await flow.handleSubmit(makeFormRef('ok'))
    expect(submit).toHaveBeenCalledTimes(1)
    expect(submit).toHaveBeenCalledWith('m2')
    expect(afterSuccess).toHaveBeenCalledTimes(1)
    // afterSuccess 在 submit 之后
    expect(submit.mock.invocationCallOrder[0]).toBeLessThan(afterSuccess.mock.invocationCallOrder[0])
    expect(flow.loading.value).toBe(false)
  })

  it('submit 成功时把返回值传给 afterSuccess', async () => {
    const { flow, afterSuccess } = mount()
    await flow.handleSubmit(makeFormRef('ok'))
    expect(afterSuccess).toHaveBeenCalledWith('m1', 'result-value')
  })

  it('submit 抛错：不调 afterSuccess，loading 复位为 false', async () => {
    const submit = vi.fn<(m: string) => Promise<unknown>>().mockRejectedValue(new Error('submit failed'))
    const afterSuccess = vi.fn<(m: string, result?: unknown) => Promise<unknown>>()
    const { flow } = mount({ submit, afterSuccess })
    await flow.handleSubmit(makeFormRef('ok'))
    expect(afterSuccess).not.toHaveBeenCalled()
    expect(flow.loading.value).toBe(false)
  })

  it('submit 返回 false 视为「阻止后续」：不调 afterSuccess，loading 复位为 false', async () => {
    const submit = vi.fn<(m: string) => Promise<unknown>>().mockResolvedValue(false)
    const afterSuccess = vi.fn<(m: string, result?: unknown) => Promise<unknown>>()
    const { flow } = mount({ submit, afterSuccess })
    await flow.handleSubmit(makeFormRef('ok'))
    expect(afterSuccess).not.toHaveBeenCalled()
    expect(flow.loading.value).toBe(false)
  })

  it('formRef 为空时直接返回，不调 validate/submit/afterSuccess', async () => {
    const { flow, submit, afterSuccess } = mount()
    await flow.handleSubmit(undefined)
    expect(submit).not.toHaveBeenCalled()
    expect(afterSuccess).not.toHaveBeenCalled()
  })
})
