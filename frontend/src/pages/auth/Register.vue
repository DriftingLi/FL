<template>
  <AuthPageShell
    title="创建账户"
    subtitle="填写以下信息完成注册"
    badge-text="学员端"
    badge-tone="student"
    :alt-modes="altModes"
    :active-alt="activeAlt"
    back-label="返回手机号注册"
    divider-text="或使用以下方式注册"
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
        <el-form-item prop="nickname">
          <el-input
            v-model="formData.nickname"
            placeholder="昵称（1-30字，展示用）"
            prefix-icon="Postcard"
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item prop="phone">
          <el-input
            v-model="formData.phone"
            placeholder="请输入手机号"
            prefix-icon="Phone"
            size="large"
            class="form-input"
            maxlength="11"
            @keyup.enter="handleRegister"
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
              @keyup.enter="handleRegister"
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
            placeholder="设置密码（6-20位，用于账号密码登录）"
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
            placeholder="确认密码"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item prop="company">
          <el-input
            v-model="formData.company"
            placeholder="您的公司（选填）"
            prefix-icon="OfficeBuilding"
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleRegister">
            {{ flow.loading ? '注册中...' : '注 册' }}
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
        <el-form-item prop="nickname">
          <el-input
            v-model="formData.nickname"
            placeholder="昵称（1-30字，展示用）"
            prefix-icon="Postcard"
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item prop="email">
          <el-input
            v-model="formData.email"
            placeholder="请输入邮箱"
            prefix-icon="Message"
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
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
              @keyup.enter="handleRegister"
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
            placeholder="设置密码（6-20位，用于账号密码登录）"
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
            placeholder="确认密码"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item prop="company">
          <el-input
            v-model="formData.company"
            placeholder="您的公司（选填）"
            prefix-icon="OfficeBuilding"
            size="large"
            class="form-input"
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleRegister">
            {{ flow.loading ? '注册中...' : '注 册' }}
          </UiButton>
        </el-form-item>
      </el-form>
    </template>

    <template #footer>
      <div class="form-footer">
        <span class="footer-text">已有账号？</span>
        <router-link to="/login" class="footer-link">返回登录</router-link>
      </div>
    </template>
  </AuthPageShell>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import type { UserProfile } from '@/types/user'
import { getDefaultWorkspaceBySubdomain } from '@/utils/subdomain'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message } from '@element-plus/icons-vue'
import { useSendCode } from '@/composables/useSendCode'
import { useCaptcha } from '@/composables/useCaptcha'
import { useAuthFlow } from '@/composables/useAuthFlow'
import AuthPageShell, { type AltMode, type AltModeKey } from '@/components/auth/AuthPageShell.vue'
import {
  passwordRules,
  nicknameRules,
  phoneRules,
  companyRules,
  requiredEmailRules,
  emailCodeRules,
  confirmPasswordRule
} from '@/utils/validate'
import UiButton from '@/components/ui/UiButton.vue'

const router = useRouter()
const authStore = useAuthStore()

type RegisterMode = 'phone' | 'email'

// useAuthFlow：收敛「mode 状态 + validate→submit→跳转」顺序约束与 loading/错误处理
const flow = useAuthFlow<RegisterMode>({
  modes: ['phone', 'email'],
  submit: async (m) => {
    if (m === 'email') {
      return authApi.emailRegister({
        nickname: formData.nickname,
        email: formData.email.trim(),
        code: formData.code.trim(),
        company: formData.company,
        password: formData.password
      })
    }
    return authApi.phoneRegister({
      nickname: formData.nickname,
      phone: formData.phone.trim(),
      code: formData.code.trim(),
      company: formData.company,
      password: formData.password
    })
  },
  afterSuccess: async (_m, info) => {
    authStore.setAuthData(info as UserProfile)
    ElMessage.success('注册成功')
    router.push(getDefaultWorkspaceBySubdomain())
  }
})

// 当前激活的 alt 方式：null = 手机号注册（主方式），来自 useAuthFlow 的 mode 派生
const activeAlt = computed<AltModeKey | null>(() => (flow.mode === 'email' ? 'email' : null))

const mainFormRef = ref<FormInstance | null>(null)
const emailFormRef = ref<FormInstance | null>(null)

const { captchaId, captchaImage, captchaValue, refreshCaptcha } = useCaptcha()
onMounted(() => {
  refreshCaptcha()
})

const { sending: codeSending, remaining: countdown, send: sendCode } = useSendCode({
  purpose: 'register',
  sendCode: (channel, target) =>
    channel === 'phone'
      ? authApi.sendPhoneCode({ phone: target, purpose: 'register', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
      : authApi.sendEmailCode({ email: target, purpose: 'register', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
})

const formData = reactive({
  nickname: '',
  phone: '',
  password: '',
  confirmPassword: '',
  company: '',
  email: '',
  code: ''
})

const altModes: AltMode[] = [{ key: 'email', label: '邮箱注册', icon: Message }]

function onSelectAlt(key: AltModeKey | null) {
  flow.setMode((key === 'email' ? 'email' : 'phone') as RegisterMode)
}

// 确认密码校验：复用 validate.ts 单一确认规则（Register 文案「请再次输入密码」）
const confirmPasswordRules = [
  { required: true, message: '请确认密码', trigger: 'blur' },
  confirmPasswordRule(formData, 'password', '请再次输入密码')
]

const mainFieldRules: FormRules = {
  nickname: nicknameRules,
  phone: phoneRules,
  code: emailCodeRules,
  password: passwordRules,
  confirmPassword: confirmPasswordRules,
  company: companyRules
}

const emailFieldRules: FormRules = {
  nickname: nicknameRules,
  email: requiredEmailRules,
  code: emailCodeRules,
  password: passwordRules,
  confirmPassword: confirmPasswordRules,
  company: companyRules
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

async function handleRegister() {
  const targetRef = flow.mode === 'email' ? emailFormRef.value : mainFormRef.value
  await flow.handleSubmit(targetRef)
}
</script>

<style scoped>
/* 表单共享样式已在 AuthPageShell 外壳收敛（:deep 触达 slot） */
</style>