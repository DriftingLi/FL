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
          <h1 class="card-title">欢迎登录</h1>
          <p class="card-subtitle">{{ subtitleByRole }}</p>
          <div class="role-badge" :class="`badge-${currentRole}`">
            {{ roleLabel }}
          </div>
        </div>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="login-form">
          <el-form-item prop="username">
            <el-input
              v-model="formData.username"
              placeholder="请输入手机号或用户名"
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
import { ref, reactive, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { ElMessage } from 'element-plus'
import { UserFilled, Avatar, Setting } from '@element-plus/icons-vue'
import { usernameRules, passwordRules } from '@/utils/validate'
import {
  getSubdomain,
  getRoleForSubdomain,
  getDefaultWorkspaceBySubdomain,
  buildSubdomainUrl,
  type SubdomainType
} from '@/utils/subdomain'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref(null)
const loading = ref(false)

// 当前子域名决定角色（不再支持手动切换）
const subdomain: SubdomainType = getSubdomain()
const currentRole = getRoleForSubdomain()
// 学员登录入口：training 和 valuation 子域名都显示注册链接
const isStudentSubdomain = subdomain === 'training' || subdomain === 'valuation'

const subtitleMap: Record<SubdomainType, string> = {
  main: '登录您的账户',
  training: '登录您的账户继续学习',
  valuation: '登录查看残值评估历史',
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

// 跨子域名入口引导（仅学员登录页显示）
const mentorLoginUrl = computed(() => buildSubdomainUrl('tutor', '/login'))
const adminLoginUrl = computed(() => buildSubdomainUrl('admin', '/login'))

const formData = reactive({
  username: '',
  password: ''
})

const rules = {
  username: usernameRules,
  password: passwordRules
}

async function handleLogin() {
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
    if (currentRole === 'student') {
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
        if (role === 'student') {
          // 学员可回跳到培训或残值评估路径
          return target.startsWith('/training') || target.startsWith('/valuation')
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
    console.error('Login error:', e)
    if (e.response) {
      console.error('Response data:', e.response.data)
      console.error('Status:', e.response.status)
    }
    if (e.message && !e.message.includes('Network')) {
      ElMessage.error(e.message || '登录失败，请检查用户名和密码')
    }
  } finally {
    loading.value = false
  }
}
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
  margin-bottom: 36px;
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
