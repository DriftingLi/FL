<template>
  <el-dialog v-model="visible" :title="title" width="440px">
    <el-form label-width="0">
      <el-form-item>
        <el-input v-model="target" placeholder="请输入新邮箱" maxlength="50" />
      </el-form-item>
      <el-form-item>
        <div class="code-row">
          <el-input v-model="code" placeholder="6位验证码" maxlength="6" @keyup.enter="submit" />
          <UiButton :disabled="countdown > 0 || sending" @click="sendCode">{{ sending ? '发送中...' : countdown > 0 ? countdown + 's 后重发' : '获取验证码' }}</UiButton>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="submitting" @click="submit">确认修改</UiButton>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { useSendCode } from '@/composables/useSendCode'
import { useVerifyDialog } from '@/composables/useVerifyDialog'
import { ref } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'

const authStore = useAuthStore()
const { remaining: countdown, send } = useSendCode({ purpose: 'bind', sendCode: (ch, tgt) => authApi.sendProfileCode({ channel: ch, target: tgt }) })
const visible = ref(false)
const title = computed(() => ((authStore.userInfo as any)?.email ? '更换邮箱' : '绑定邮箱'))
const dlg = useVerifyDialog({
  sendCode: (t, ch) => send(t, ch),
  submitAsync: (t, c) => authApi.updateProfileEmail({ email: t, code: c }),
  onSuccess: async () => { ElMessage.success('修改成功'); await authStore.refreshUserInfo(); visible.value = false }
})
const target = dlg.target; const code = dlg.code; const sending = dlg.sending; const submitting = dlg.submitting; const submit = dlg.submit
function open(){ visible.value=true; dlg.open() }
async function sendCode(){ await dlg.send(target.value, 'email') }
defineExpose({ open })
</script>

<style scoped>
.code-row { display: flex; gap: 10px; width: 100%; }
.code-row .el-input { flex: 1; }
</style>
