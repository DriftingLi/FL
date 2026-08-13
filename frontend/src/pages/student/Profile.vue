<template>
  <div class="profile-page">
    <div class="page-header">
      <h2>个人资料</h2>
    </div>

    <el-card class="profile-card">
      <template #header>
        <span class="card-title">基本信息</span>
      </template>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="UID">
          <span>{{ userInfo.uid || '未生成' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="账号">
          <div class="cell-row">
            <span>{{ userInfo.account || '未生成' }}</span>
            <el-button link type="primary" size="small" @click="openAccountDialog">设置账号</el-button>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="昵称">
          <div class="cell-row">
            <span>{{ userInfo.username || '未设置' }}</span>
            <el-button link type="primary" size="small" @click="openProfileDialog">修改昵称/头像</el-button>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="单位">{{ userInfo.company || '未填写' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="profile-card">
      <template #header>
        <span class="card-title">联系方式（修改需验证码验证）</span>
      </template>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="手机号">
          <div class="cell-row">
            <span>{{ displayPhone || '未绑定' }}</span>
            <el-button link type="primary" size="small" @click="openBind('phone')">
              {{ displayPhone ? '修改' : '绑定' }}
            </el-button>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="邮箱">
          <div class="cell-row">
            <span>{{ userInfo.email || '未绑定' }}</span>
            <el-button link type="primary" size="small" @click="openBind('email')">
              {{ userInfo.email ? '修改' : '绑定' }}
            </el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="profile-card">
      <template #header>
        <span class="card-title">账号密码</span>
      </template>
      <div class="cell-row">
        <template v-if="userInfo.has_password">
          <span>已设置密码，可使用「账号密码登录」</span>
          <el-tag size="small" type="success">已设置</el-tag>
        </template>
        <template v-else>
          <span>尚未设置密码，设置后可使用「账号密码登录」</span>
          <el-tag size="small" type="warning">未设置</el-tag>
        </template>
        <el-button type="primary" plain @click="passwordDialogVisible = true">
          {{ userInfo.has_password ? '修改密码' : '设置密码' }}
        </el-button>
      </div>
    </el-card>

    <ProfileEditDialog ref="profileDialogRef" />

    <!-- 绑定/修改手机号或邮箱 -->
    <el-dialog v-model="bindVisible" :title="bindTitle" width="440px">
      <el-form label-width="0">
        <el-form-item>
          <el-input
            v-model="bindTarget"
            :placeholder="bindChannel === 'email' ? '请输入新邮箱' : '请输入新手机号'"
            maxlength="50"
          />
        </el-form-item>
        <el-form-item>
          <div class="code-row">
            <el-input v-model="bindCode" placeholder="6位验证码" maxlength="6" @keyup.enter="handleBind" />
            <el-button :disabled="countdown > 0 || codeSending" @click="handleSendBindCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindVisible = false">取消</el-button>
        <el-button type="primary" :loading="binding" @click="handleBind">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- 设置/修改密码 -->
    <el-dialog v-model="passwordDialogVisible" :title="userInfo.has_password ? '修改密码' : '设置密码'" width="440px">
      <el-form label-width="0">
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            show-password
            placeholder="新密码（6-20位）"
          />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="confirmPassword"
            type="password"
            show-password
            placeholder="确认新密码"
            @keyup.enter="handleSetPassword"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingPassword" @click="handleSetPassword">保存</el-button>
      </template>
    </el-dialog>

    <!-- 设置账号（短信验证码确认） -->
    <el-dialog v-model="accountDialogVisible" title="设置账号" width="440px">
      <el-form label-width="0">
        <el-form-item>
          <el-input
            v-model="newAccount"
            placeholder="新账号（4-20位字母/数字/下划线）"
            maxlength="20"
          />
        </el-form-item>
        <el-form-item>
          <div class="code-row">
            <el-input v-model="accountCode" placeholder="短信验证码" maxlength="6" @keyup.enter="handleUpdateAccount" />
            <el-button :disabled="countdown > 0 || codeSending" @click="handleSendAccountCode">
              {{ codeSending ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="accountDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="updatingAccount" @click="handleUpdateAccount">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import ProfileEditDialog from '@/components/layout/ProfileEditDialog.vue'

const authStore = useAuthStore()
const profileDialogRef = ref<InstanceType<typeof ProfileEditDialog> | null>(null)

const userInfo = computed(() => authStore.userInfo || {})

// 邮箱注册账号的 phone 是占位值（email_ 前缀），不视为已绑定手机号
const displayPhone = computed(() => {
  const phone = userInfo.value.phone || ''
  return phone.startsWith('email_') ? '' : phone
})

const bindVisible = ref(false)
const bindChannel = ref<'phone' | 'email'>('phone')
const bindTarget = ref('')
const bindCode = ref('')
const binding = ref(false)
const countdown = ref(0)
const codeSending = ref(false)
let countdownTimer: number | undefined

const passwordDialogVisible = ref(false)
const password = ref('')
const confirmPassword = ref('')
const savingPassword = ref(false)

const accountDialogVisible = ref(false)
const newAccount = ref('')
const accountCode = ref('')
const updatingAccount = ref(false)

const bindTitle = computed(() => (bindChannel.value === 'email' ? '修改邮箱' : '修改手机号'))

function openProfileDialog() {
  profileDialogRef.value?.open()
}

function openAccountDialog() {
  newAccount.value = ''
  accountCode.value = ''
  accountDialogVisible.value = true
}

async function handleSendAccountCode() {
  codeSending.value = true
  try {
    await authApi.sendAccountChangeCode()
    ElMessage.success('验证码已发送至绑定手机号，请查收')
    startCountdown()
  } catch (e) {
    // 拦截器已提示
  } finally {
    codeSending.value = false
  }
}

async function handleUpdateAccount() {
  const account = newAccount.value.trim()
  if (!/^[A-Za-z0-9_]{4,20}$/.test(account)) {
    ElMessage.warning('账号需为4-20位字母、数字或下划线')
    return
  }
  if (accountCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }
  updatingAccount.value = true
  try {
    const result = await authApi.updateAccount({ account, code: accountCode.value.trim() })
    // 响应携带新签发的 token：替换本地登录态（JWT claim 随新账号同步）
    if (result?.token) {
      authStore.setAuthData(result)
    }
    ElMessage.success('账号修改成功')
    accountDialogVisible.value = false
    await authStore.refreshUserInfo()
  } catch (e) {
    // 拦截器已提示
  } finally {
    updatingAccount.value = false
  }
}

function openBind(channel: 'phone' | 'email') {
  bindChannel.value = channel
  bindTarget.value = ''
  bindCode.value = ''
  bindVisible.value = true
}

function startCountdown() {
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

async function handleSendBindCode() {
  const target = bindTarget.value.trim()
  if (bindChannel.value === 'email') {
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(target)) {
      ElMessage.warning('请输入正确的邮箱地址')
      return
    }
  } else {
    if (!/^1[3-9]\d{9}$/.test(target)) {
      ElMessage.warning('请输入正确的手机号')
      return
    }
  }
  codeSending.value = true
  try {
    await authApi.sendProfileCode({ channel: bindChannel.value, target })
    ElMessage.success('验证码已发送，请查收')
    startCountdown()
  } catch (e) {
    // 拦截器已提示
  } finally {
    codeSending.value = false
  }
}

async function handleBind() {
  const target = bindTarget.value.trim()
  const code = bindCode.value.trim()
  if (!target) {
    ElMessage.warning('请输入手机号或邮箱')
    return
  }
  if (code.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }
  binding.value = true
  try {
    if (bindChannel.value === 'email') {
      await authApi.updateProfileEmail({ email: target, code })
    } else {
      await authApi.updateProfilePhone({ phone: target, code })
    }
    ElMessage.success('修改成功')
    bindVisible.value = false
    await authStore.refreshUserInfo()
  } catch (e) {
    // 拦截器已提示
  } finally {
    binding.value = false
  }
}

async function handleSetPassword() {
  if (password.value.length < 6 || password.value.length > 20) {
    ElMessage.warning('密码长度需为6-20位')
    return
  }
  if (password.value !== confirmPassword.value) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  savingPassword.value = true
  try {
    await authApi.updateProfilePassword(password.value)
    ElMessage.success('密码设置成功')
    passwordDialogVisible.value = false
    password.value = ''
    confirmPassword.value = ''
    // 刷新用户信息以更新 has_password 状态
    await authStore.refreshUserInfo()
  } catch (e) {
    // 拦截器已提示
  } finally {
    savingPassword.value = false
  }
}

onMounted(() => {
  authStore.refreshUserInfo()
})

onUnmounted(() => {
  if (countdownTimer) {
    window.clearInterval(countdownTimer)
    countdownTimer = undefined
  }
})
</script>

<style scoped>
.profile-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
}

.profile-card {
  margin-bottom: 16px;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.cell-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.code-row .el-input {
  flex: 1;
}
</style>
