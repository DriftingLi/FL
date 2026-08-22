<template>
  <el-dialog v-model="visible" title="修改账号" width="440px">
    <el-form label-width="0">
      <el-form-item>
        <el-input v-model="target" placeholder="新账号（4-20位字母/数字/下划线）" maxlength="20" />
      </el-form-item>
      <el-form-item>
        <div class="code-row">
          <el-input v-model="code" placeholder="短信验证码" maxlength="6" @keyup.enter="submit" />
          <el-button :disabled="countdown>0||sending" @click="sendCode">{{ sending?'发送中...':countdown>0?countdown+'s 后重发':'获取验证码'}}</el-button>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible=false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">确认修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { useSendCode } from '@/composables/useSendCode'
import { useVerifyDialog } from '@/composables/useVerifyDialog'
import { isValidAccount } from '@/utils/validate'

const authStore = useAuthStore()
const visible = ref(false)
const { remaining: countdown, send } = useSendCode({ purpose: 'account_change', sendCode: ()=> authApi.sendAccountChangeCode() })
const dlg = useVerifyDialog({
  sendCode: (_t,ch)=> send(_t,ch),
  submitAsync: async (t,c)=>{
    if(!isValidAccount(t)){ ElMessage.warning('账号需为4-20位字母、数字或下划线'); throw new Error('invalid')}
    const r = await authApi.updateAccount({ account: t, code: c })
    if((r as any)?.token) authStore.setAuthData(r as any)
  },
  onSuccess: async ()=>{ ElMessage.success('账号修改成功'); await authStore.refreshUserInfo(); visible.value=false }
})
const target=dlg.target; const code=dlg.code; const sending=dlg.sending; const submitting=dlg.submitting; const submit=dlg.submit
function open(){ visible.value=true; dlg.open() }
async function sendCode(){ await dlg.send('', 'phone')}
defineExpose({ open })
</script>

<style scoped>
.code-row { display: flex; gap: 10px; width: 100%; }
.code-row .el-input { flex: 1; }
</style>
