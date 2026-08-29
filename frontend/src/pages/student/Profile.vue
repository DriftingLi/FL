<template>
  <div class="profile-page">
    <div class="profile-list">
      <div class="profile-group">
        <div class="profile-row" @click="openAvatar">
          <span class="row-label">修改头像</span>
          <span class="row-value">
            <el-avatar :size="32" :src="avatarUrl || undefined" class="row-avatar">{{ letter }}</el-avatar>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="profile-row" @click="openNickname">
          <span class="row-label">修改昵称</span>
          <span class="row-value">
            <span class="value-text">{{ userInfo.username || '未设置' }}</span>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="profile-row" @click="openCompany">
          <span class="row-label">单位</span>
          <span class="row-value">
            <span class="value-text">{{ userInfo.company || '未填写' }}</span>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="profile-row" @click="openAccount">
          <span class="row-label">我的帐号</span>
          <span class="row-value">
            <span class="value-text">{{ userInfo.account || '未生成' }}</span>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
      </div>

      <div class="profile-group">
        <div class="profile-row" @click="openPassword">
          <span class="row-label">修改密码</span>
          <span class="row-value"><el-icon class="arrow"><ArrowRight /></el-icon></span>
        </div>
        <div class="profile-row" @click="openPhone">
          <span class="row-label">更换手机号</span>
          <span class="row-value">
            <span class="value-text">{{ maskedPhone }}</span>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
        <div class="profile-row" @click="openEmail">
          <span class="row-label">更换邮箱</span>
          <span class="row-value">
            <span class="value-text">{{ userInfo.email || '未绑定' }}</span>
            <el-icon class="arrow"><ArrowRight /></el-icon>
          </span>
        </div>
      </div>

      <div class="profile-group danger-group">
        <div class="profile-row danger-row" @click="openDelete">
          <span class="row-label">注销帐号</span>
          <span class="row-value"><el-icon class="arrow"><ArrowRight /></el-icon></span>
        </div>
      </div>

      <div class="logout-row">
        <el-button type="primary" class="logout-btn" @click="handleLogout">退出当前账号</el-button>
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
import AvatarEditDialog from '@/components/profile/AvatarEditDialog.vue'
import NicknameEditDialog from '@/components/profile/NicknameEditDialog.vue'
import CompanyEditDialog from '@/components/profile/CompanyEditDialog.vue'
import PhoneEditDialog from '@/components/profile/PhoneEditDialog.vue'
import EmailEditDialog from '@/components/profile/EmailEditDialog.vue'
import PasswordEditDialog from '@/components/profile/PasswordEditDialog.vue'
import AccountEditDialog from '@/components/profile/AccountEditDialog.vue'
import DeleteAccountDialog from '@/components/profile/DeleteAccountDialog.vue'

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

function openAvatar(){ avatarRef.value?.open() }
function openNickname(){ nicknameRef.value?.open() }
function openCompany(){ companyRef.value?.open() }
function openPhone(){ phoneRef.value?.open() }
function openEmail(){ emailRef.value?.open() }
function openPassword(){ passwordRef.value?.open() }
function openAccount(){ accountRef.value?.open() }
function openDelete(){ deleteRef.value?.open() }

async function handleLogout(){
  try {
    await ElMessageBox.confirm('确定要退出当前账号吗？', '提示', { type: 'warning' })
  } catch { return }
  try { await authApi.logout() } catch {}
  authStore.clearAuthData()
  router.push('/login')
}

onMounted(()=>{ authStore.refreshUserInfo() })
</script>

<style scoped>
.profile-page { max-width: 560px; margin: 0 auto; padding: 16px; }
.profile-list { display: flex; flex-direction: column; gap: 12px; }
.profile-group {
  background: var(--color-bg-card);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.profile-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px;
  cursor: pointer;
  border-bottom: 1px solid var(--color-bg-page);
  transition: background 0.15s;
}
.profile-row:last-child { border-bottom: none; }
.profile-row:hover { background: var(--color-bg-page); }
.profile-row:active { background: var(--color-bg-page); }
.row-label { font-size: 14px; color: var(--color-text-primary); }
.row-value { display: flex; align-items: center; gap: 8px; color: var(--color-text-tertiary); font-size: 14px; }
.value-text { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-avatar { flex-shrink: 0; background: var(--gradient-brand, var(--color-primary-500)); color: var(--color-bg-card); }
.arrow { font-size: 14px; color: var(--color-text-disabled); }
.danger-group .danger-row .row-label { color: var(--color-danger); }
.logout-row { background: var(--color-bg-card); border-radius: 12px; display: flex; justify-content: center; padding: 4px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.logout-btn { color: var(--color-bg-card); font-size: 15px; width: 100%; padding: 14px; }
.logout-btn :deep(span) { color: var(--color-bg-card); }
</style>
