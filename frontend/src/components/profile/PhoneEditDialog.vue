<template>
  <el-dialog v-model="visible" :title="title" width="440px">
    <el-form label-width="0">
      <el-form-item>
        <el-input v-model="target" :placeholder="placeholder" maxlength="20" />
      </el-form-item>
      <el-form-item>
        <div class="code-row">
          <el-input v-model="code" placeholder="6位验证码" maxlength="6" @keyup.enter="submit" />
          <el-button :disabled="countdown > 0 || sending" @click="sendCode">{{ sending ? '发送中...' : countdown > 0 ? countdown + 's 后重发' : '获取验证码' }}</el-button>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">确认修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { useSendCode } from '@/composables/useSendCode'
import { useVerifyDialog } from '@/composables/useVerifyDialog'

const authStore = useAuthStore()
const { remaining: countdown, send } = useSendCode({ purpose: 'bind', sendCode: (ch, tgt) => authApi.sendProfileCode({ channel: ch, target: tgt }) })
const visible = ref(false)

const title = computed(() => ((authStore.userInfo as any)?.phone ? '更换手机号' : '绑定手机号'))
const placeholder = computed(() => '请输入新手机号')

const dlg = useVerifyDialog({
  sendCode: (t, ch) => send(t, ch),
  submitAsync: (t, c) => authApi.updateProfilePhone({ phone: t, code: c }),
  onSuccess: async () => { ElMessage.success('修改成功'); await authStore.refreshUserInfo(); visible.value = false }
})

const target = dlg.target
const code = dlg.code
const sending = dlg.sending
const submitting = dlg.submitting
const submit = dlg.submit

// wrap visible to dlg
function open() { visible.value = true; dlg.open() }
async function sendCode() { await dlg.send(target.value, 'phone') }

defineExpose({ open })
</script>

<style scoped>
.code-row { display: flex; gap: 10px; width: 100%; }
.code-row .el-input { flex: 1; }
</style>
