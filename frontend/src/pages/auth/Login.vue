<template>
  <AuthPageShell
    title="欢迎回来"
    :subtitle="subtitleByRole"
    :alt-modes="altModes"
    :active-alt="activeAlt"
    @select-alt="onSelectAlt"
  >
    <template #main>
      <el-form
        ref="mainFormRef"
        :model="formData"
        :rules="passwordFieldRules"
        label-width="0"
        class="auth-form"
        @submit.prevent
      >
        <el-form-item prop="username">
          <UiInput v-model="formData.username" :placeholder="usernamePlaceholder" size="large" class="form-input" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="user" /></template>
          </UiInput>
        </el-form-item>

        <el-form-item prop="password">
          <UiInput v-model="formData.password" type="password" placeholder="请输入密码" show-password size="large" class="form-input" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="lock" /></template>
          </UiInput>
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleLogin">
            {{ flow.loading ? '登录中...' : '登 录' }}
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
          <UiInput v-model="formData.email" placeholder="请输入邮箱" size="large" class="form-input" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="mail" /></template>
          </UiInput>
        </el-form-item>

        <el-form-item>
          <div class="captcha-row">
            <UiInput v-model="captchaValue" placeholder="图形验证码" size="large" class="form-input captcha-input" :maxlength="4" @keyup.enter="handleSendCode"  />
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
            <UiInput v-model="formData.code" placeholder="6位邮箱验证码" size="large" class="form-input code-input" :maxlength="6" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="mail" /></template>
          </UiInput>
            <UiButton :disabled="countdown > 0 || codeSending" size="large" class="code-btn" @click="handleSendCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </UiButton>
          </div>
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleLogin">
            {{ flow.loading ? '登录中...' : '登 录' }}
          </UiButton>
        </el-form-item>
      </el-form>
    </template>

    <template #alt-phone>
      <el-form
        ref="phoneFormRef"
        :model="formData"
        :rules="phoneFieldRules"
        label-width="0"
        class="auth-form"
        @submit.prevent
      >
        <el-form-item prop="phone">
          <UiInput v-model="formData.phone" placeholder="请输入手机号" size="large" class="form-input" :maxlength="11" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="phone" /></template>
          </UiInput>
        </el-form-item>

        <el-form-item>
          <div class="captcha-row">
            <UiInput v-model="captchaValue" placeholder="图形验证码" size="large" class="form-input captcha-input" :maxlength="4" @keyup.enter="handleSendCode"  />
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
            <UiInput v-model="formData.code" placeholder="6位手机验证码" size="large" class="form-input code-input" :maxlength="6" @keyup.enter="handleLogin">
            <template #prefix><UiIcon name="mail" /></template>
          </UiInput>
            <UiButton :disabled="countdown > 0 || codeSending" size="large" class="code-btn" @click="handleSendCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </UiButton>
          </div>
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="flow.loading" class="auth-btn" size="large" @click="handleLogin">
            {{ flow.loading ? '登录中...' : '登 录' }}
          </UiButton>
        </el-form-item>
      </el-form>
    </template>

    <template #alt-wechat>
      <div class="wechat-box w-full">
        <div class="wechat-qr-placeholder flex flex-col items-center gap-2.5 border-2 border-dashed border-line-strong rounded-card bg-canvas px-4 py-7 text-ink-3">
          <UiIcon name="message-circle" :size="42" />
          <p class="wechat-title m-0 text-base font-semibold text-ink-3">扫码登录（即将开放）</p>
          <span class="wechat-tip text-center text-xs leading-normal text-ink-3">微信授权暂未配置，待开放平台配置完成后开放</span>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="form-footer" v-if="isStudentSubdomain">
        <span class="footer-text">还没有账号？</span>
        <router-link :to="href('Register')" class="footer-link">立即注册</router-link>
        <span class="footer-sep mx-2.5 text-[var(--color-text-disabled)]">·</span>
        <router-link :to="href('ForgotPassword')" class="footer-link">忘记密码？</router-link>
      </div>
    </template>
  </AuthPageShell>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { href } from '@/config/pages'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { isSafeRedirect } from '@/utils/authRedirect'
import type { UserProfile } from '@/types/user'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { usernameRules, passwordRules, requiredEmailRules, emailCodeRules, phoneRules } from '@/utils/validate'
import { useSendCode } from '@/composables/useSendCode'
import { useCaptcha } from '@/composables/useCaptcha'
import { useAuthFlow } from '@/composables/useAuthFlow'
import AuthPageShell, { type AltMode, type AltModeKey } from '@/components/auth/AuthPageShell.vue'
import {
  getSubdomain,
  getRoleForSubdomain,
  getDefaultWorkspaceBySubdomain,
  type SubdomainType
} from '@/utils/subdomain'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

type LoginMode = 'password' | 'email' | 'phone' | 'wechat'

