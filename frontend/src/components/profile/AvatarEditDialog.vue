<template>
  <el-dialog v-model="visible" title="修改头像" width="440px" @closed="onClosed">
    <div class="avatar-edit">
      <div class="avatar-preview">
        <el-avatar :size="72" :src="avatarUrl || undefined">{{ letter }}</el-avatar>
        <span v-if="avatarPending" class="pending-tag"><el-tag type="warning" size="small">审核中</el-tag></span>
      </div>
      <el-upload
        accept="image/*"
        :show-file-list="false"
        :http-request="handleAvatarUpload"
        :disabled="avatarPending"
      >
        <el-button size="small" :loading="avatarUploading" :disabled="avatarPending">
          {{ avatarPending ? '头像审核中' : '上传新头像' }}
        </el-button>
      </el-upload>
      <p class="hint">管理员审核通过后生效</p>
    </div>
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'

const authStore = useAuthStore()
const visible = ref(false)
const avatarUploading = ref(false)

const pending = computed(() => (authStore.userInfo as any)?.pending_profile_change || null)
const avatarPending = computed(() => pending.value?.field_type === 'avatar')
const avatarUrl = computed(() => (authStore.userInfo as any)?.avatar_url || '')
const letter = computed(() => {
  const name = (authStore.userInfo as any)?.username || '?'
  return name.charAt(0).toUpperCase()
})

function open() {
  visible.value = true
}
function onClosed() {}

async function handleAvatarUpload(options: any) {
  const file = options?.file
  if (!file) return
  if (!file.type?.startsWith('image/')) {
    ElMessage.warning('请上传图片文件')
    return
  }
  if (file.size > 20 * 1024 * 1024) {
    ElMessage.warning('图片大小不能超过 20MB')
    return
  }
  avatarUploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    await authApi.uploadAvatar(fd)
    ElMessage.success('头像修改已提交，审核通过后生效')
    await authStore.refreshUserInfo()
    options.onSuccess?.({})
  } catch (e) {
    options.onError?.(e as Error)
  } finally {
    avatarUploading.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.avatar-edit { display: flex; flex-direction: column; align-items: center; gap: 12px; }
.avatar-preview { display: flex; align-items: center; gap: 12px; }
.hint { font-size: 12px; color: #909399; margin: 0; }
</style>
