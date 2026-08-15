// 验证码确认对话框状态机 module：可见性/target/code/sending/submitting 状态 + 6 位验证码校验
// + 提交成功回调的统一实现。interface 构造期注入 { sendCode, submitAsync }（可带 onSuccess）。
// 发送+倒计时交互仍由 useSendCode 负责（本 module 不重复）；这里只吸收对话框自身的状态与提交收敛。
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import type { SendCodeChannel } from '@/composables/useSendCode'

const INVALID_CODE_MESSAGE = '请输入6位验证码'

export interface UseVerifyDialogOptions {
  /** 发送验证码实现（data-source adapter）：target + channel → 现有 useSendCode 发送语义 */
  sendCode: (target: string, channel: SendCodeChannel) => Promise<unknown>
  /** 提交实现（data-source adapter）：target + 6 位 code → 对应 API 调用；错误由拦截器统一提示 */
  submitAsync: (target: string, code: string) => Promise<unknown>
  /** 提交成功后的额外副作用（默认无）；「提交后刷新资料」等规则统一在此注入表达 */
  onSuccess?: () => Promise<void> | void
}

export function useVerifyDialog(options: UseVerifyDialogOptions) {
  const { submitAsync, onSuccess } = options
  const sendCode = options.sendCode

  const visible = ref(false)
  const target = ref('')
  const code = ref('')
  const sending = ref(false)
  const submitting = ref(false)

  function open() {
    target.value = ''
    code.value = ''
    visible.value = true
  }

  function close() {
    visible.value = false
  }

  async function send(value: string, channel: SendCodeChannel): Promise<void> {
    sending.value = true
    try {
      await sendCode(value.trim(), channel)
    } finally {
      sending.value = false
    }
  }

  async function submit(): Promise<void> {
    const trimmedCode = code.value.trim()
    if (!/^\d{6}$/.test(trimmedCode)) {
      ElMessage.warning(INVALID_CODE_MESSAGE)
      return
    }
    submitting.value = true
    try {
      await submitAsync(target.value.trim(), trimmedCode)
      visible.value = false
      await onSuccess?.()
    } catch (e) {
      // 拦截器已统一提示；提交失败保持对话框打开
    } finally {
      submitting.value = false
    }
  }

  return { visible, target, code, sending, submitting, open, close, send, submit }
}
