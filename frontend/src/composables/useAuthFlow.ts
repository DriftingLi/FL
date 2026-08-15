// useAuthFlow：认证页「mode 状态 + validate→submit→afterSuccess」顺序状态机 composable。
// 吸收三张认证页（Login/Register/ForgotPassword）共用的提交流程：
//   handleSubmit(formRef) → formRef.validate()（失败短路，不触发 submit）
//   → loading=true → await submit(mode) → await afterSuccess(mode, result) → finally loading=false。
// 异常不抛给页面（console.error 记录，与页面原 catch 行为一致）。
// 约定：submit 返回 false 视为「阻止后续」（如 wechat 未开放），不执行 afterSuccess。
import { ref } from 'vue'

// 表单实例的极简结构 seam：Element Plus FormInstance（validate(可选回调) => Promise<boolean>）
// 天然满足该结构，单测可用内存 fake 直接注入，避免与 element-plus 实现耦合。
export interface ValidatableFormRef {
  validate: () => Promise<boolean>
}

export interface UseAuthFlowOptions<Mode extends string = string> {
  /** 允许的 mode 列表；mode 默认取首项 */
  modes: readonly Mode[]
  /** 提交实现（data-source adapter）：按当前 mode 执行各自的 authApi 调用；可返回业务数据或 false 阻止后续 */
  submit: (mode: Mode) => Promise<unknown> | unknown
  /** 成功后回调：setAuthData / 成功提示 / 跳转；receive 为 submit 返回的业务数据 */
  afterSuccess: (mode: Mode, result?: unknown) => Promise<unknown> | unknown
}

export function useAuthFlow<Mode extends string>(options: UseAuthFlowOptions<Mode>) {
  const { modes, submit, afterSuccess } = options

  const mode = ref<Mode>(modes[0])
  const loading = ref(false)

  /** 切换当前认证方式 */
  function setMode(next: Mode): void {
    mode.value = next
  }

  /** 表单提交入口：validate 通过后依次执行 submit → afterSuccess，全程异常不外抛 */
  async function handleSubmit(formRef: ValidatableFormRef | null | undefined): Promise<void> {
    if (!formRef || typeof formRef.validate !== 'function') return
    const valid = await formRef.validate().catch(() => false)
    if (!valid) return

    loading.value = true
    try {
      const result = await submit(mode.value)
      // submit 返回 false（如 wechat 未开放）视为阻止后续，不执行 afterSuccess
      if (result === false) return
      await afterSuccess(mode.value, result)
    } catch (e) {
      console.error('[useAuthFlow]', e)
    } finally {
      loading.value = false
    }
  }

  return { mode, setMode, loading, handleSubmit }
}
