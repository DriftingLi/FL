<template>
  <div class="register-page">
    <div class="register-bg">
      <div class="bg-blob bg-blob-1"></div>
      <div class="bg-blob bg-blob-2"></div>
      <div class="bg-blob bg-blob-3"></div>
    </div>

    <div class="register-card-wrap">
      <div class="register-card">
        <div class="card-header">
          <div class="card-icon">
            <el-icon :size="24">
              <EditPen />
            </el-icon>
          </div>
          <h1 class="card-title">创建账户</h1>
          <p class="card-subtitle">填写以下信息完成注册</p>
        </div>

        <el-radio-group v-model="registerMode" class="mode-switch">
          <el-radio-button label="phone">手机号注册</el-radio-button>
          <el-radio-button label="email">邮箱注册</el-radio-button>
        </el-radio-group>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="register-form">
          <el-form-item prop="name">
            <el-input
              v-model="formData.name"
              placeholder="真实姓名"
              prefix-icon="Postcard"
              size="large"
              class="form-input"
            />
          </el-form-item>

          <template v-if="registerMode === 'phone'">
            <el-form-item prop="phone">
              <el-input
                v-model="formData.phone"
                placeholder="手机号"
                prefix-icon="Phone"
                size="large"
                class="form-input"
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
              />
            </el-form-item>

            <el-form-item prop="company">
              <el-input
                v-model="formData.company"
                placeholder="单位（选填）"
                prefix-icon="OfficeBuilding"
                size="large"
                class="form-input"
              />
            </el-form-item>

            <el-form-item prop="email">
              <el-input
                v-model="formData.email"
                placeholder="邮箱（选填）"
                prefix-icon="Message"
                size="large"
                class="form-input"
                @keyup.enter="handleRegister"
              />
            </el-form-item>
          </template>

          <template v-else>
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

            <el-form-item prop="company">
              <el-input
                v-model="formData.company"
                placeholder="单位（选填）"
                prefix-icon="OfficeBuilding"
                size="large"
                class="form-input"
              />
            </el-form-item>
          </template>

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
import { ref, reactive, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import type { FormItemRule } from 'element-plus'
import { EditPen } from '@element-plus/icons-vue'
import {
  passwordRules,
  nameRules,
  phoneRules,
  emailRules,
  companyRules,
  requiredEmailRules,
  emailCodeRules
} from '@/utils/validate'

const router = useRouter()
const authStore = useAuthStore()
const formRef = ref(null)
const loading = ref(false)
const registerMode = ref<'phone' | 'email'>('phone')
const countdown = ref(0)
const codeSending = ref(false)
let countdownTimer: number | undefined

const formData = reactive({
  name: '',
  phone: '',
  password: '',
  confirmPassword: '',
  company: '',
  email: '',
  code: ''
})

const validateConfirmPassword: FormItemRule['validator'] = (_rule, value: string, callback) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== formData.password) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const rules = computed(() =>
  registerMode.value === 'email'
    ? {
        name: nameRules,
        email: requiredEmailRules,
        code: emailCodeRules,
        company: companyRules
      }
    : {
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
)

async function handleSendCode() {
  const email = formData.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    ElMessage.warning('请输入正确的邮箱地址')
    return
  }
  codeSending.value = true
  try {
    const res = await authApi.sendEmailCode({ email, purpose: 'register' })
    if (res.code === 200) {
      ElMessage.success('验证码已发送，请查收邮箱')
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

async function handleRegister() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    let res
    if (registerMode.value === 'email') {
      res = await authApi.emailRegister({
        name: formData.name,
        email: formData.email.trim(),
        code: formData.code.trim(),
        company: formData.company
      })
    } else {
      res = await authApi.register({
        name: formData.name,
        phone: formData.phone,
        password: formData.password,
        company: formData.company,
        email: formData.email
      })
    }

    if (res.code === 201 || res.code === 200) {
      if (registerMode.value === 'email') {
        authStore.setAuthData(res.data)
        ElMessage.success('注册成功')
        router.push('/training')
      } else {
        ElMessage.success('注册成功，即将跳转到登录页...')
        setTimeout(() => {
          router.push('/login')
        }, 1500)
      }
    }
  } catch (e) {
    console.error('Register error:', e)
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
.register-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #F1F5F9;
  position: relative;
  overflow: hidden;
  padding: 40px 24px;
}

.register-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.bg-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.3;
}

.bg-blob-1 {
  width: 460px;
  height: 460px;
  background: #A7F3D0;
  top: -120px;
  left: -80px;
  animation: blob-float 12s ease-in-out infinite;
}

.bg-blob-2 {
  width: 360px;
  height: 360px;
  background: #BAE6FD;
  bottom: -100px;
  right: -60px;
  animation: blob-float 14s ease-in-out infinite 3s;
}

.bg-blob-3 {
  width: 240px;
  height: 240px;
  background: #C4B5FD;
  top: 45%;
  right: 25%;
  opacity: 0.15;
  animation: blob-float 10s ease-in-out infinite 1.5s;
}

@keyframes blob-float {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(20px, -20px) scale(1.05); }
  66% { transform: translate(-15px, 15px) scale(0.95); }
}

.register-card-wrap {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 440px;
}

.register-card {
  background: #FFFFFF;
  border-radius: 20px;
  padding: 44px 40px 36px;
  box-shadow:
    0 4px 6px -1px rgba(15, 23, 42, 0.05),
    0 20px 50px -12px rgba(15, 23, 42, 0.1);
  border: 1px solid rgba(226, 232, 240, 0.6);
}

.card-header {
  text-align: center;
  margin-bottom: 32px;
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
  background: linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%);
  box-shadow: 0 8px 20px rgba(14, 165, 233, 0.25);
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
  margin: 0;
  line-height: 1.5;
}

.register-form {
  margin-top: 4px;
}

.mode-switch {
  display: flex;
  width: 100%;
  padding: 4px;
  margin-bottom: 24px;
  background: #F1F5F9;
  border: 1px solid #E2E8F0;
  border-radius: 12px;
}

.mode-switch :deep(.el-radio-button) {
  flex: 1;
  display: flex;
}

.mode-switch :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 40px;
  width: 100%;
  border: none;
  border-radius: 9px;
  background: transparent;
  color: #64748B;
  font-size: 14px;
  font-weight: 600;
  padding: 0;
  box-shadow: none;
  transition: all 0.25s ease;
}

.mode-switch :deep(.el-radio-button:not(.is-active):hover .el-radio-button__inner) {
  color: #0D9488;
  background: rgba(20, 184, 166, 0.08);
}

.mode-switch :deep(.el-radio-button.is-active .el-radio-button__inner) {
  background: linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%);
  color: #FFFFFF;
  box-shadow: 0 6px 16px rgba(14, 165, 233, 0.3);
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

.form-input :deep(.el-input__wrapper) {
  border-radius: 12px;
  padding: 4px 14px;
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

.register-btn {
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
  background: linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%);
  border: none;
  letter-spacing: 0.08em;
  margin-top: 8px;
  transition: all 0.2s ease;
}

.register-btn:hover {
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

@media screen and (max-width: 480px) {
  .register-page {
    padding: 24px 16px;
  }

  .register-card {
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
