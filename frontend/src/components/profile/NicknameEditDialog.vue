<template>
  <el-dialog v-model="visible" title="修改昵称" width="440px">
    <el-input v-model="nickname" maxlength="30" show-word-limit placeholder="昵称（1-30字）" :disabled="nicknamePending" />
    <p v-if="nicknamePending" class="hint">昵称审核中，请等待管理员审核</p>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" :disabled="nicknamePending" @click="save">提交审核</UiButton>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import UiButton from '@/components/ui/UiButton.vue'

const authStore = useAuthStore()
const visible = ref(false)
const nickname = ref('')
const saving = ref(false)

const pending = computed(() => (authStore.userInfo as any)?.pending_profile_change || null)
const nicknamePending = computed(() => pending.value?.field_type === 'nickname')

function open() {
  nickname.value = (authStore.userInfo as any)?.username || ''
  visible.value = true
}

async function save() {
  const v = nickname.value.trim()
  if (!v) { ElMessage.warning('昵称不能为空'); return }
  if (v.length > 30) { ElMessage.warning('昵称不能超过30字符'); return }
  saving.value = true
  try {
    await authApi.updateProfile({ nickname: v })
    ElMessage.success('昵称修改已提交，审核通过后生效')
    await authStore.refreshUserInfo()
    visible.value = false
  } catch {}
  saving.value = false
}

defineExpose({ open })
</script>

<style scoped>
.hint { font-size: 12px; color: #e6a23c; margin-top: 8px; }
</style>
