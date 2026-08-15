// 图形验证码交互 module：加载/刷新图片、持有 id 与用户输入（三处登录/注册/找回页面共用）。
import { ref } from 'vue'
import { authApi } from '@/api/auth'

export function useCaptcha() {
  const captchaId = ref('')
  const captchaImage = ref('')
  const captchaValue = ref('')
  const captchaLoading = ref(false)

  async function refreshCaptcha() {
    captchaLoading.value = true
    try {
      const data = await authApi.getCaptcha()
      captchaId.value = data.id
      captchaImage.value = data.image
      captchaValue.value = ''
    } catch (e) {
      // 拦截器已提示
    } finally {
      captchaLoading.value = false
    }
  }

  return { captchaId, captchaImage, captchaValue, captchaLoading, refreshCaptcha }
}
