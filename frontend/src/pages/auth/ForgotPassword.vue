<template>
  <AuthPageShell
    title="找回密码"
    subtitle="通过验证码重置登录密码"
    badge-text="学员端"
    badge-tone="student"
    :alt-modes="altModes"
    :active-alt="activeAlt"
    back-label="返回手机号找回"
    divider-text="或使用以下方式重置"
    @select-alt="onSelectAlt"
  >
    <template #main>
      <el-form
        ref="mainFormRef"
        :model="formData"
        :rules="mainFieldRules"
        label-width="0"
        class="auth-form"
        @submit.prevent
      >
        <el-form-item prop="phone">
          <el-input
            v-model="formData.phone"
            placeholder="请输入注册手机号"
            prefix-icon="Phone"
            size="large"
            class="form-input"
            maxlength="11"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <div class="captcha-row">
            <el-input
              v-model="captchaValue"
              placeholder="图形验证码"
              size="large"
              class="form-input captcha-input"
              maxlength="4"
              @keyup.enter="handleSendCode"
            />
            <img
              v-if="captchaImage"
              :src="captchaImage"
              class="captcha-img"
              alt="验证码"
              title="看不清？点击刷新"
              @click="refreshCaptcha"
            />
          </div>
        </el-form-item>

        <el-form-item prop="code">
          <div class="code-row">
            <el-input
              v-model="formData.code"
              placeholder="6位手机验证码"
              prefix-icon="Message"
              size="large"
              class="form-input code-input"
              maxlength="6"
              @keyup.enter="handleSubmit"
            />
            <UiButton :disabled="countdown > 0 || codeSending" size="large" class="code-btn" @click="handleSendCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </UiButton>
          </div>
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="formData.password"
            type="password"
            placeholder="新密码（6-20位）"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
          />
        </el-form-item>

        <el-form-item prop="confirmPassword">
          <el-input
            v-model="formData.confirmPassword"
            type="password"
            placeholder="确认新密码"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleSubmit">
            {{ flow.loading ? '提交中...' : '重置密码' }}
          </UiButton>
        </el-form-item>
      </el-form>
    </template>

    <template #alt-email>
      <el-form
        ref="emailFormRef"
        :model="formData"
        :rules="emailFieldRules"
        label-width="0"
        class="auth-form"
        @submit.prevent
      >
        <el-form-item prop="email">
          <el-input
            v-model="formData.email"
            placeholder="请输入注册邮箱"
            prefix-icon="Message"
            size="large"
            class="form-input"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <div class="captcha-row">
            <el-input
              v-model="captchaValue"
              placeholder="图形验证码"
              size="large"
              class="form-input captcha-input"
              maxlength="4"
              @keyup.enter="handleSendCode"
            />
            <img
              v-if="captchaImage"
              :src="captchaImage"
              class="captcha-img"
              alt="验证码"
              title="看不清？点击刷新"
              @click="refreshCaptcha"
            />
          </div>
        </el-form-item>

        <el-form-item prop="code">
          <div class="code-row">
            <el-input
              v-model="formData.code"
              placeholder="6位邮箱验证码"
              prefix-icon="Message"
              size="large"
              class="form-input code-input"
              maxlength="6"
              @keyup.enter="handleSubmit"
            />
            <UiButton :disabled="countdown > 0 || codeSending" size="large" class="code-btn" @click="handleSendCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </UiButton>
          </div>
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="formData.password"
            type="password"
            placeholder="新密码（6-20位）"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
          />
        </el-form-item>

        <el-form-item prop="confirmPassword">
          <el-input
            v-model="formData.confirmPassword"
            type="password"
            placeholder="确认新密码"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleSubmit">
            {{ flow.loading ? '提交中...' : '重置密码' }}
          </UiButton>
        </el-form-item>
      </el-form>
    </template>

    <template #footer>
      <div class="form-footer">
        <span class="footer-text">想起密码了？</span>
        <router-link to="/login" class="footer-link">返回登录</router-link>
      </div>
    </template>
  </AuthPageShell>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message } from '@element-plus/icons-vue'
import { useSendCode } from '@/composables/useSendCode'
import { useCaptcha } from '@/composables/useCaptcha'
import { useAuthFlow } from '@/composables/useAuthFlow'
import AuthPageShell, { type AltMode, type AltModeKey } from '@/components/auth/AuthPageShell.vue'
import { passwordRules, phoneRules, requiredEmailRules, emailCodeRules, confirmPasswordRule } from '@/utils/validate'
import UiButton from '@/components/ui/UiButton.vue'

const router = useRouter()

type ResetMode = 'phone' | 'email'

// useAuthFlow：收敛「mode 状态 + validate→submit→跳转」顺序约束与 loading/错误处理
const flow = useAuthFlow<ResetMode>({
  modes: ['phone', 'email'],
  submit: async (m) => {
    if (m === 'phone') {
      return authApi.phoneResetPassword({
        phone: formData.phone.trim(),
        code: formData.code.trim(),
        password: formData.password
      })
    }
    return authApi.emailResetPassword({
      email: formData.email.trim(),
      code: formData.code.trim(),
      password: formData.password
    })
  },
  afterSuccess: async () => {
    ElMessage.success('密码已重置，请使用新密码登录')
    router.push('/login')
  }
})

// 当前激活的 alt 方式：null = 手机号找回（主方式），来自 useAuthFlow 的 mode 派生
const activeAlt = computed<AltModeKey | null>(() => (flow.mode === 'email' ? 'email' : null))

const mainFormRef = ref<FormInstance | null>(null)
const emailFormRef = ref<FormInstance | null>(null)

const { captchaId, captchaImage, captchaValue, refreshCaptcha } = useCaptcha()
onMounted(() => {
  refreshCaptcha()
})

const { sending: codeSending, remaining: countdown, send: sendCode } = useSendCode({
  purpose: 'reset_password',
  sendCode: (channel, target) =>
    channel === 'phone'
      ? authApi.sendPhoneCode({ phone: target, purpose: 'reset_password', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
      : authApi.sendEmailCode({ email: target, purpose: 'reset_password', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
})

const formData = reactive({
  phone: '',
  email: '',
  code: '',
  password: '',
  confirmPassword: ''
})

const altModes: AltMode[] = [{ key: 'email', label: '邮箱找回', icon: Message }]

function onSelectAlt(key: AltModeKey | null) {
  flow.setMode((key === 'email' ? 'email' : 'phone') as ResetMode)
}

// 确认新密码校验：复用 validate.ts 单一确认规则（Forgot 文案「请再次输入新密码」）
const confirmPasswordRules = [
  { required: true, message: '请确认新密码', trigger: 'blur' },
  confirmPasswordRule(formData, 'password', '请再次输入新密码')
]

const mainFieldRules: FormRules = {
  phone: phoneRules,
  code: emailCodeRules,
  password: passwordRules,
  confirmPassword: confirmPasswordRules
}

const emailFieldRules: FormRules = {
  email: requiredEmailRules,
  code: emailCodeRules,
  password: passwordRules,
  confirmPassword: confirmPasswordRules
}

async function handleSendCode() {
  if (captchaValue.value.trim() === '') {
    ElMessage.warning('请输入图形验证码')
    return
  }
  const ok =
    flow.mode === 'phone'
      ? await sendCode(formData.phone.trim(), 'phone')
      : await sendCode(formData.email.trim(), 'email')
  if (!ok) {
    refreshCaptcha()
  }
}

async function handleSubmit() {
  const targetRef = flow.mode === 'email' ? emailFormRef.value : mainFormRef.value
  await flow.handleSubmit(targetRef)
}
</script>
