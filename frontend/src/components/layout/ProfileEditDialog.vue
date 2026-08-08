<template>
  <el-dialog v-model="visible" title="修改头像与昵称" width="480px">
    <div class="profile-edit-dialog">
      <div class="avatar-row">
        <el-avatar :size="72" :src="avatarUrl || undefined" class="current-avatar">
          {{ letter }}
        </el-avatar>
        <div class="avatar-actions">
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
      </div>

      <el-divider />

      <div class="nickname-row">
        <label class="nickname-label">昵称</label>
        <el-input
          v-model="nickname"
          maxlength="30"
          show-word-limit
          placeholder="论坛展示名（1-30 字）"
          :disabled="nicknamePending"
        />
        <div class="nickname-actions">
          <el-tag v-if="nicknamePending" type="warning" size="small">昵称审核中</el-tag>
          <el-button
            v-else
            type="primary"
            :loading="savingNickname"
            @click="saveNickname"
          >
            保存昵称
          </el-button>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'

const authStore = useAuthStore()

const visible = ref(false)
const nickname = ref('')
const avatarUploading = ref(false)
const savingNickname = ref(false)

const pending = computed(() => (authStore.userInfo as any)?.pending_profile_change || null)
const avatarPending = computed(() => pending.value?.field_type === 'avatar')
const nicknamePending = computed(() => pending.value?.field_type === 'nickname')
const avatarUrl = computed(() => (authStore.userInfo as any)?.avatar_url || '')

const letter = computed(() => {
  const name = authStore.userInfo?.nickname || authStore.userInfo?.name || authStore.userInfo?.username || '?'
  return name.charAt(0).toUpperCase()
})

function open() {
  nickname.value = authStore.userInfo?.nickname || ''
  visible.value = true
}

async function refresh() {
  await authStore.refreshUserInfo()
}

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
    const formData = new FormData()
    formData.append('file', file)
    const res = await authApi.uploadAvatar(formData)
    ElMessage.success('头像修改已提交，审核通过后生效')
    await refresh()
    options.onSuccess?.(res)
  } catch (e) {
    console.error('头像上传失败:', e)
    ElMessage.error('头像上传失败，请稍后重试')
    options.onError?.(e as Error)
  } finally {
    avatarUploading.value = false
  }
}

async function saveNickname() {
  const value = nickname.value.trim()
  if (value.length > 30) {
    ElMessage.warning('昵称不能超过 30 个字符')
    return
  }
  savingNickname.value = true
  try {
    await authApi.updateProfile({ nickname: value })
    ElMessage.success('昵称修改已提交，审核通过后生效')
    await refresh()
  } catch (e) {
    console.error('昵称提交失败:', e)
    ElMessage.error('昵称提交失败，请稍后重试')
  } finally {
    savingNickname.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.profile-edit-dialog {
  padding: 0 4px;
}

.avatar-row {
  display: flex;
  align-items: center;
  gap: 18px;
}

.current-avatar {
  flex-shrink: 0;
  font-size: 28px;
  background: var(--gradient-brand);
  color: #fff;
}

.avatar-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.hint {
  font-size: 12px;
  color: #909399;
  margin: 0;
}

.nickname-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.nickname-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.nickname-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
}

@media screen and (max-width: 480px) {
  .avatar-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
