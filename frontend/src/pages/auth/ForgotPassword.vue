<template>
  <div class="forgot-page">
    <div class="forgot-bg">
      <div class="bg-blob bg-blob-1"></div>
      <div class="bg-blob bg-blob-2"></div>
      <div class="bg-blob bg-blob-3"></div>
    </div>

    <div class="forgot-card-wrap">
      <div class="forgot-card">
        <div class="card-header">
          <div class="card-icon">
            <el-icon :size="24">
              <Unlock />
            </el-icon>
          </div>
          <h1 class="card-title">找回密码</h1>
          <p class="card-subtitle">通过验证码重置登录密码</p>
        </div>

        <el-radio-group v-model="resetMode" class="mode-switch">
          <el-radio-button label="phone">手机号找回</el-radio-button>
          <el-radio-button label="email">邮箱找回</el-radio-button>
        </el-radio-group>

        <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="forgot-form">
          <template v-if="resetMode === 'phone'">
            <el-form-item prop="phone">
              <el-input
                v-model="formData.phone"
                placeholder="请输入注册手机号"
                prefix-icon="Phone"
                size="large"
                class="form-input"
                maxlength="11"
                @keyup.enter="handleReset"
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
                  @keyup.enter="handleReset"
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
            <el-form-item prop="email">
              <el-input
                v-model="formData.email"
                placeholder="请输入注册邮箱"
                prefix-icon="Message"
                size="large"
                class="form-input"
                @keyup.enter="handleReset"
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
                  @keyup.enter="handleReset"
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
              @keyup.enter="handleReset"
            />
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              :loading="loading"
              class="forgot-btn"
              size="large"
              @click="handleReset"
            >
              {{ loading ? '提交中...' : '重置密码' }}
            </el-button>
          </el-form-item>

          <div class="form-footer">
            <span class="footer-text">想起密码了？</span>
            <router-link to="/login" class="footer-link">返回登录</router-link>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import { ElMessage, type FormInstance } from 'element-plus'
import type { FormItemRule } from 'element-plus'
import { Unlock } from '@element-plus/icons-vue'
import { useSendCode } from '@/composables/useSendCode'
import { passwordRules, phoneRules, requiredEmailRules, emailCodeRules } from '@/utils/validate'

const router = useRouter()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const resetMode = ref<'phone' | 'email'>('phone')
const { sending: codeSending, remaining: countdown, send: sendCode } = useSendCode({
  purpose: 'reset_password',
  sendCode: (channel, target) =>
    channel === 'phone'
      ? authApi.sendPhoneCode({ phone: target, purpose: 'reset_password' })
      : authApi.sendEmailCode({ email: target, purpose: 'reset_password' })
})

const formData = reactive({
  phone: '',
  email: '',
  code: '',
  password: '',
  confirmPassword: ''
})

const validateConfirmPassword: FormItemRule['validator'] = (_rule, value: string, callback) => {
  if (value === '') {
    callback(new Error('请再次输入新密码'))
  } else if (value !== formData.password) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const confirmPasswordRules: FormItemRule[] = [
  { required: true, message: '请确认新密码', trigger: 'blur' },
  { validator: validateConfirmPassword, trigger: 'blur' }
]

const rules = computed(() =>
  resetMode.value === 'phone'
    ? { phone: phoneRules, code: emailCodeRules, password: passwordRules, confirmPassword: confirmPasswordRules }
    : { email: requiredEmailRules, code: emailCodeRules, password: passwordRules, confirmPassword: confirmPasswordRules }
)

async function handleSendCode() {
  if (resetMode.value === 'phone') {
    await sendCode(formData.phone.trim(), 'phone')
  } else {
    await sendCode(formData.email.trim(), 'email')
  }
}

async function handleReset() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    if (resetMode.value === 'phone') {
      await authApi.phoneResetPassword({
        phone: formData.phone.trim(),
        code: formData.code.trim(),
        password: formData.password
      })
    } else {
      await authApi.emailResetPassword({
        email: formData.email.trim(),
        code: formData.code.trim(),
        password: formData.password
      })
    }
    ElMessage.success('密码已重置，请使用新密码登录')
    router.push('/login')
  } catch (e) {
    console.error('Reset password error:', e)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.forgot-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #F1F5F9;
  position: relative;
  overflow: hidden;
  padding: 40px 24px;
}

.forgot-bg {
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

.forgot-card-wrap {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 440px;
}

.forgot-card {
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

.forgot-form {
  margin-top: 4px;
}

.mode-switch {
  display: flex;
  gap: 8px;
  width: 100%;
  margin-bottom: 16px;
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
  border: 1px solid #E2E8F0;
  border-radius: 10px;
  background: #FFFFFF;
  color: #64748B;
  font-size: 13px;
  font-weight: 600;
  padding: 0;
  box-shadow: none;
  transition: all 0.25s ease;
}

.mode-switch :deep(.el-radio-button:not(.is-active):hover .el-radio-button__inner) {
  border-color: #99F6E4;
  color: #0D9488;
  background: #F0FDFA;
}

.mode-switch :deep(.el-radio-button.is-active .el-radio-button__inner) {
  background: linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%);
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

.forgot-btn {
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

.forgot-btn:hover {
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
  .forgot-page {
    padding: 24px 16px;
  }

  .forgot-card {
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
