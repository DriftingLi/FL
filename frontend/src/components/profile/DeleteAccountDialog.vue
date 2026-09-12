<template>
  <UiDialog v-model="visible" title="注销帐号" width="440px" confirm-text="确认注销" :confirm-loading="loading" :confirm-disabled="input.trim()!==account" @confirm="confirm">
    <div class="delete-content">
      <el-alert type="warning" :closable="false" show-icon title="注销后账号及所有学习数据将被永久删除且不可恢复，论坛发言将匿名化为“已注销用户”。" />
      <p class="confirm-text">请输入你的账号 <strong>{{ account }}</strong> 以确认注销：</p>
      <el-input v-model="input" placeholder="请输入账号" />
    </div>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import UiDialog from '@/components/ui/UiDialog.vue'

const authStore = useAuthStore()
const router = useRouter()
const visible = ref(false)
const input = ref('')
const loading = ref(false)
const account = computed(()=> (authStore.userInfo as any)?.account || '')

function open(){
  input.value = ''
  visible.value = true
}

async function confirm(){
  if(input.value.trim()!==account.value) return
  loading.value = true
  try{
    await authApi.deleteAccount()
    ElMessage.success('帐号已注销')
    authStore.clearAuthData()
    visible.value = false
    router.push('/login')
  } catch {}
  loading.value = false
}

defineExpose({ open })
</script>

<style scoped>
.delete-content { display:flex; flex-direction:column; gap:12px; }
.confirm-text { font-size:13px; color:#606266; margin:0; }
</style>
