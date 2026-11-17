<script setup lang="ts">
/**
 * 讲师端「无讲师权限」页（#2600 / 规格 F5）。
 *
 * 谁落到这里：**已登录的讲师账号，但有效能力集为空**（新建后还没挂角色，或挂的角色一个
 * 能力都没勾）。讲师自 #2600 起是动态角色，可见面完全由所挂角色的能力集回答 —— 空集时
 * 侧栏过滤后一项不剩，若落点仍写死 `/training/tutor`，就会落在自己看不见的页上再被守卫回弹
 * （管理端 #1638 踩过的那种「整页空白」）。
 *
 * 本页刻意**不声明能力位**（`pages.ts` 的描述符里没有 `capability`）：它正是「一个能力都没有」
 * 时的落点，再要求能力就是自锁。侧栏此时是空的，顶栏照常给出身份与退出登录。
 *
 * **两态（#1645 同口径）**：`capabilityState === 'failed'` 时页面必须说实话 —— 那是「没能取到
 * 权限」，不是「你没有权限」。两者共用一页，但标题、正文与动作都不同（失败态多一个「重试」）。
 *
 * 刻意**不**复用管理端 `pages/admin/NoAccess.vue`：那一页的求助指引是「联系超级管理员在
 * 『角色权限』里勾能力、再到『管理员管理』挂角色」，印给讲师会把求助对象指错（讲师角色挂在
 * 「讲师管理」下，操作人是管理员而非超管）。两页形态相同但话术不同，合并就得按角色分支。
 */
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { href } from '@/config/pages'
import { TUTOR_NO_ACCESS, tutorLandingPage } from '@/config/navigation'
import { holdsCapability } from '@/utils/authzRuntime'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const router = useRouter()
const authStore = useAuthStore()

/** 账号名：优先昵称，回落到用户名（两者都可能缺，故再回落到空串）。 */
const who = computed(() => authStore.userInfo?.name || authStore.userInfo?.username || '')

/** 能力集没取到（网络/服务端异常）—— 与「取到了、是空的」不是同一件事。 */
const loadFailed = computed(() => authStore.capabilityState === 'failed')
const retrying = ref(false)

/**
 * 重试装载：成功且落点不再是本页 → 直接进那一页（用户不必自己找路）；
 * 仍然空集 → 本页就地切回「一个能力位都没有」的说明（集合是响应式的）。
 */
async function retry(): Promise<void> {
  retrying.value = true
  try {
    await authStore.loadCapabilities()
    const landing = tutorLandingPage(cap =>
      holdsCapability(authStore.userInfo?.role, authStore.capabilities, cap)
    )
    if (landing !== TUTOR_NO_ACCESS) await router.push({ name: landing })
  } finally {
    retrying.value = false
  }
}

/**
 * 退出登录：与顶栏用户菜单**同一条链**（`authStore.signOut()` 是登出单点：撤销 refresh +
 * 清本地，见 ADR-0016），落点也是本子域登录页。这里不再弹确认框 —— 本页唯一的动作
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
      <template v-if="loadFailed">
        <h1 class="no-access-title">讲师权限加载失败</h1>
        <p class="no-access-text">
          没能从服务器读到这个账号的讲师权限（网络或服务端异常），因此暂时无法判断你能看到哪些菜单。
        </p>
        <p class="no-access-text">这不代表账号没有权限 —— 请点「重试」，或稍后刷新页面。</p>
      </template>
      <template v-else>
        <h1 class="no-access-title">当前账号还没有讲师权限</h1>
        <p class="no-access-text">
          讲师端的每一项功能都由「讲师角色」里的能力位决定。这个账号
          <strong v-if="who">{{ who }}</strong>
          还没有挂上任何讲师角色，因此看不到任何菜单。
        </p>
        <p class="no-access-text">
          请联系管理员：在「讲师管理」的「讲师角色」里勾选所需功能，再把你的账号挂到该角色上。角色变更最多
          40 秒后生效，刷新页面即可。
        </p>
      </template>
      <div class="no-access-actions">
        <UiButton v-if="loadFailed" :loading="retrying" @click="retry">重试</UiButton>
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
  gap: var(--space-2);
}
</style>
