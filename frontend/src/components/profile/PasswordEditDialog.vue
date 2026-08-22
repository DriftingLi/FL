<template>
  <el-dialog v-model="visible" :title="hasPassword ? '修改密码' : '设置密码'" width="440px">
    <el-form label-width="0">
      <el-form-item>
        <div class="code-row">
          <el-input v-model="code" placeholder="短信验证码" maxlength="6" @keyup.enter="submit" />
          <el-button :disabled="countdown > 0 || sending" @click="sendCode">{{ sending ? '发送中...' : countdown > 0 ? countdown + 's 后重发' : '获取验证码' }}</el-button>
        </div>
      </el-form-item>
      <el-form-item>
        <el-input v-model="password" type="password" show-password placeholder="新密码（6-20位）" />
      </el-form-item>
      <el-form-item>
        <el-input v-model="confirm" type="password" show-password placeholder="确认新密码" @keyup.enter="submit" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible=false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
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
const visible = ref(false)
const password = ref('')
const confirm = ref('')
const hasPassword = computed(()=> !!(authStore.userInfo as any)?.has_password)

const { remaining: countdown, send } = useSendCode({ purpose: 'change_password', sendCode: () => authApi.sendChangePasswordCode() })
const dlg = useVerifyDialog({
  sendCode: (_t,ch)=> send(_t,ch),
  submitAsync: async (_t,c)=>{
    if(password.value.length<6||password.value.length>20){ElMessage.warning('密码长度需为6-20位'); throw new Error('invalid')}
    if(password.value!==confirm.value){ElMessage.warning('两次输入的密码不一致'); throw new Error('mismatch')}
    await authApi.updateProfilePassword({ code: c, password: password.value })
  },
  onSuccess: async ()=>{ ElMessage.success('密码设置成功'); password.value=''; confirm.value=''; await authStore.refreshUserInfo(); visible.value=false }
})
const code = dlg.code; const sending = dlg.sending; const submitting = dlg.submitting
const submit = dlg.submit
async function sendCode(){ await dlg.send('', 'phone') }
function open(){ visible.value=true; dlg.open() }
defineExpose({ open })
</script>

<style scoped>
.code-row { display: flex; gap: 10px; width: 100%; }
.code-row .el-input { flex: 1; }
</style>
