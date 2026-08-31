<template>
  <AuthPageShell
    title="欢迎回来"
    :subtitle="subtitleByRole"
    :badge-text="roleLabel"
    :badge-tone="badgeTone"
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
          <el-input
            v-model="formData.username"
            :placeholder="usernamePlaceholder"
            prefix-icon="User"
            size="large"
            class="form-input"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="formData.password"
            type="password"
            placeholder="请输入密码"
            prefix-icon="Lock"
            show-password
            size="large"
            class="form-input"
            @keyup.enter="handleLogin"
          />
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
          <el-input
            v-model="formData.email"
            placeholder="请输入邮箱"
            prefix-icon="Message"
            size="large"
            class="form-input"
            @keyup.enter="handleLogin"
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
              @keyup.enter="handleLogin"
            />
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
          <el-input
            v-model="formData.phone"
            placeholder="请输入手机号"
            prefix-icon="Phone"
            size="large"
            class="form-input"
            maxlength="11"
            @keyup.enter="handleLogin"
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
              @keyup.enter="handleLogin"
            />
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
      <div class="wechat-box">
        <div class="wechat-qr-placeholder">
          <el-icon :size="42"><ChatDotRound /></el-icon>
          <p class="wechat-title">扫码登录（即将开放）</p>
          <span class="wechat-tip">微信授权暂未配置，待开放平台配置完成后开放</span>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="form-footer" v-if="isStudentSubdomain">
        <span class="footer-text">还没有账号？</span>
        <router-link to="/register" class="footer-link">立即注册</router-link>
        <span class="footer-sep">·</span>
        <router-link to="/forgot-password" class="footer-link">忘记密码？</router-link>
      </div>
    </template>
  </AuthPageShell>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { isSafeRedirect } from '@/utils/authRedirect'
import type { UserProfile } from '@/types/user'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Phone, ChatDotRound } from '@element-plus/icons-vue'
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
// 学员登录入口：training 和 valuation 子域名都显示注册链接
const isStudentSubdomain = subdomain === 'training' || subdomain === 'valuation'

const subtitleMap: Record<SubdomainType, string> = {
  main: '登录您的HRWAI账户',
  training: '登录您的HRWAI账户',
  valuation: '登录您的HRWAI账户',
  tutor: '登录导师工作台',
  admin: '登录管理后台',
  recruit: '登录企业招聘端'
}
const subtitleByRole = computed(() => subtitleMap[subdomain])

// badge 角色色：学员=蓝/导师=青/管理=紫/企业=蓝（复用学员色，避免新增视觉分支）
const badgeTone = computed<'student' | 'tutor' | 'admin'>(
  () => (currentRole === 'tutor' ? 'tutor' : currentRole === 'admin' ? 'admin' : 'student')
)
const roleLabel = computed(() =>
  currentRole === 'tutor' ? '导师端' : currentRole === 'admin' ? '管理端' : currentRole === 'recruiter' ? '企业端' : '学员端'
)

const usernamePlaceholder = computed(() =>
  currentRole === 'tutor' || currentRole === 'admin' || currentRole === 'recruiter' ? '账号' : '账号或手机号'
)

// 非学员角色不显示 alt 方式与分隔线（tutor/admin 仅密码登录）
const altModes = computed<AltMode[]>(() =>
  currentRole === 'hrwai_user'
    ? [
        { key: 'email', label: '邮箱验证码', icon: Message },
        { key: 'phone', label: '手机验证码', icon: Phone },
        { key: 'wechat', label: '微信扫码', icon: ChatDotRound }
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
/* 表单共享样式已在 AuthPageShell 外壳收敛（:deep 触达 slot） */
.wechat-box {
  width: 100%;
}

.wechat-qr-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 28px 16px;
  border: 2px dashed #cbd5e1;
  border-radius: 12px;
  color: #94a3b8;
  background: #f8fafc;
}

.wechat-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #64748b;
}

.wechat-tip {
  font-size: 12px;
  color: #94a3b8;
  text-align: center;
  line-height: 1.5;
}

/* Login 独有：input 内边距 6px（其余认证页为 4px，由外壳统一），此处覆盖保持像素级不变 */
.auth-form .form-input :deep(.el-input__wrapper) {
  padding: 6px 14px;
}

.footer-sep {
  margin: 0 10px;
  color: #cbd5e1;
}
</style>