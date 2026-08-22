<template>
  <el-dialog v-model="visible" title="修改单位" width="440px">
    <el-input v-model="company" maxlength="50" placeholder="请输入单位名称" />
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'

const authStore = useAuthStore()
const visible = ref(false)
const company = ref('')
const saving = ref(false)

function open() {
  company.value = (authStore.userInfo as any)?.company || ''
  visible.value = true
}

async function save() {
  saving.value = true
  try {
    await authApi.updateCompany({ company: company.value })
    ElMessage.success('单位已更新')
    await authStore.refreshUserInfo()
    visible.value = false
  } catch {}
  saving.value = false
}

defineExpose({ open })
</script>
