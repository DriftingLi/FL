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
            <span>{{ userInfo.phone || '未绑定' }}</span>
            <el-button link type="primary" size="small" @click="openBind('phone')">
              {{ userInfo.phone ? '修改' : '绑定' }}
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
      <el-descriptions :column="1" border>
        <el-descriptions-item label="登录密码">
          <div class="cell-row">
            <span>{{ userInfo.has_password ? '已设置，可使用「账号密码登录」' : '尚未设置' }}</span>
            <el-tag size="small" :type="userInfo.has_password ? 'success' : 'warning'">
              {{ userInfo.has_password ? '已设置' : '未设置' }}
            </el-tag>
            <el-button link type="primary" size="small" @click="passwordDialog.open()">
              {{ userInfo.has_password ? '修改密码' : '设置密码' }}
            </el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
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
            <el-input v-model="bindCode" placeholder="6位验证码" maxlength="6" @keyup.enter="bindDialog.submit" />
            <el-button :disabled="bindCountdown > 0 || bindSending" @click="handleSendBindCode">
              {{ bindSending ? '发送中...' : bindCountdown > 0 ? bindCountdown + 's 后重发' : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindVisible = false">取消</el-button>
        <el-button type="primary" :loading="bindSubmitting" @click="bindDialog.submit">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- 设置/修改密码（短信验证码确认） -->
    <el-dialog v-model="passwordVisible" :title="userInfo.has_password ? '修改密码' : '设置密码'" width="440px">
      <el-form label-width="0">
        <el-form-item>
          <div class="code-row">
            <el-input v-model="passwordCode" placeholder="短信验证码" maxlength="6" @keyup.enter="passwordDialog.submit" />
            <el-button :disabled="passwordCountdown > 0 || passwordSending" @click="handleSendPasswordCode">
              {{ passwordSending ? '发送中...' : passwordCountdown > 0 ? passwordCountdown + 's 后重发' : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
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
            @keyup.enter="passwordDialog.submit"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordVisible = false">取消</el-button>
        <el-button type="primary" :loading="passwordSubmitting" @click="passwordDialog.submit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 设置账号（短信验证码确认） -->
    <el-dialog v-model="accountVisible" title="设置账号" width="440px">
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
            <el-input v-model="accountCode" placeholder="短信验证码" maxlength="6" @keyup.enter="accountDialog.submit" />
            <el-button :disabled="accountCountdown > 0 || accountSending" @click="handleSendAccountCode">
              {{ accountSending ? '发送中...' : accountCountdown > 0 ? accountCountdown + 's 后重发' : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="accountVisible = false">取消</el-button>
        <el-button type="primary" :loading="accountSubmitting" @click="accountDialog.submit">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { isValidAccount } from '@/utils/validate'
import { useSendCode } from '@/composables/useSendCode'
import { useVerifyDialog } from '@/composables/useVerifyDialog'
import ProfileEditDialog from '@/components/layout/ProfileEditDialog.vue'

const authStore = useAuthStore()
const profileDialogRef = ref<InstanceType<typeof ProfileEditDialog> | null>(null)

const userInfo = computed(() => authStore.userInfo || {})

// ---- 绑定/修改手机号或邮箱（useSendCode 负责发送+倒计时，useVerifyDialog 负责对话框状态机）----
const bindChannel = ref<'phone' | 'email'>('phone')
const {
  remaining: bindCountdown,
  send: sendBindCode
} = useSendCode({
  purpose: 'bind',
  sendCode: (channel, target) => authApi.sendProfileCode({ channel, target })
})
const bindDialog = useVerifyDialog({
  sendCode: (target, channel) => sendBindCode(target, channel),
  submitAsync: (target, code) =>
    bindChannel.value === 'email'
      ? authApi.updateProfileEmail({ email: target, code })
      : authApi.updateProfilePhone({ phone: target, code }),
  onSuccess: async () => {
    ElMessage.success('修改成功')
    await authStore.refreshUserInfo()
  }
})
const bindVisible = bindDialog.visible
const bindTarget = bindDialog.target
const bindCode = bindDialog.code
const bindSending = bindDialog.sending
const bindSubmitting = bindDialog.submitting
const bindTitle = computed(() => (bindChannel.value === 'email' ? '修改邮箱' : '修改手机号'))

// ---- 设置/修改密码（短信验证码确认）----
const {
  remaining: passwordCountdown,
  send: sendPasswordCode
} = useSendCode({
  purpose: 'change_password',
  sendCode: () => authApi.sendChangePasswordCode()
})
const password = ref('')
const confirmPassword = ref('')
const passwordDialog = useVerifyDialog({
  sendCode: (target, channel) => sendPasswordCode(target, channel),
  submitAsync: async (_target, code) => {
    if (password.value.length < 6 || password.value.length > 20) {
      ElMessage.warning('密码长度需为6-20位')
      throw new Error('password-length-invalid')
    }
    if (password.value !== confirmPassword.value) {
      ElMessage.warning('两次输入的密码不一致')
      throw new Error('password-mismatch')
    }
    await authApi.updateProfilePassword({ code, password: password.value })
  },
  onSuccess: async () => {
    ElMessage.success('密码设置成功')
    password.value = ''
    confirmPassword.value = ''
    // 刷新用户信息以更新 has_password 状态
    await authStore.refreshUserInfo()
  }
})
const passwordVisible = passwordDialog.visible
const passwordCode = passwordDialog.code
const passwordSending = passwordDialog.sending
const passwordSubmitting = passwordDialog.submitting

// ---- 设置账号（短信验证码确认）----
const {
  remaining: accountCountdown,
  send: sendAccountCode
} = useSendCode({
  purpose: 'account_change',
  sendCode: () => authApi.sendAccountChangeCode()
})
const accountDialog = useVerifyDialog({
  sendCode: (target, channel) => sendAccountCode(target, channel),
  submitAsync: async (target, code) => {
    if (!isValidAccount(target)) {
      ElMessage.warning('账号需为4-20位字母、数字或下划线')
      throw new Error('account-invalid')
    }
    // 响应携带新签发的 token：替换本地登录态（JWT claim 随新账号同步）
    const result = await authApi.updateAccount({ account: target, code })
    if (result?.token) {
      authStore.setAuthData(result)
    }
  },
  onSuccess: async () => {
    ElMessage.success('账号修改成功')
    await authStore.refreshUserInfo()
  }
})
const accountVisible = accountDialog.visible
const newAccount = accountDialog.target
const accountCode = accountDialog.code
const accountSending = accountDialog.sending
const accountSubmitting = accountDialog.submitting

function openProfileDialog() {
  profileDialogRef.value?.open()
}

function openAccountDialog() {
  accountDialog.open()
}

function openBind(channel: 'phone' | 'email') {
  bindChannel.value = channel
  bindDialog.open()
}

async function handleSendBindCode() {
  await bindDialog.send(bindTarget.value, bindChannel.value)
}

async function handleSendPasswordCode() {
  await passwordDialog.send('', 'phone')
}

async function handleSendAccountCode() {
  await accountDialog.send('', 'phone')
}

onMounted(() => {
  authStore.refreshUserInfo()
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
