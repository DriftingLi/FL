// 发送验证码交互 module：格式校验、60s 节流倒计时、发送中状态与成功文案的唯一实现。
// interface：send(target, channel)（account_change 目的忽略 target/channel）+ sending + remaining。
// 格式校验引用 utils/validate.ts 单一事实源（邮箱与后端 net/mail 语义对齐）。
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useCountdown } from '@/composables/useCountdown'
import { isValidEmail, isValidPhone } from '@/utils/validate'

export type SendCodePurpose = 'register' | 'login' | 'bind' | 'account_change' | 'reset_password'
export type SendCodeChannel = 'email' | 'phone'

const SUCCESS_MESSAGE: Record<SendCodePurpose, string> = {
  register: '验证码已发送，请查收',
  login: '验证码已发送，请查收',
  bind: '验证码已发送，请查收',
  account_change: '验证码已发送至绑定手机号，请查收',
  reset_password: '验证码已发送，请查收'
}

export interface UseSendCodeOptions {
  purpose: SendCodePurpose
  /** 发送实现（data-source adapter）：channel + target → 对应 API 调用；错误由拦截器统一提示 */
  sendCode: (channel: SendCodeChannel, target: string) => Promise<unknown>
}

export function useSendCode(options: UseSendCodeOptions) {
  const { purpose, sendCode } = options
  const sending = ref(false)
  const countdown = useCountdown()

  async function send(target: string, channel: SendCodeChannel): Promise<boolean> {
    if (purpose !== 'account_change') {
      const invalid = channel === 'phone' ? !isValidPhone(target) : !isValidEmail(target)
      if (invalid) {
        ElMessage.warning(channel === 'phone' ? '请输入正确的手机号' : '请输入正确的邮箱地址')
        return false
      }
    }
    sending.value = true
    try {
      await sendCode(channel, target)
      ElMessage.success(SUCCESS_MESSAGE[purpose])
      countdown.start(60)
      return true
    } catch (e) {
      return false
    } finally {
      sending.value = false
    }
  }

  return { sending, remaining: countdown.remaining, send }
}
