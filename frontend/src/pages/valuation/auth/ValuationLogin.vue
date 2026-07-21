<template>
  <div class="valuation-login-page">
    <div class="login-bg">
      <div class="bg-blob bg-blob-1"></div>
      <div class="bg-blob bg-blob-2"></div>
      <div class="bg-blob bg-blob-3"></div>
    </div>

    <div class="login-card-wrap">
      <div class="login-card">
        <div class="card-header">
          <div class="card-icon">
            <el-icon :size="24"><DataAnalysis /></el-icon>
          </div>
          <h1 class="card-title">残值评估登录</h1>
          <p class="card-subtitle">登录后查看您的评估历史记录</p>
          <div class="role-badge">评估用户端</div>
        </div>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="login-form">
          <el-form-item prop="account">
            <el-input
              v-model="formData.account"
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

          <div class="form-footer">
            <span class="footer-text">还没有账号？</span>
            <router-link to="/valuation/register" class="footer-link">立即注册</router-link>
          </div>

          <div class="back-home">
            <a :href="portalUrl" class="back-link">返回官网</a>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useValuationAuthStore } from '@/stores/valuationAuth'
import { valuationAuthApi } from '@/api/valuation/auth'
import { ElMessage } from 'element-plus'
import { DataAnalysis } from '@element-plus/icons-vue'
import { passwordRules } from '@/utils/validate'
import { buildSubdomainUrl } from '@/utils/subdomain'

const router = useRouter()
const route = useRoute()
const valuationAuth = useValuationAuthStore()
const formRef = ref(null)
const loading = ref(false)

// 官网链接（主域名根路径）
const portalUrl = computed(() => buildSubdomainUrl('main', '/'))

const formData = reactive({
  account: '',
  password: ''
})

const rules = {
  account: [
    { required: true, message: '请输入用户名或手机号', trigger: 'blur' },
    { min: 3, max: 20, message: '长度在3到20个字符', trigger: 'blur' }
  ],
  password: passwordRules
}

async function handleLogin() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await valuationAuthApi.login({
      account: formData.account,
      password: formData.password
    })
    if (res.data && res.data.token) {
      valuationAuth.setAuthData(res.data)
      ElMessage.success('登录成功')

      // redirect 回跳：仅允许跳转到 /valuation 路径下，防止越权/钓鱼
      const redirect = route.query.redirect as string | undefined
      const isSafeRedirect = (target: string): boolean => {
        return target.startsWith('/valuation') && !target.startsWith('/valuation/login') && !target.startsWith('/valuation/register')
      }

      if (redirect && isSafeRedirect(redirect)) {
        router.push(redirect)
      } else {
        router.push('/valuation/history')
      }
    }
  } catch (e: any) {
    console.error('Valuation login error:', e)
    if (e.message && !e.message.includes('Network')) {
      ElMessage.error(e.message || '登录失败，请检查用户名和密码')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.valuation-login-page {
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
  background: linear-gradient(135deg, #0EA5E9 0%, #38BDF8 100%);
  box-shadow: 0 8px 20px rgba(14, 165, 233, 0.3);
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
  background: rgba(14, 165, 233, 0.08);
  color: #0284C7;
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
  background: linear-gradient(135deg, #0EA5E9 0%, #0284C7 100%);
}

.login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgba(14, 165, 233, 0.3);
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

.back-home {
  text-align: center;
  margin-top: 16px;
}

.back-link {
  font-size: 13px;
  color: #94A3B8;
  text-decoration: none;
  transition: color 0.15s ease;
}

.back-link:hover {
  color: #64748B;
}

@media screen and (max-width: 480px) {
  .valuation-login-page {
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
