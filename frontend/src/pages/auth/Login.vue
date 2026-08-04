<template>
  <div class="login-page">
    <div class="login-bg">
      <div class="bg-blob bg-blob-1"></div>
      <div class="bg-blob bg-blob-2"></div>
      <div class="bg-blob bg-blob-3"></div>
    </div>

    <div class="login-card-wrap">
      <div class="login-card" :class="`card-${currentRole}`">
        <div class="card-header">
          <div class="card-icon" :class="`icon-${currentRole}`">
            <el-icon :size="24">
              <component :is="roleIcon" />
            </el-icon>
          </div>
          <h1 class="card-title">欢迎回来</h1>
          <p class="card-subtitle">{{ subtitleByRole }}</p>
          <div class="role-badge" :class="`badge-${currentRole}`">
            {{ roleLabel }}
          </div>
        </div>

        <el-radio-group
          v-if="currentRole === 'hrwai_user'"
          v-model="loginMode"
          class="mode-switch"
        >
          <el-radio-button label="password">账号密码登录</el-radio-button>
          <el-radio-button label="email">邮箱登录</el-radio-button>
          <el-radio-button label="phone">手机号登录</el-radio-button>
          <el-radio-button label="wechat">微信扫码</el-radio-button>
        </el-radio-group>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="login-form">
          <template v-if="loginMode === 'password'">
            <el-form-item prop="username">
              <el-input
                v-model="formData.username"
                placeholder="请输入您的账号"
                prefix-icon="User"
                size="large"
                class="form-input"
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
          </template>

          <template v-else-if="loginMode === 'email'">
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
                <el-button
                  :disabled="countdown > 0 || codeSending"
                  size="large"
                  class="code-btn"
                  @click="handleSendCode"
                >
                  {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
                </el-button>
              </div>
            </el-form-item>
          </template>

          <template v-else-if="loginMode === 'phone'">
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
                <el-button
                  :disabled="countdown > 0 || codeSending"
                  size="large"
                  class="code-btn"
                  @click="handleSendCode"
                >
                  {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
                </el-button>
              </div>
            </el-form-item>
          </template>

          <template v-else>
            <el-form-item>
              <div class="wechat-box">
                <div class="wechat-qr-placeholder">
                  <el-icon :size="42"><ChatDotRound /></el-icon>
                  <p class="wechat-title">微信扫码登录</p>
                  <span class="wechat-tip">微信授权暂未配置，待开放平台配置完成后开放</span>
                </div>
              </div>
            </el-form-item>
          </template>

          <el-form-item>
            <el-button
              type="primary"
              :loading="loading"
              class="login-btn"
              size="large"
              @click="handleLogin"
            >
              {{ loading ? '登录中...' : '登 录' }}
            </el-button>
          </el-form-item>

          <div class="form-footer" v-if="isStudentSubdomain">
            <span class="footer-text">还没有账号？</span>
            <router-link to="/register" class="footer-link">立即注册</router-link>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { ElMessage, type FormInstance } from 'element-plus'
import { UserFilled, Avatar, Setting, ChatDotRound } from '@element-plus/icons-vue'
import { usernameRules, passwordRules, requiredEmailRules, emailCodeRules, phoneRules } from '@/utils/validate'
import {
  getSubdomain,
  getRoleForSubdomain,
  getDefaultWorkspaceBySubdomain,
  type SubdomainType
} from '@/utils/subdomain'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const loginMode = ref<'password' | 'email' | 'phone' | 'wechat'>('password')
const countdown = ref(0)
const codeSending = ref(false)
let countdownTimer: number | undefined

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
  admin: '登录管理后台'
}
const subtitleByRole = computed(() => subtitleMap[subdomain])

const roleLabelMap: Record<string, string> = {
  student: '学员端',
  tutor: '导师端',
  admin: '管理端'
}
const roleIconMap: Record<string, any> = {
  student: UserFilled,
  tutor: Avatar,
  admin: Setting
}
const roleLabel = computed(() => roleLabelMap[currentRole])
const roleIcon = computed(() => roleIconMap[currentRole])

const formData = reactive({
  username: '',
  password: '',
  email: '',
  phone: '',
  code: ''
})

const rules = computed(() => {
  if (loginMode.value === 'email') return { email: requiredEmailRules, code: emailCodeRules }
  if (loginMode.value === 'phone') return { phone: phoneRules, code: emailCodeRules }
  if (loginMode.value === 'wechat') return {}
  return { username: usernameRules, password: passwordRules }
})

async function handleSendCode() {
  codeSending.value = true
  try {
    let res
    if (loginMode.value === 'phone') {
      const phone = formData.phone.trim()
      if (!/^1[3-9]\d{9}$/.test(phone)) {
        ElMessage.warning('请输入正确的手机号')
        return
      }
      res = await authApi.sendPhoneCode({ phone, purpose: 'login' })
    } else {
      const email = formData.email.trim()
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        ElMessage.warning('请输入正确的邮箱地址')
        return
      }
      res = await authApi.sendEmailCode({ email, purpose: 'login' })
    }
    if (res.code === 200) {
      ElMessage.success('验证码已发送，请查收')
      countdown.value = 60
      if (countdownTimer) window.clearInterval(countdownTimer)
      countdownTimer = window.setInterval(() => {
        countdown.value--
        if (countdown.value <= 0 && countdownTimer) {
          window.clearInterval(countdownTimer)
          countdownTimer = undefined
        }
      }, 1000)
    }
  } catch (e) {
    // 拦截器已提示
  } finally {
    codeSending.value = false
  }
}

