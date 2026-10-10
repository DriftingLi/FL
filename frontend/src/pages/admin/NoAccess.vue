<script setup lang="ts">
/**
 * 管理端「无管理权限」页（#1638）。
 *
 * 谁落到这里：**已登录的管理员账号，但有效能力集为空**（新建后还没挂角色，或挂的角色一个
 * 能力都没勾）。收敛前这种情况没有落点 —— 守卫统一回 /admin/dashboard，而 dashboard 自己
 * 也要能力位，于是在 dashboard 上无限重定向、vue-router 中止导航，用户看到的是一片空白。
 *
 * 本页刻意**不声明能力位**（`pages.ts` 的描述符里没有 `capability`）：它正是「一个能力都没有」
 * 时的落点，再要求能力就是自锁。侧栏此时是空的（按能力过滤后没有项），顶栏照常给出身份与
 * 退出登录 —— 用户有明确的两条路：让超管给自己授权，或退出。
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { href } from '@/config/pages'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const router = useRouter()
const authStore = useAuthStore()

/** 账号名：优先昵称，回落到用户名（两者都可能缺，故再回落到空串）。 */
const who = computed(() => authStore.userInfo?.name || authStore.userInfo?.username || '')

/**
 * 退出登录：与顶栏用户菜单**同一条链**（`authStore.signOut()` 是登出单点：撤销 refresh +
 * 清本地，见 ADR-0016），落点也同一个是本子域登录页。这里不再弹确认框 —— 本页唯一的动作
 * 就是退出，多一次确认只是噪音。
 */
async function signOut(): Promise<void> {
  await authStore.signOut()
  router.push(href('Login'))
}
</script>

<template>
  <div class="no-access">
    <UiCard class="no-access-card">
      <h1 class="no-access-title">当前账号还没有管理权限</h1>
      <p class="no-access-text">
        管理端的每一项功能都由「角色权限」里的能力位决定。这个账号
        <strong v-if="who">{{ who }}</strong>
        目前一个能力位都没有，因此看不到任何菜单。
      </p>
      <p class="no-access-text">
        请联系超级管理员：在「角色权限」里给你的角色勾选所需功能，再到「管理员管理」把这个账号挂到该角色上。
      </p>
      <div class="no-access-actions">
        <UiButton @click="signOut">退出登录</UiButton>
      </div>
    </UiCard>
  </div>
</template>

<style scoped>
.no-access {
  display: flex;
  justify-content: center;
  padding-top: var(--space-10);
}

.no-access-card {
  max-width: 560px;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.no-access-title {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.no-access-text {
  margin: 0;
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-text-secondary);
}

.no-access-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
