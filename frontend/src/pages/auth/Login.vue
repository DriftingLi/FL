<template>
  <div class="login-page">
    <div class="login-brand">
      <div class="brand-content">
        <div class="brand-logo">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
        </div>
        <h1 class="brand-title">和润天下</h1>
        <p class="brand-subtitle">专业叉车维修培训系统</p>
        <div class="brand-features">
          <div class="feature-item">
            <div class="feature-dot"></div>
            <span>3D 虚拟实操训练</span>
          </div>
          <div class="feature-item">
            <div class="feature-dot"></div>
            <span>AI 智能辅助教学</span>
          </div>
          <div class="feature-item">
            <div class="feature-dot"></div>
            <span>全流程考核认证</span>
          </div>
        </div>
      </div>
      <div class="brand-decor">
        <div class="decor-circle decor-circle-1"></div>
        <div class="decor-circle decor-circle-2"></div>
        <div class="decor-circle decor-circle-3"></div>
        <div class="decor-shape decor-shape-1"></div>
        <div class="decor-shape decor-shape-2"></div>
      </div>
    </div>

    <div class="login-form-side">
      <div class="form-container">
        <div class="form-header">
          <h2 class="form-title">欢迎回来</h2>
          <p class="form-subtitle">{{ subtitleByRole }}</p>
        </div>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="login-form">
          <el-form-item prop="username">
            <el-input
              v-model="formData.username"
              placeholder="请输入手机号或用户名"
              prefix-icon="User"
              size="large"
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

          <div class="subdomain-guide" v-if="isStudentSubdomain">
            <div class="guide-item">
              <span class="guide-label">导师入口：</span>
              <a :href="mentorLoginUrl" class="guide-link">{{ mentorLoginUrl }}</a>
            </div>
            <div class="guide-item">
              <span class="guide-label">管理员入口：</span>
              <a :href="adminLoginUrl" class="guide-link">{{ adminLoginUrl }}</a>
            </div>
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
import { usernameRules, passwordRules } from '@/utils/validate'
import {
  getSubdomain,
  isMainSubdomain,
  buildSubdomainUrl,
  type SubdomainType
} from '@/utils/subdomain'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref(null)
const loading = ref(false)

// 当前子域名决定的角色（学员/导师/管理员），不再支持手动切换
const subdomain: SubdomainType = getSubdomain()
const isStudentSubdomain = isMainSubdomain()
const currentRole = subdomain === 'tutor' ? 'tutor' : subdomain === 'admin' ? 'admin' : 'student'

const subtitleMap: Record<SubdomainType, string> = {
  student: '登录您的账户继续学习',
  tutor: '登录导师工作区',
  admin: '登录管理后台'
}
const subtitleByRole = computed(() => subtitleMap[subdomain])

// 跨子域名入口（仅主域名登录页显示引导）
const mentorLoginUrl = computed(() => buildSubdomainUrl('mentor', '/login'))
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

      // 子域名已与角色绑定，不再需要按角色跨域名跳转
      const dashboardByRole: Record<string, string> = {
        admin: '/admin/dashboard',
        tutor: '/training/tutor',
        student: '/training'
      }
      const role = authStore.userInfo?.role
      const dashboard = (role && dashboardByRole[role]) || '/'

      // redirect 回跳：仅允许在同身份工作台内回跳，防止越权/钓鱼
      const redirect = route.query.redirect as string | undefined
      const isSafeRedirect = (target: string): boolean => {
        if (role === 'admin') return target.startsWith('/admin')
        if (role === 'tutor') return target.startsWith('/training/tutor')
        if (role === 'student') return target.startsWith('/training')
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
}

.login-brand {
  flex: 0 0 55%;
  background: var(--gradient-hero);
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  padding: var(--space-10);
}

.brand-content {
  position: relative;
  z-index: 2;
  max-width: 480px;
}

.brand-logo {
  margin-bottom: var(--space-6);
}