async function handleLogin() {
  if (loginMode.value === 'wechat') {
    ElMessage.info('微信扫码登录暂未开放，请等待开放平台配置')
    return
  }
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const payload = {
      username: formData.username,
      password: formData.password,
      role: currentRole
    }
    let res
    if (loginMode.value === 'email') {
      res = await authApi.emailLogin({
        email: formData.email.trim(),
        code: formData.code.trim()
      })
    } else if (loginMode.value === 'phone') {
      res = await authApi.phoneLogin({
        phone: formData.phone.trim(),
        code: formData.code.trim()
      })
    } else if (currentRole === 'hrwai_user') {
      res = await authApi.login(payload)
    } else if (currentRole === 'tutor') {
      res = await authApi.tutorLogin(payload)
    } else {
      res = await authApi.adminLogin(payload)
    }

    if (res.code === 200 || res.code === 201) {
      authStore.setAuthData(res.data)
      ElMessage.success('登录成功')

      // 默认跳转到当前子域名对应的工作区
      const dashboard = getDefaultWorkspaceBySubdomain()

      // redirect 回跳：仅允许在同身份工作台内回跳，防止越权/钓鱼
      const role = authStore.userInfo?.role
      const redirect = route.query.redirect as string | undefined
      const isSafeRedirect = (target: string): boolean => {
        if (role === 'admin') return target.startsWith('/admin')
        if (role === 'tutor') return target.startsWith('/training/tutor')
        if (role === 'hrwai_user') {
          // 学员可回跳到培训、残值评估或 AI 助手路径
          return target.startsWith('/training') || target.startsWith('/valuation') || target.startsWith('/ai-assistant')
        }
        return false
      }

      if (redirect && isSafeRedirect(redirect)) {
        router.push(redirect)
      } else {
        router.push(dashboard)
      }
    }
  } catch (e) {
    const err = e as { response?: { data?: unknown; status?: number }; message?: string }
    console.error('Login error:', err)
    if (err.response) {
      console.error('Response data:', err.response.data)
      console.error('Status:', err.response.status)
    }
    if (err.message && !err.message.includes('Network')) {
      ElMessage.error(err.message || '登录失败，请检查用户名和密码')
    }
  } finally {
    loading.value = false
  }
}

onUnmounted(() => {
  if (countdownTimer) {
    window.clearInterval(countdownTimer)
    countdownTimer = undefined
  }
})
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #F1F5F9;
  position: relative;
  overflow: hidden;
  padding: 24px;
}

.login-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.bg-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.35;
}

.bg-blob-1 {
  width: 480px;
  height: 480px;
  background: #BAE6FD;
  top: -120px;
  right: -80px;
  animation: blob-float 12s ease-in-out infinite;
}

.bg-blob-2 {
  width: 380px;
  height: 380px;
  background: #99F6E4;
  bottom: -100px;
  left: -60px;
  animation: blob-float 14s ease-in-out infinite 3s;
}

.bg-blob-3 {
  width: 280px;
  height: 280px;
  background: #7DD3FC;
  top: 40%;
  left: 35%;
  opacity: 0.15;
  animation: blob-float 10s ease-in-out infinite 1.5s;
}

@keyframes blob-float {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(20px, -20px) scale(1.05); }
  66% { transform: translate(-15px, 15px) scale(0.95); }
}

.login-card-wrap {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 420px;
}

.login-card {
  background: #FFFFFF;
  border-radius: 20px;
  padding: 48px 40px 40px;
  box-shadow:
    0 4px 6px -1px rgba(15, 23, 42, 0.05),
    0 20px 50px -12px rgba(15, 23, 42, 0.1);
  border: 1px solid rgba(226, 232, 240, 0.6);
}

.card-header {
  text-align: center;
  margin-bottom: 20px;
}

.card-icon {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
  color: white;
}

