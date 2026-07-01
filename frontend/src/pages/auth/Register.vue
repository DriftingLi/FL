<template>
  <div class="register-page">
    <div class="register-brand">
      <div class="brand-content">
        <div class="brand-logo">
          <svg width="56" height="56" viewBox="0 0 56 56" fill="none">
            <rect width="56" height="56" rx="16" fill="rgba(255,255,255,0.1)"/>
            <path d="M16 36L28 16L40 36H16Z" stroke="white" stroke-width="2" stroke-linejoin="round" fill="none"/>
            <circle cx="28" cy="30" r="3" fill="white"/>
          </svg>
        </div>
        <h1 class="brand-title">ForkLift<span class="brand-accent">Pro</span></h1>
        <p class="brand-subtitle">叉车维修一站式服务平台</p>
      </div>
      <div class="brand-decor">
        <div class="decor-circle decor-circle-1"></div>
        <div class="decor-circle decor-circle-2"></div>
      </div>
    </div>

    <div class="register-form-side">
      <div class="form-container">
        <div class="form-header">
          <h2 class="form-title">创建账户</h2>
          <p class="form-subtitle">填写以下信息完成注册</p>
        </div>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="register-form">
          <el-form-item prop="name">
            <el-input
              v-model="formData.name"
              placeholder="真实姓名"
              prefix-icon="Postcard"
              size="large"
            />
          </el-form-item>

          <el-form-item prop="phone">
            <el-input
              v-model="formData.phone"
              placeholder="手机号"
              prefix-icon="Phone"
              size="large"
              maxlength="11"
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="formData.password"
              type="password"
              placeholder="密码（6-20位字符）"
              prefix-icon="Lock"
              show-password
              size="large"
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
            />
          </el-form-item>

          <el-form-item prop="company">
            <el-input
              v-model="formData.company"
              placeholder="单位（选填）"
              prefix-icon="OfficeBuilding"
              size="large"
            />
          </el-form-item>

          <el-form-item prop="email">
            <el-input
              v-model="formData.email"
              placeholder="邮箱（选填）"
              prefix-icon="Message"
              size="large"
              @keyup.enter="handleRegister"
            />
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              :loading="loading"
              class="register-btn"
              size="large"
              @click="handleRegister"
            >
              {{ loading ? '注册中...' : '注 册' }}
            </el-button>
          </el-form-item>

          <div class="form-footer">
            <span class="footer-text">已有账号？</span>
            <router-link to="/login" class="footer-link">返回登录</router-link>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import { ElMessage } from 'element-plus'
import { passwordRules, nameRules, phoneRules, emailRules, companyRules } from '@/utils/validate'

const router = useRouter()
const formRef = ref(null)
const loading = ref(false)

const formData = reactive({
  name: '',
  phone: '',
  password: '',
  confirmPassword: '',
  company: '',
  email: ''
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== formData.password) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  name: nameRules,
  phone: phoneRules,
  password: passwordRules,
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ],
  company: companyRules,
  email: emailRules
}

async function handleRegister() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await authApi.register({
      name: formData.name,
      phone: formData.phone,
      password: formData.password,
      company: formData.company,
      email: formData.email
    })

    if (res.code === 201 || res.code === 200) {
      ElMessage.success('注册成功，即将跳转到登录页...')
      setTimeout(() => {
        router.push('/login')
      }, 1500)
    }
  } catch (e) {
    console.error('Register error:', e)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.register-page {
  min-height: 100vh;
  display: flex;
}

.register-brand {
  flex: 0 0 45%;
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
  max-width: 400px;
}

.brand-logo {
  margin-bottom: var(--space-6);
}

.brand-title {
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: var(--font-bold);
  color: white;
  margin-bottom: var(--space-3);
  letter-spacing: -0.03em;
}

.brand-accent {
  color: var(--color-primary-300);
}

.brand-subtitle {
  font-size: var(--text-lg);
  color: rgba(255, 255, 255, 0.7);
  line-height: var(--leading-relaxed);
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
  width: 300px;
  height: 300px;
  right: -80px;
  bottom: -60px;
  animation: float 8s ease-in-out infinite;
}

.decor-circle-2 {
  width: 180px;
  height: 180px;
  left: -40px;
  top: -30px;
  animation: float 10s ease-in-out infinite 2s;
}

.register-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-10);
  background: var(--color-bg-card);
}

.form-container {
  width: 100%;
  max-width: 420px;
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

.register-form :deep(.el-input__wrapper) {
  padding: 4px 12px;
  border-radius: var(--radius-lg);
}

.register-btn {
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

.register-btn:hover {
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
  .register-page {
    flex-direction: column;
  }

  .register-brand {
    flex: none;
    padding: var(--space-8) var(--space-6);
    min-height: 180px;
  }

  .brand-title {
    font-size: var(--text-2xl);
  }

  .register-form-side {
    padding: var(--space-6);
  }

  .form-title {
    font-size: var(--text-2xl);
  }
}

@media screen and (max-width: 480px) {
  .register-brand {
    padding: var(--space-6) var(--space-4);
    min-height: 150px;
  }

  .register-form-side {
    padding: var(--space-5) var(--space-4);
  }
}
</style>