.logo-img {
  width: 72px;
  height: 72px;
  border-radius: var(--radius-xl);
  object-fit: cover;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.brand-title {
  font-family: var(--font-display);
  font-size: var(--text-5xl);
  font-weight: var(--font-bold);
  color: white;
  margin-bottom: var(--space-3);
  letter-spacing: -0.03em;
  line-height: 1.1;
}

.brand-subtitle {
  font-size: var(--text-xl);
  color: rgba(255, 255, 255, 0.7);
  margin-bottom: var(--space-10);
  line-height: var(--leading-relaxed);
}

.brand-features {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.feature-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  color: rgba(255, 255, 255, 0.8);
  font-size: var(--text-base);
}

.feature-dot {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-full);
  background: var(--color-accent-400);
  flex-shrink: 0;
}

.brand-decor {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.decor-circle {
  position: absolute;
  border-radius: var(--radius-full);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.decor-circle-1 {
  width: 400px;
  height: 400px;
  right: -100px;
  top: -80px;
  animation: float 8s ease-in-out infinite;
}

.decor-circle-2 {
  width: 250px;
  height: 250px;
  left: -60px;
  bottom: -40px;
  animation: float 10s ease-in-out infinite 2s;
}

.decor-circle-3 {
  width: 150px;
  height: 150px;
  right: 20%;
  bottom: 15%;
  background: rgba(255, 255, 255, 0.03);
  animation: float 6s ease-in-out infinite 1s;
}

.decor-shape {
  position: absolute;
  background: rgba(255, 255, 255, 0.05);
}

.decor-shape-1 {
  width: 60px;
  height: 60px;
  right: 15%;
  top: 20%;
  border-radius: var(--radius-lg);
  transform: rotate(45deg);
  animation: float 7s ease-in-out infinite 3s;
}

.decor-shape-2 {
  width: 40px;
  height: 40px;
  left: 20%;
  top: 30%;
  border-radius: var(--radius-md);
  transform: rotate(30deg);
  animation: float 9s ease-in-out infinite 1.5s;
}

.login-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-10);
  background: var(--color-bg-card);
}

.form-container {
  width: 100%;
  max-width: 400px;
}

.form-header {
  margin-bottom: var(--space-8);
}

.form-title {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
}

.form-subtitle {
  font-size: var(--text-base);
  color: var(--color-text-tertiary);
}

.subdomain-guide {
  margin-top: var(--space-5);
  padding: var(--space-3) var(--space-4);
  background: var(--color-bg-secondary, #f7f8fa);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.guide-item {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.guide-label {
  flex-shrink: 0;
}

.guide-link {
  color: var(--color-primary-500);
  text-decoration: none;
  word-break: break-all;
  transition: color var(--duration-fast);
}

.guide-link:hover {
  color: var(--color-primary-600);
}

.login-form :deep(.el-input__wrapper) {
  padding: 4px 12px;
  border-radius: var(--radius-lg);
}

.login-btn {
  width: 100%;
  height: 48px;
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  border-radius: var(--radius-lg);
  background: var(--gradient-brand);
  border: none;
  letter-spacing: 0.05em;
  transition: all var(--duration-fast) var(--ease-default);
}

.login-btn:hover {
  box-shadow: var(--shadow-glow);
  transform: translateY(-1px);
}

.form-footer {
  text-align: center;
  margin-top: var(--space-5);
}

.footer-text {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
}

.footer-link {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--color-primary-500);
  text-decoration: none;
  transition: color var(--duration-fast);
}

.footer-link:hover {
  color: var(--color-primary-600);
}

@media screen and (max-width: 768px) {
  .login-page {
    flex-direction: column;
  }

  .login-brand {
    flex: none;
    padding: var(--space-10) var(--space-6);
    min-height: 240px;
  }

  .brand-title {
    font-size: var(--text-3xl);
  }

  .brand-subtitle {
    font-size: var(--text-base);
    margin-bottom: var(--space-6);
  }

  .brand-features {
    display: none;
  }

  .login-form-side {
    padding: var(--space-6);
  }

  .form-title {
    font-size: var(--text-2xl);
  }
}

@media screen and (max-width: 480px) {
  .login-brand {
    padding: var(--space-8) var(--space-4);
    min-height: 200px;
  }

  .brand-title {
    font-size: var(--text-2xl);
  }

  .login-form-side {
    padding: var(--space-5) var(--space-4);
  }
}
</style>
