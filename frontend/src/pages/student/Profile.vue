<template>
  <div class="mx-auto max-w-[560px] p-4">
    <div class="flex flex-col gap-3">
      <div class="overflow-hidden rounded-card bg-panel shadow-card">
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openAvatar">
          <span class="text-sm text-ink">修改头像</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <el-avatar :size="32" :src="avatarUrl || undefined" class="shrink-0 bg-[image:var(--gradient-brand)] text-panel">{{ letter }}</el-avatar>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openNickname">
          <span class="text-sm text-ink">修改昵称</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ userInfo.username || '未设置' }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openCompany">
          <span class="text-sm text-ink">单位</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ userInfo.company || '未填写' }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openAccount">
          <span class="text-sm text-ink">我的帐号</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ userInfo.account || '未生成' }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
      </div>

      <div class="overflow-hidden rounded-card bg-panel shadow-card">
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openPassword">
          <span class="text-sm text-ink">修改密码</span>
          <span class="flex items-center gap-2 text-sm text-ink-3"><el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon></span>
        </div>
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openPhone">
          <span class="text-sm text-ink">更换手机号</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ maskedPhone }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openEmail">
          <span class="text-sm text-ink">更换邮箱</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ userInfo.email || '未绑定' }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
      </div>

      <div class="overflow-hidden rounded-card bg-panel shadow-card">
        <div class="flex cursor-pointer items-center justify-between border-b border-line px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas last:border-b-0" @click="openResume">
          <span class="text-sm text-ink">我的简历</span>
          <span class="flex items-center gap-2 text-sm text-ink-3">
            <span class="max-w-[180px] truncate">{{ resumeVisibilityText }}</span>
            <el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon>
          </span>
        </div>
      </div>

      <div class="overflow-hidden rounded-card bg-panel shadow-card">
        <div class="flex cursor-pointer items-center justify-between px-4 py-4 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:bg-canvas" @click="openDelete">
          <span class="text-sm text-danger">注销帐号</span>
          <span class="flex items-center gap-2 text-sm text-ink-3"><el-icon class="text-sm text-ink-muted"><ArrowRight /></el-icon></span>
        </div>
      </div>

      <div class="logout-row flex justify-center rounded-card bg-panel p-1 shadow-card">
        <UiButton variant="primary" class="w-full p-3.5 text-[15px]" @click="handleLogout">退出当前账号</UiButton>
      </div>
    </div>

    <AvatarEditDialog ref="avatarRef" />
    <NicknameEditDialog ref="nicknameRef" />
    <CompanyEditDialog ref="companyRef" />
    <PhoneEditDialog ref="phoneRef" />
    <EmailEditDialog ref="emailRef" />
    <PasswordEditDialog ref="passwordRef" />
    <AccountEditDialog ref="accountRef" />
    <DeleteAccountDialog ref="deleteRef" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { resumeApi } from '@/api/resume'
import AvatarEditDialog from '@/components/profile/AvatarEditDialog.vue'
import NicknameEditDialog from '@/components/profile/NicknameEditDialog.vue'
import CompanyEditDialog from '@/components/profile/CompanyEditDialog.vue'
import PhoneEditDialog from '@/components/profile/PhoneEditDialog.vue'
import EmailEditDialog from '@/components/profile/EmailEditDialog.vue'
import PasswordEditDialog from '@/components/profile/PasswordEditDialog.vue'
import AccountEditDialog from '@/components/profile/AccountEditDialog.vue'
import DeleteAccountDialog from '@/components/profile/DeleteAccountDialog.vue'
import UiButton from '@/components/ui/UiButton.vue'

const authStore = useAuthStore()
const router = useRouter()

const userInfo = computed(() => (authStore.userInfo as any) || {})

const avatarUrl = computed(() => userInfo.value.avatar_url || '')
const letter = computed(() => (userInfo.value.username || '?').charAt(0).toUpperCase())
const maskedPhone = computed(() => {
  const p = userInfo.value.phone
  if (!p) return '未绑定'
  if (p.length < 7) return p
  return p.slice(0, 3) + '******' + p.slice(-2)
})

const avatarRef = ref<InstanceType<typeof AvatarEditDialog> | null>(null)
const nicknameRef = ref<InstanceType<typeof NicknameEditDialog> | null>(null)
const companyRef = ref<InstanceType<typeof CompanyEditDialog> | null>(null)
const phoneRef = ref<InstanceType<typeof PhoneEditDialog> | null>(null)
const emailRef = ref<InstanceType<typeof EmailEditDialog> | null>(null)
const passwordRef = ref<InstanceType<typeof PasswordEditDialog> | null>(null)
const accountRef = ref<InstanceType<typeof AccountEditDialog> | null>(null)
const deleteRef = ref<InstanceType<typeof DeleteAccountDialog> | null>(null)

const resumeVisibility = ref<string>('hidden')
const resumeVisibilityText = computed(() => resumeVisibility.value === 'open' ? '已公开' : '未公开')

function openAvatar(){ avatarRef.value?.open() }
function openNickname(){ nicknameRef.value?.open() }
function openCompany(){ companyRef.value?.open() }
function openPhone(){ phoneRef.value?.open() }
function openEmail(){ emailRef.value?.open() }
function openPassword(){ passwordRef.value?.open() }
function openAccount(){ accountRef.value?.open() }
function openDelete(){ deleteRef.value?.open() }
function openResume(){ router.push({ name: 'StudentResume' }) }

async function handleLogout(){
  try {
    await ElMessageBox.confirm('确定要退出当前账号吗？', '提示', { type: 'warning' })
  } catch { return }
  try { await authApi.logout() } catch {}
  authStore.clearAuthData()
  router.push('/login')
}

onMounted(async ()=>{
  authStore.refreshUserInfo()
  try {
    const data = await resumeApi.get()
    if (data && (data as any).visibility) resumeVisibility.value = (data as any).visibility
  } catch {}
})
</script>