// useAuthFlow：收敛「mode 状态 + validate→submit→跳转」顺序约束与 loading/错误处理
const flow = useAuthFlow<LoginMode>({
  modes: ['password', 'email', 'phone', 'wechat'],
  submit: async (m) => {
    // wechat 未开放：提示后 return false 阻止后续（不执行 afterSuccess）
    if (m === 'wechat') {
      ElMessage.info('微信扫码登录暂未开放，请等待开放平台配置')
      return false
    }
    const payload = {
      username: formData.username,
      password: formData.password,
      role: currentRole
    }
    if (m === 'email') {
      return authApi.emailLogin({ email: formData.email.trim(), code: formData.code.trim() })
    }
    if (m === 'phone') {
      return authApi.phoneLogin({ phone: formData.phone.trim(), code: formData.code.trim() })
    }
    if (currentRole === 'hrwai_user') return authApi.login(payload)
    if (currentRole === 'tutor') return authApi.tutorLogin(payload)
    if (currentRole === 'recruiter') return authApi.recruiterLogin(payload)
    return authApi.adminLogin(payload)
  },
  afterSuccess: async (_m, userInfo) => {
    authStore.setAuthData(userInfo as UserProfile)
    // #1651 / ADR-0075：登录身份与子域缺席角色不符 → 不落地、就地清态提示换号。
    // 不挡这一拍的话，随后的 router.push 会被守卫的回落不变式清态打回本页，形成登录循环。
    if (getSubdomain() !== 'main' && authStore.userInfo?.role !== getRoleForSubdomain()) {
      authStore.clearAuthData()
      ElMessage.error('该账号与本工作区身份不符，请使用对应身份的账号登录')
      return
    }
    ElMessage.success('登录成功')

    // redirect 回跳：复用 authRedirect.isSafeRedirect 单点白名单（仅同身份工作台内放行）
    const role = authStore.userInfo?.role
    const redirect = route.query.redirect as string | undefined
    if (redirect && isSafeRedirect(role, redirect)) {
      router.push(redirect)
    } else {
      router.push(getDefaultWorkspaceBySubdomain())
    }
  }
})

// 当前激活的 alt 方式：null = 密码登录（主方式），来自 useAuthFlow 的 mode 派生
const activeAlt = computed<AltModeKey | null>(() =>
  flow.mode === 'password' ? null : (flow.mode as AltModeKey)
)

const mainFormRef = ref<FormInstance | null>(null)
const emailFormRef = ref<FormInstance | null>(null)
const phoneFormRef = ref<FormInstance | null>(null)

const { captchaId, captchaImage, captchaValue, refreshCaptcha } = useCaptcha()
onMounted(() => {
  refreshCaptcha()
})

const { sending: codeSending, remaining: countdown, send: sendCode } = useSendCode({
  purpose: 'login',
  sendCode: (channel, target) =>
    channel === 'phone'
      ? authApi.sendPhoneCode({ phone: target, purpose: 'login', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
      : authApi.sendEmailCode({ email: target, purpose: 'login', captcha_id: captchaId.value, captcha_value: captchaValue.value.trim() })
})

// 当前子域名决定角色（不再支持手动切换）
const subdomain: SubdomainType = getSubdomain()
const currentRole = getRoleForSubdomain()
// 学员登录入口：training 子域名显示注册链接
const isStudentSubdomain = subdomain === 'training'

const subtitleMap: Record<SubdomainType, string> = {
  main: '登录您的HRWAI账户',
  training: '登录您的HRWAI账户',
  tutor: '登录讲师工作台',
  admin: '登录管理后台',
  recruit: '登录企业招聘端'
}
const subtitleByRole = computed(() => subtitleMap[subdomain])

const usernamePlaceholder = computed(() =>
  currentRole === 'tutor' || currentRole === 'admin' || currentRole === 'recruiter' ? '账号' : '账号或手机号'
)

// 非学员角色不显示 alt 方式与分隔线（tutor/admin 仅密码登录）
const altModes = computed<AltMode[]>(() =>
  currentRole === 'hrwai_user'
    ? [
        { key: 'email', label: '邮箱验证码', icon: 'message-square' },
        { key: 'phone', label: '手机验证码', icon: 'phone' },
        { key: 'wechat', label: '微信扫码', icon: 'message-circle' }
      ]
    : []
)

function onSelectAlt(key: AltModeKey | null) {
  flow.setMode((key ?? 'password') as LoginMode)
}

const formData = reactive({
  username: '',
  password: '',
  email: '',
  phone: '',
  code: ''
})

const passwordFieldRules: FormRules = { username: usernameRules, password: passwordRules }
const emailFieldRules: FormRules = { email: requiredEmailRules, code: emailCodeRules }
const phoneFieldRules: FormRules = { phone: phoneRules, code: emailCodeRules }

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

async function handleLogin() {
  const m = flow.mode
  // wechat 无独立表单：走主表单 ref（由 submit 内拦截提示「暂未开放」）
  const targetRef =
    m === 'email' ? emailFormRef.value : m === 'phone' ? phoneFormRef.value : mainFormRef.value
  await flow.handleSubmit(targetRef)
}
</script>

<style scoped>
/* Login 独有：input 内边距 6px，覆盖外壳共享样式的 4px（#569 起外壳样式真实生效） */
.auth-form .form-input :deep(.el-input__wrapper) {
  padding: 6px 14px;
}
</style>