.icon-student {
  background: linear-gradient(135deg, #0EA5E9 0%, #38BDF8 100%);
  box-shadow: 0 8px 20px rgba(14, 165, 233, 0.3);
}

.icon-tutor {
  background: linear-gradient(135deg, #14B8A6 0%, #2DD4BF 100%);
  box-shadow: 0 8px 20px rgba(20, 184, 166, 0.3);
}

.icon-admin {
  background: linear-gradient(135deg, #6366F1 0%, #818CF8 100%);
  box-shadow: 0 8px 20px rgba(99, 102, 241, 0.3);
}

.card-title {
  font-size: 26px;
  font-weight: 700;
  color: #0F172A;
  margin: 0 0 8px;
  letter-spacing: -0.02em;
}

.card-subtitle {
  font-size: 14px;
  color: #64748B;
  margin: 0 0 14px;
  line-height: 1.5;
}

.role-badge {
  display: inline-block;
  padding: 4px 14px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.02em;
}

.badge-student {
  background: rgba(14, 165, 233, 0.08);
  color: #0284C7;
}

.badge-tutor {
  background: rgba(20, 184, 166, 0.08);
  color: #0D9488;
}

.badge-admin {
  background: rgba(99, 102, 241, 0.08);
  color: #4F46E5;
}

.login-form {
  margin-top: 8px;
}

.mode-switch {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  width: 100%;
  margin-bottom: 16px;
}

.mode-switch :deep(.el-radio-button) {
  display: flex;
}

.mode-switch :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 44px;
  width: 100%;
  border: 1px solid #E2E8F0;
  border-radius: 10px;
  background: #FFFFFF;
  color: #64748B;
  font-size: 14px;
  font-weight: 600;
  padding: 0;
  box-shadow: none;
  transition: all 0.25s ease;
}

.mode-switch :deep(.el-radio-button:not(.is-active):hover .el-radio-button__inner) {
  border-color: #93C5FD;
  color: #0284C7;
  background: #F0F9FF;
}

.mode-switch :deep(.el-radio-button.is-active .el-radio-button__inner) {
  background: linear-gradient(135deg, #0EA5E9 0%, #0284C7 100%);
  border-color: transparent;
  color: #FFFFFF;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.3);
}

.code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.code-input {
  flex: 1;
}

.code-btn {
  min-width: 124px;
}

.wechat-box {
  width: 100%;
}

.wechat-qr-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 28px 16px;
  border: 2px dashed #CBD5E1;
  border-radius: 12px;
  color: #94A3B8;
  background: #F8FAFC;
}

.wechat-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #64748B;
}

.wechat-tip {
  font-size: 12px;
  color: #94A3B8;
  text-align: center;
  line-height: 1.5;
}

.form-input :deep(.el-input__wrapper) {
  border-radius: 12px;
  padding: 6px 14px;
  box-shadow: 0 0 0 1px #E2E8F0 inset;
  transition: all 0.2s ease;
  background: #F8FAFC;
}

.form-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #CBD5E1 inset;
  background: #FFFFFF;
}

.form-input :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.25) inset;
  background: #FFFFFF;
}

.form-input :deep(.el-input__prefix-inner) {
  color: #94A3B8;
}

.login-btn {
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
  border: none;
  letter-spacing: 0.08em;
  margin-top: 8px;
  transition: all 0.2s ease;
}

.card-student .login-btn {
  background: linear-gradient(135deg, #0EA5E9 0%, #0284C7 100%);
}

.card-tutor .login-btn {
  background: linear-gradient(135deg, #14B8A6 0%, #0D9488 100%);
}

.card-admin .login-btn {
  background: linear-gradient(135deg, #6366F1 0%, #4F46E5 100%);
}

.card-student .login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgba(14, 165, 233, 0.3);
  opacity: 0.95;
}

.card-tutor .login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgba(20, 184, 166, 0.3);
  opacity: 0.95;
}

.card-admin .login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgba(99, 102, 241, 0.3);
  opacity: 0.95;
}

.form-footer {
  text-align: center;
  margin-top: 24px;
}

.footer-text {
  font-size: 14px;
  color: #94A3B8;
}

.footer-link {
  font-size: 14px;
  font-weight: 600;
  color: #0EA5E9;
  text-decoration: none;
  margin-left: 4px;
  transition: color 0.15s ease;
}

.footer-link:hover {
  color: #0284C7;
}

@media screen and (max-width: 480px) {
  .login-page {
    padding: 16px;
  }

  .login-card {
    padding: 36px 24px 28px;
    border-radius: 16px;
  }

  .card-title {
    font-size: 22px;
  }

  .card-icon {
    width: 48px;
    height: 48px;
    border-radius: 14px;
  }
}
</style>
