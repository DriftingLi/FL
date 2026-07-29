<template>
  <el-dialog
    v-model="visible"
    title="登录 HRWAI 账号"
    width="420px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <div class="login-hint">
      登录后可保存对话历史，并在不同设备间同步。未登录也可临时使用 AI 助手。
    </div>
    <el-form ref="formRef" :model="formData" :rules="rules" label-width="0" class="login-form">
      <el-form-item prop="account">
        <el-input
          v-model="formData.account"
          placeholder="手机号或用户名"
          prefix-icon="User"
          size="large"
        />
      </el-form-item>
      <el-form-item prop="password">
        <el-input
          v-model="formData.password"
          type="password"
          placeholder="密码"
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
    </el-form>

    <div class="form-footer">
      <span class="footer-text">还没有 HRWAI 账号？</span>
      <a :href="registerUrl" class="footer-link">立即注册</a>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { buildSubdomainUrl } from '@/utils/subdomain'
import { passwordRules } from '@/utils/validate'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'success'): void
}>()

const authStore = useAuthStore()
const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const formRef = ref<FormInstance>()
const loading = ref(false)

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

// 注册需要跳转到 valuation 子域名（HRWAI 账号注册页）
const registerUrl = computed(() => buildSubdomainUrl('valuation', '/valuation/register'))

watch(visible, (v) => {
  if (v) {
    formData.account = ''
    formData.password = ''
    formRef.value?.clearValidate()
  }
})

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await authApi.login({
      username: formData.account,
      password: formData.password
    })
    if (res.code === 200 && res.data && res.data.token) {
      authStore.setAuthData(res.data)
      ElMessage.success('登录成功')
      emit('success')
      visible.value = false
    }
  } catch (e: any) {
    if (e.message && !e.message.includes('Network')) {
      ElMessage.error(e.message || '登录失败，请检查用户名和密码')
    }
  } finally {
    loading.value = false
  }
}

function handleClose() {
  formData.account = ''
  formData.password = ''
}
</script>

<style scoped>
.login-hint {
  font-size: 13px;
  color: var(--color-text-tertiary, #94a3b8);
  line-height: 1.6;
  margin-bottom: 16px;
  padding: 10px 12px;
  background: var(--color-bg-page, #f8fafc);
  border-radius: 8px;
}

.login-form {
  margin-top: 8px;
}

.login-btn {
  width: 100%;
  font-weight: 600;
  letter-spacing: 0.08em;
}

.form-footer {
  text-align: center;
  margin-top: 12px;
}

.footer-text {
  font-size: 13px;
  color: var(--color-text-tertiary, #94a3b8);
}

.footer-link {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-brand-600, #0284c7);
  text-decoration: none;
  margin-left: 4px;
}

.footer-link:hover {
  text-decoration: underline;
}
</style>
